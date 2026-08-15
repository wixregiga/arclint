package engine

import (
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/lang"
	"github.com/wixregiga/arclint/internal/tree"
)

// ModuleInfo is the presentation view of one declared module, consumed by
// `arclint module ls` and `arclint module info`.
type ModuleInfo struct {
	Name        string
	Description string
	Paths       []string
	// Files is the number of member files in the walked tree.
	Files int
	// Langs lists the language targets present among member files
	// (derived from the files themselves, ordered go, ts, py).
	Langs []string
}

// ModuleInfos walks the tree and resolves membership for every declared
// module, without running any rules.
func ModuleInfos(rs *config.RuleSet) ([]ModuleInfo, error) {
	t, err := tree.Walk(rs.Root, tree.Options{
		Exclude:         rs.Scan.Exclude,
		IncludeTestdata: rs.Scan.IncludeTestdata,
	})
	if err != nil {
		return nil, err
	}
	ctx, err := newContext(rs, t)
	if err != nil {
		return nil, err
	}

	out := make([]ModuleInfo, 0, len(ctx.ModuleNames))
	for _, name := range ctx.ModuleNames {
		def := rs.Modules[name]
		info := ModuleInfo{
			Name:        name,
			Description: def.Description,
			Paths:       def.Paths,
			Files:       len(ctx.ModuleFiles[name]),
		}
		present := map[string]bool{}
		for _, f := range ctx.ModuleFiles[name] {
			if target := lang.TargetOf(f.Path); target != "" {
				present[target] = true
			}
		}
		for _, target := range []string{"go", "ts", "py"} {
			if present[target] {
				info.Langs = append(info.Langs, target)
			}
		}
		out = append(out, info)
	}
	// ctx.ModuleNames is sorted, so out is already in name order.
	return out, nil
}

// PathModules resolves the declared modules that own one repo path,
// using the same membership semantics as check. A file resolves through
// its own path; a directory resolves through the union of the walked
// files under it. The bool reports whether the path names any walked
// file or directory; a walked path owned by no module returns an empty
// slice with true.
func PathModules(rs *config.RuleSet, p string) ([]string, bool, error) {
	t, err := tree.Walk(rs.Root, tree.Options{
		Exclude:         rs.Scan.Exclude,
		IncludeTestdata: rs.Scan.IncludeTestdata,
	})
	if err != nil {
		return nil, false, err
	}
	ctx, err := newContext(rs, t)
	if err != nil {
		return nil, false, err
	}
	clean := strings.TrimPrefix(path.Clean(filepath.ToSlash(p)), "./")
	if mods, ok := ctx.FileModules[clean]; ok {
		return mods, true, nil
	}
	// Directory: union the modules of every walked file underneath.
	// FileModules is scanned directly instead of DirModules so that
	// parent directories holding only subdirectories still resolve.
	prefix := clean + "/"
	if clean == "." {
		prefix = ""
	}
	set := map[string]bool{}
	walked := clean == "."
	for f, mods := range ctx.FileModules {
		if !strings.HasPrefix(f, prefix) {
			continue
		}
		walked = true
		for _, m := range mods {
			set[m] = true
		}
	}
	if !walked {
		// A walked file or dir whose files belong to no module still
		// exists: check the tree before reporting the path unknown.
		for _, f := range t.Files {
			if f.Path == clean || strings.HasPrefix(f.Path, prefix) {
				walked = true
				break
			}
		}
	}
	if !walked {
		return nil, false, nil
	}
	names := make([]string, 0, len(set))
	for m := range set {
		names = append(names, m)
	}
	sort.Strings(names)
	return names, true, nil
}
