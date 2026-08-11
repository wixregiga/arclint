package engine

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/lang/golang"
	"github.com/wixregiga/arclint/internal/tree"
)

// Context carries everything a rule provider may consume: the walked tree,
// module membership, the per-language import analysis, and cached file
// contents. Providers never touch the filesystem directly beyond this.
type Context struct {
	RS   *config.RuleSet
	Tree *tree.Tree
	Go   *golang.Analysis // nil unless the go target is active

	// ModuleNames is sorted for deterministic iteration.
	ModuleNames []string
	// ModuleFiles maps a declared module to its member files (tree order).
	ModuleFiles map[string][]*tree.File
	// FileModules maps a file path to the sorted names of every module the
	// file belongs to. Overlapping module globs are legal; membership is a
	// set.
	FileModules map[string][]string
	// DirModules maps a repo-relative directory to the sorted union of the
	// modules of files directly inside it.
	DirModules map[string][]string

	warnings []string

	contentMu sync.Mutex
	contents  map[string][]byte
}

func newContext(rs *config.RuleSet, t *tree.Tree) (*Context, error) {
	ctx := &Context{
		RS:          rs,
		Tree:        t,
		ModuleFiles: map[string][]*tree.File{},
		FileModules: map[string][]string{},
		DirModules:  map[string][]string{},
		contents:    map[string][]byte{},
	}
	for name := range rs.Modules {
		ctx.ModuleNames = append(ctx.ModuleNames, name)
	}
	sort.Strings(ctx.ModuleNames)

	for _, f := range t.Files {
		for _, name := range ctx.ModuleNames {
			if matchAny(rs.Modules[name], f.Path) {
				ctx.ModuleFiles[name] = append(ctx.ModuleFiles[name], f)
				ctx.FileModules[f.Path] = append(ctx.FileModules[f.Path], name)
			}
		}
	}
	dirSets := map[string]map[string]bool{}
	for _, f := range t.Files {
		mods := ctx.FileModules[f.Path]
		if len(mods) == 0 {
			continue
		}
		d := f.Dir()
		set := dirSets[d]
		if set == nil {
			set = map[string]bool{}
			dirSets[d] = set
		}
		for _, m := range mods {
			set[m] = true
		}
	}
	for d, set := range dirSets {
		names := make([]string, 0, len(set))
		for m := range set {
			names = append(names, m)
		}
		sort.Strings(names)
		ctx.DirModules[d] = names
	}
	return ctx, nil
}

// matchAny implements module-membership glob semantics: a pattern matches
// a file directly, or names a directory whose whole subtree belongs to the
// module (the proposal's `features: ["internal/features/*"]` shape).
func matchAny(globs []string, p string) bool {
	for _, g := range globs {
		if ok, _ := doublestar.Match(g, p); ok {
			return true
		}
		if ok, _ := doublestar.Match(strings.TrimSuffix(g, "/")+"/**", p); ok {
			return true
		}
	}
	return false
}

// Content returns a file's bytes, cached per run. Unreadable files warn
// once and read as empty.
func (c *Context) Content(f *tree.File) []byte {
	c.contentMu.Lock()
	defer c.contentMu.Unlock()
	if data, ok := c.contents[f.Path]; ok {
		return data
	}
	data, err := os.ReadFile(f.Abs)
	if err != nil {
		c.warnings = append(c.warnings, fmt.Sprintf("%s: unreadable: %v", f.Path, err))
		data = nil
	}
	c.contents[f.Path] = data
	return data
}

// Warn records a non-rule diagnostic.
func (c *Context) Warn(format string, args ...any) {
	c.warnings = append(c.warnings, fmt.Sprintf(format, args...))
}

// severityOf resolves a rule's severity, defaulting to error.
func severityOf(s string) string {
	if s == "" {
		return "error"
	}
	return s
}

// contains reports set membership in a small sorted slice.
func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// intersects reports whether two small string sets overlap.
func intersects(a, b []string) bool {
	for _, v := range a {
		if contains(b, v) {
			return true
		}
	}
	return false
}
