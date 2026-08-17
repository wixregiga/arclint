package typescript

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// sourceExts drives relative-specifier extension probing across the
// full JS/TS extension set; which files this producer claims is decided
// by analyzable, not by this list.
var sourceExts = []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}

// isStdlib reports Node builtin membership ("fs", "fs/promises"); the
// node: prefix is stripped before lookup.
func isStdlib(spec string) bool {
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

// resolver classifies module specifiers for files of one repository. An
// unreadable or invalid package.json is skipped rather than fatal, as
// in the legacy analyzer: classification proceeds without its
// dependency declarations.
type resolver struct {
	files     map[string]bool
	dirs      map[string]bool
	manifests []*manifest       // sorted deepest-first
	pkgByName map[string]string // in-repo package name -> its dir
}

func newResolver(root string, files []conformance.ObservedFile) *resolver {
	r := &resolver{files: map[string]bool{}, dirs: map[string]bool{}, pkgByName: map[string]string{}}
	for _, f := range files {
		r.files[f.Path] = true
		d := path.Dir(f.Path)
		for d != "." && d != "/" {
			if r.dirs[d] {
				break
			}
			r.dirs[d] = true
			d = path.Dir(d)
		}
	}
	for _, f := range files {
		if path.Base(f.Path) != "package.json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path)))
		if err != nil {
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
			continue
		}
		m := &manifest{dir: path.Dir(f.Path), name: doc.Name, deps: map[string]bool{}}
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
	if r.files[base] {
		return base, path.Dir(base)
	}
	for _, e := range sourceExts {
		if r.files[base+e] {
			return base + e, path.Dir(base)
		}
	}
	for _, e := range sourceExts {
		idx := path.Join(base, "index"+e)
		if r.files[idx] {
			return idx, base
		}
	}
	if r.dirs[base] {
		return "", base
	}
	return "", ""
}

// classify one specifier for a file in fileDir.
func (r *resolver) classify(fileDir, spec string) (conformance.ImportClass, string, string) {
	switch {
	case strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || spec == "." || spec == "..":
		tf, td := r.resolveRelative(fileDir, spec)
		return conformance.ImportInternal, td, tf
	case strings.HasPrefix(spec, "/"):
		// Absolute filesystem specifiers are environment-dependent.
		return conformance.ImportUnknown, "", ""
	case strings.HasPrefix(spec, "#"):
		// package.json "imports" subpath aliases need manifest-conditional
		// resolution; documented as unresolved.
		return conformance.ImportUnknown, "", ""
	}
	if isStdlib(spec) {
		return conformance.ImportStdlib, "", ""
	}
	pkg := packageName(spec)
	// A bare specifier naming an in-repo package (monorepo workspace)
	// resolves internal, mirroring go.work member semantics.
	if dir, ok := r.pkgByName[pkg]; ok {
		td := dir
		if td == "" {
			td = "."
		}
		return conformance.ImportInternal, td, ""
	}
	if m := r.nearestManifest(fileDir); m != nil && m.deps[pkg] {
		return conformance.ImportExternal, "", ""
	}
	return conformance.ImportUnknown, "", ""
}
