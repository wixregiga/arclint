// Package python is the Python language target: a lexer-grade import
// extractor with documented false-negative classes, an embedded stdlib
// table (sys.stdlib_module_names of the pinned CPython), and
// manifest-based (pyproject.toml) external classification.
package python

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"golang.org/x/sync/errgroup"

	"github.com/wixregiga/arclint/internal/lang"
	"github.com/wixregiga/arclint/internal/tree"
)

// Analysis is the Python view of the repository.
type Analysis struct {
	Files    map[string]*lang.FileAnalysis
	Warnings []string
}

// IsStdlib reports exact membership of a top-level module in the embedded
// sys.stdlib_module_names table.
func IsStdlib(topLevel string) bool {
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

type resolver struct {
	files map[string]*tree.File
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

func newResolver(t *tree.Tree, warns *[]string) *resolver {
	r := &resolver{files: map[string]*tree.File{}, dirs: map[string]bool{}, deps: map[string]bool{}}
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
	rootSet := map[string]bool{"": true, "src": true}
	for _, f := range t.Files {
		if f.Name() != "pyproject.toml" {
			continue
		}
		dir := f.Dir()
		if dir == "." {
			dir = ""
		}
		rootSet[dir] = true
		rootSet[path.Join(dir, "src")] = true

		data, err := os.ReadFile(f.Abs)
		if err != nil {
			*warns = append(*warns, fmt.Sprintf("%s: unreadable: %v", f.Path, err))
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
			*warns = append(*warns, fmt.Sprintf("%s: invalid TOML: %v", f.Path, err))
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
	if r.files[candidate+".py"] != nil {
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

// classify one dotted module for a file in fileDir.
func (r *resolver) classify(fileDir, module string) (lang.Class, string, string) {
	if strings.HasPrefix(module, ".") {
		// Relative import: level dots go up level-1 directories from the
		// importing package.
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
			return lang.ClassInternal, "", ""
		}
		if rest == "" {
			dir := base
			if dir == "" {
				dir = "."
			}
			return lang.ClassInternal, dir, ""
		}
		if tf, td, ok := r.resolveUnder(base, rest); ok {
			return lang.ClassInternal, td, tf
		}
		return lang.ClassInternal, "", ""
	}

	top := module
	if idx := strings.IndexByte(module, '.'); idx >= 0 {
		top = module[:idx]
	}
	if IsStdlib(top) {
		return lang.ClassStdlib, "", ""
	}
	for _, root := range r.sourceRoots {
		if tf, td, ok := r.resolveUnder(root, module); ok {
			return lang.ClassInternal, td, tf
		}
		// An in-repo top-level package decides internal-ness even when
		// the submodule itself does not resolve (generated or missing).
		if _, _, ok := r.resolveUnder(root, top); ok {
			return lang.ClassInternal, "", ""
		}
	}
	// Distribution-name vs module-name mismatch (PyYAML provides yaml) is
	// the documented limitation of manifest-based classification: such
	// imports classify unknown, never silently external.
	if r.deps[normalizeName(top)] {
		return lang.ClassExternal, "", ""
	}
	return lang.ClassUnknown, "", ""
}

// Analyze extracts and classifies imports for every .py file, in
// parallel, deterministically.
func Analyze(t *tree.Tree) *Analysis {
	a := &Analysis{Files: map[string]*lang.FileAnalysis{}}
	res := newResolver(t, &a.Warnings)

	var mu sync.Mutex
	g := new(errgroup.Group)
	g.SetLimit(runtime.GOMAXPROCS(0))
	for _, f := range t.Files {
		if lang.TargetOf(f.Path) != "py" {
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
					imp := lang.Import{Path: ri.module, Line: ri.line}
					imp.Class, imp.TargetDir, imp.TargetFile = res.classify(dir, ri.module)
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
