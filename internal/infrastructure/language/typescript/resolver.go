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
// unreadable or invalid package.json or tsconfig is skipped rather than
// fatal, as in the legacy analyzer: classification proceeds without its
// declarations.
type resolver struct {
	files     map[string]bool
	dirs      map[string]bool
	manifests []*manifest       // sorted deepest-first
	pkgByName map[string]string // in-repo package name -> its dir
	configs   *configLoader
	specs     map[string][]string        // analyzed file -> the specifiers it imports
	owners    map[string]*project        // analyzed file -> its owning configuration, nil for none
	selected  map[string]map[string]bool // config file -> the analyzed files it selects
	imported  map[string]map[string]bool // config file -> files its program holds through imports
	reachable map[string][]*project      // config file -> every configuration it references, transitively
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
	r.configs = newConfigLoader(root, r.files)
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

// substitutes maps a JavaScript extension written in a specifier to the
// TypeScript sources and declarations tsc tries first, in its order:
// "./mod.js" reaches mod.ts, then mod.tsx, then mod.d.ts, and only then
// mod.js. Source: TypeScript Modules Reference, "File extension
// substitution" (https://www.typescriptlang.org/docs/handbook/modules/reference.html).
var substitutes = map[string][]string{
	".js":  {".ts", ".tsx", ".d.ts"},
	".mjs": {".mts", ".d.mts"},
	".cjs": {".cts", ".d.cts"},
}

// resolveFile implements extension probing from a repo-relative base: a
// TypeScript substitute for a written JavaScript extension, the exact
// file, base+ext, then base/index+ext. It returns the file reached and
// the directory the import names: the file's own directory, or base for
// an index file. Both are "" when no file answers.
func (r *resolver) resolveFile(base string) (targetFile, targetDir string) {
	ext := path.Ext(base)
	for _, sub := range substitutes[ext] {
		if f := strings.TrimSuffix(base, ext) + sub; r.files[f] {
			return f, path.Dir(f)
		}
	}
	if r.files[base] {
		return base, path.Dir(base)
	}
	for _, e := range sourceExts {
		if r.files[base+e] {
			return base + e, path.Dir(base)
		}
	}
	for _, e := range sourceExts {
		if idx := path.Join(base, "index"+e); r.files[idx] {
			return idx, base
		}
	}
	return "", ""
}

// resolveRelative resolves a relative specifier to the file it reaches,
// else to the directory it names.
func (r *resolver) resolveRelative(fileDir, spec string) (targetFile, targetDir string) {
	base := path.Join(fileDir, spec)
	if f, d := r.resolveFile(base); f != "" {
		return f, d
	}
	if r.dirs[base] {
		return "", base
	}
	return "", ""
}

// isRelative reports a specifier written relative to the importing
// file.
func isRelative(spec string) bool {
	return strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || spec == "." || spec == ".."
}

// targetFile returns the file a specifier reaches, relative or through
// p's aliases, "" when it reaches none that way; it is classify without
// the classes, for following a program's imports.
func (r *resolver) targetFile(p *project, fileDir, spec string) string {
	switch {
	case isRelative(spec):
		f, _ := r.resolveFile(path.Join(fileDir, spec))
		return f
	case strings.HasPrefix(spec, "/") || p == nil:
		return ""
	}
	f, _ := r.resolveAlias(p, spec)
	return f
}

// classify one specifier for a file in fileDir owned by the
// configuration p (nil when no configuration owns the file).
func (r *resolver) classify(p *project, fileDir, spec string) (conformance.ImportClass, string, string) {
	switch {
	case isRelative(spec):
		tf, td := r.resolveRelative(fileDir, spec)
		return conformance.ImportInternal, td, tf
	case strings.HasPrefix(spec, "/"):
		// Absolute filesystem specifiers are environment-dependent.
		return conformance.ImportUnknown, "", ""
	}
	// compilerOptions.paths and baseUrl apply to every non-relative
	// specifier before anything else, "#" ones included, as in
	// TypeScript; an alias that reaches no file falls through.
	if p != nil {
		if f, d := r.resolveAlias(p, spec); f != "" {
			return conformance.ImportInternal, d, f
		}
	}
	if strings.HasPrefix(spec, "#") {
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
