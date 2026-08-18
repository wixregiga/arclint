package python

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// isStdlib reports exact membership of a top-level module in the embedded
// sys.stdlib_module_names table.
func isStdlib(topLevel string) bool {
	_, ok := pyStdlib[topLevel]
	return ok
}

// pep503Re normalizes distribution names per PEP 503: case-insensitive,
// runs of -, _, . equivalent.
var pep503Re = regexp.MustCompile(`[-_.]+`)

func normalizeName(name string) string {
	return pep503Re.ReplaceAllString(strings.ToLower(name), "-")
}

// requirementName extracts the distribution name from a PEP 508
// requirement string ("requests>=2.0", "pydantic[email]==2; python_version
// > '3.8'").
var requirementNameRe = regexp.MustCompile(`^\s*([A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?)`)

func requirementName(req string) string {
	m := requirementNameRe.FindStringSubmatch(req)
	if m == nil {
		return ""
	}
	return m[1]
}

// resolver classifies dotted modules for files of one repository. An
// unreadable or invalid pyproject.toml is skipped rather than fatal, as
// in the legacy analyzer: classification proceeds without its
// dependency declarations (its directory still counts as a source
// root).
type resolver struct {
	files map[string]bool
	dirs  map[string]bool
	// deps holds normalized distribution names from every pyproject.toml:
	// PEP 621 [project] dependencies and optional-dependencies, PEP 735
	// [dependency-groups], and [tool.poetry.dependencies].
	deps map[string]bool
	// sourceRoots are candidate bases for absolute-import resolution:
	// the repo root, src/ (src layout), and each pyproject.toml's dir and
	// its src/.
	sourceRoots []string
}

func newResolver(root string, files []conformance.ObservedFile) *resolver {
	r := &resolver{files: map[string]bool{}, dirs: map[string]bool{}, deps: map[string]bool{}}
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
	rootSet := map[string]bool{"": true, "src": true}
	for _, f := range files {
		if path.Base(f.Path) != "pyproject.toml" {
			continue
		}
		dir := path.Dir(f.Path)
		if dir == "." {
			dir = ""
		}
		rootSet[dir] = true
		rootSet[path.Join(dir, "src")] = true

		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path)))
		if err != nil {
			continue
		}
		var doc struct {
			Project struct {
				Dependencies         []string            `toml:"dependencies"`
				OptionalDependencies map[string][]string `toml:"optional-dependencies"`
			} `toml:"project"`
			DependencyGroups map[string][]string `toml:"dependency-groups"`
			Tool             struct {
				Poetry struct {
					Dependencies map[string]any `toml:"dependencies"`
				} `toml:"poetry"`
			} `toml:"tool"`
		}
		if err := toml.Unmarshal(data, &doc); err != nil {
			continue
		}
		add := func(req string) {
			if name := requirementName(req); name != "" {
				r.deps[normalizeName(name)] = true
			}
		}
		for _, req := range doc.Project.Dependencies {
			add(req)
		}
		for _, group := range doc.Project.OptionalDependencies {
			for _, req := range group {
				add(req)
			}
		}
		for _, group := range doc.DependencyGroups {
			for _, req := range group {
				add(req)
			}
		}
		for name := range doc.Tool.Poetry.Dependencies {
			if !strings.EqualFold(name, "python") {
				r.deps[normalizeName(name)] = true
			}
		}
	}
	for root := range rootSet {
		r.sourceRoots = append(r.sourceRoots, root)
	}
	sort.Strings(r.sourceRoots)
	return r
}

// resolveUnder maps a dotted module to a file or package directory below
// one base directory.
func (r *resolver) resolveUnder(base, dotted string) (targetFile, targetDir string, ok bool) {
	rel := strings.ReplaceAll(dotted, ".", "/")
	candidate := path.Join(base, rel)
	if candidate == "." {
		candidate = ""
	}
	if r.files[candidate+".py"] {
		return candidate + ".py", path.Dir(candidate + ".py"), true
	}
	if candidate != "" && r.dirs[candidate] {
		// PEP 420 namespace packages: a matching directory counts even
		// without __init__.py; the ambiguity this can introduce is the
		// documented false-negative class of path-to-module mapping.
		return "", candidate, true
	}
	return "", "", false
}

// classifyRelative resolves a leading-dot import: level dots go up
// level-1 directories from the importing package. Relative imports are
// internal by construction; only the target location varies.
func (r *resolver) classifyRelative(fileDir, module string) (conformance.ImportClass, string, string) {
	level := 0
	for level < len(module) && module[level] == '.' {
		level++
	}
	rest := module[level:]
	base := fileDir
	if base == "." {
		base = ""
	}
	for i := 1; i < level; i++ {
		if base == "" {
			base = ".." // escapes the repo: unresolvable
			break
		}
		base = path.Dir(base)
		if base == "." {
			base = ""
		}
	}
	if base == ".." {
		return conformance.ImportInternal, "", ""
	}
	if rest == "" {
		dir := base
		if dir == "" {
			dir = "."
		}
		return conformance.ImportInternal, dir, ""
	}
	if tf, td, ok := r.resolveUnder(base, rest); ok {
		return conformance.ImportInternal, td, tf
	}
	return conformance.ImportInternal, "", ""
}

// classify one dotted module for a file in fileDir.
func (r *resolver) classify(fileDir, module string) (conformance.ImportClass, string, string) {
	if strings.HasPrefix(module, ".") {
		return r.classifyRelative(fileDir, module)
	}

	top := module
	if idx := strings.IndexByte(module, '.'); idx >= 0 {
		top = module[:idx]
	}
	if isStdlib(top) {
		return conformance.ImportStdlib, "", ""
	}
	for _, root := range r.sourceRoots {
		if tf, td, ok := r.resolveUnder(root, module); ok {
			return conformance.ImportInternal, td, tf
		}
		// An in-repo top-level package decides internal-ness even when
		// the submodule itself does not resolve (generated or missing).
		if _, _, ok := r.resolveUnder(root, top); ok {
			return conformance.ImportInternal, "", ""
		}
	}
	// Distribution-name vs module-name mismatch (PyYAML provides yaml) is
	// the documented limitation of manifest-based classification: such
	// imports classify unknown, never silently external.
	if r.deps[normalizeName(top)] {
		return conformance.ImportExternal, "", ""
	}
	return conformance.ImportUnknown, "", ""
}
