package engine

import (
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
