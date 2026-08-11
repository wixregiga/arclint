// Package jsts is the JS/TS language target: a lexer-grade import
// extractor with documented false-negative classes, an embedded Node
// builtin table, and manifest-based (package.json) external
// classification.
package jsts

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"runtime"
	"sort"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/wixregiga/arclint/internal/lang"
	"github.com/wixregiga/arclint/internal/tree"
)

// Analysis is the JS/TS view of the repository.
type Analysis struct {
	Files    map[string]*lang.FileAnalysis
	Warnings []string
}

var sourceExts = []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}

func analyzable(p string) bool {
	if strings.HasSuffix(p, ".d.ts") {
		return false
	}
	for _, e := range sourceExts {
		if strings.HasSuffix(p, e) {
			return true
		}
	}
	return false
}

// IsStdlib reports Node builtin membership ("fs", "fs/promises"); the
// node: prefix is stripped before lookup.
func IsStdlib(spec string) bool {
	spec = strings.TrimPrefix(spec, "node:")
	_, ok := nodeBuiltins[spec]
	return ok
}

// manifest is one package.json: its dependency names across all
// dependency sections, and its package name for in-repo (workspace)
// resolution.
type manifest struct {
	dir  string // repo-relative, "" for root
	name string
	deps map[string]bool
}

type resolver struct {
	files     map[string]*tree.File
	dirs      map[string]bool
	manifests []*manifest       // sorted deepest-first
	pkgByName map[string]string // in-repo package name -> its dir
}

func newResolver(t *tree.Tree, warns *[]string) *resolver {
	r := &resolver{files: map[string]*tree.File{}, dirs: map[string]bool{}, pkgByName: map[string]string{}}
	for _, f := range t.Files {
		r.files[f.Path] = f
		d := f.Dir()
		for d != "." && d != "/" {
			if r.dirs[d] {
				break
			}
			r.dirs[d] = true
			d = path.Dir(d)
		}
	}
	for _, f := range t.Files {
		if f.Name() != "package.json" {
			continue
		}
		data, err := os.ReadFile(f.Abs)
		if err != nil {
			*warns = append(*warns, fmt.Sprintf("%s: unreadable: %v", f.Path, err))
			continue
		}
		var doc struct {
			Name                 string            `json:"name"`
			Dependencies         map[string]string `json:"dependencies"`
			DevDependencies      map[string]string `json:"devDependencies"`
			PeerDependencies     map[string]string `json:"peerDependencies"`
			OptionalDependencies map[string]string `json:"optionalDependencies"`
		}
		if err := json.Unmarshal(data, &doc); err != nil {
			*warns = append(*warns, fmt.Sprintf("%s: invalid JSON: %v", f.Path, err))
			continue
		}
		m := &manifest{dir: f.Dir(), name: doc.Name, deps: map[string]bool{}}
		if m.dir == "." {
			m.dir = ""
		}
		for _, section := range []map[string]string{doc.Dependencies, doc.DevDependencies, doc.PeerDependencies, doc.OptionalDependencies} {
			for name := range section {
				m.deps[name] = true
			}
		}
		r.manifests = append(r.manifests, m)
		if doc.Name != "" {
			if _, exists := r.pkgByName[doc.Name]; !exists {
				r.pkgByName[doc.Name] = m.dir
			}
		}
	}
	sort.Slice(r.manifests, func(i, j int) bool {
		if len(r.manifests[i].dir) != len(r.manifests[j].dir) {
			return len(r.manifests[i].dir) > len(r.manifests[j].dir)
		}
		return r.manifests[i].dir < r.manifests[j].dir
	})
	return r
}

// nearestManifest returns the deepest manifest whose dir encloses fileDir.
func (r *resolver) nearestManifest(fileDir string) *manifest {
	if fileDir == "." {
		fileDir = ""
	}
	for _, m := range r.manifests {
		if m.dir == "" || m.dir == fileDir || strings.HasPrefix(fileDir, m.dir+"/") {
			return m
		}
	}
	return nil
}

// packageName extracts the npm package from a specifier: first segment,
// or first two for @scope/name.
func packageName(spec string) string {
	parts := strings.Split(spec, "/")
	if strings.HasPrefix(spec, "@") && len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

// resolveRelative implements extension-probing resolution for relative
// specifiers: exact file, spec+ext, spec/index+ext, else the directory.
func (r *resolver) resolveRelative(fileDir, spec string) (targetFile, targetDir string) {
	base := path.Join(fileDir, spec)
	if r.files[base] != nil {
		return base, path.Dir(base)
	}
	for _, e := range sourceExts {
		if r.files[base+e] != nil {
			return base + e, path.Dir(base)
		}
	}
	for _, e := range sourceExts {
		idx := path.Join(base, "index"+e)
		if r.files[idx] != nil {
			return idx, base
		}
	}
	if r.dirs[base] {
		return "", base
	}
	return "", ""
}

// classify one specifier for a file in fileDir.
func (r *resolver) classify(fileDir, spec string) (lang.Class, string, string) {
	switch {
	case strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || spec == "." || spec == "..":
		tf, td := r.resolveRelative(fileDir, spec)
		return lang.ClassInternal, td, tf
	case strings.HasPrefix(spec, "/"):
		// Absolute filesystem specifiers are environment-dependent.
		return lang.ClassUnknown, "", ""
	case strings.HasPrefix(spec, "#"):
		// package.json "imports" subpath aliases need manifest-conditional
		// resolution; documented as unresolved.
		return lang.ClassUnknown, "", ""
	}
	if IsStdlib(spec) {
		return lang.ClassStdlib, "", ""
	}
	pkg := packageName(spec)
	// A bare specifier naming an in-repo package (monorepo workspace)
	// resolves internal, mirroring go.work member semantics.
	if dir, ok := r.pkgByName[pkg]; ok {
		td := dir
		if td == "" {
			td = "."
		}
		return lang.ClassInternal, td, ""
	}
	if m := r.nearestManifest(fileDir); m != nil && m.deps[pkg] {
		return lang.ClassExternal, "", ""
	}
	return lang.ClassUnknown, "", ""
}

// Analyze extracts and classifies imports for every JS/TS file, in
// parallel, deterministically.
func Analyze(t *tree.Tree) *Analysis {
	a := &Analysis{Files: map[string]*lang.FileAnalysis{}}
	res := newResolver(t, &a.Warnings)

	var mu sync.Mutex
	g := new(errgroup.Group)
	g.SetLimit(runtime.GOMAXPROCS(0))
	for _, f := range t.Files {
		if !analyzable(f.Path) {
			continue
		}
		g.Go(func() error {
			fa := &lang.FileAnalysis{Path: f.Path}
			src, err := os.ReadFile(f.Abs)
			if err != nil {
				fa.ParseError = fmt.Sprintf("read: %v", err)
			} else {
				dir := f.Dir()
				for _, ri := range extract(string(src)) {
					imp := lang.Import{Path: ri.spec, Line: ri.line}
					imp.Class, imp.TargetDir, imp.TargetFile = res.classify(dir, ri.spec)
					fa.Imports = append(fa.Imports, imp)
				}
			}
			mu.Lock()
			a.Files[f.Path] = fa
			if fa.ParseError != "" {
				a.Warnings = append(a.Warnings, fmt.Sprintf("%s: skipped: %s", f.Path, fa.ParseError))
			}
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()
	sort.Strings(a.Warnings)
	return a
}
