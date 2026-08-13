// Package engine orchestrates rule evaluation: it walks the tree, builds
// per-language import graphs, resolves module membership, and runs every
// rule provider, collecting violations deterministically.
package engine

import (
	"path/filepath"
	"sort"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/ext"
	"github.com/wixregiga/arclint/internal/lang/golang"
	"github.com/wixregiga/arclint/internal/lang/jsts"
	"github.com/wixregiga/arclint/internal/lang/python"
	"github.com/wixregiga/arclint/internal/report"
	"github.com/wixregiga/arclint/internal/tree"
)

// Result of one check run.
type Result struct {
	Violations []report.Violation
	// Warnings are non-rule diagnostics: unparseable files skipped,
	// unknown imports under the warn policy, and similar.
	Warnings []string
	// FilesScanned is the walked tree size.
	FilesScanned int
	// Suppressed counts findings dropped by except clauses; they are
	// never silent.
	Suppressed int
}

// HasErrors reports whether any violation carries severity error, which is
// what makes `arclint check` exit 1.
func (r *Result) HasErrors() bool {
	for _, v := range r.Violations {
		if v.Severity == report.SeverityError {
			return true
		}
	}
	return false
}

// Check evaluates the ruleset against the tree rooted at rs.Root.
func Check(rs *config.RuleSet) (*Result, error) {
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
	ctx.warnings = append(ctx.warnings, t.Warnings...)

	if contains(rs.Runtime, "go") {
		ctx.Go = golang.Analyze(t)
		ctx.warnings = append(ctx.warnings, ctx.Go.Warnings...)
		for p, fa := range ctx.Go.Files {
			ctx.Imports[p] = fa.Imports
		}
	}
	if contains(rs.Runtime, "ts") {
		ja := jsts.Analyze(t)
		ctx.warnings = append(ctx.warnings, ja.Warnings...)
		for p, fa := range ja.Files {
			ctx.Imports[p] = fa.Imports
		}
	}
	if contains(rs.Runtime, "py") {
		pa := python.Analyze(t)
		ctx.warnings = append(ctx.warnings, pa.Warnings...)
		for p, fa := range pa.Files {
			ctx.Imports[p] = fa.Imports
		}
	}

	var vs []report.Violation
	vs = append(vs, checkConsumes(ctx)...)
	vs = append(vs, checkUnknownImports(ctx)...)
	vs = append(vs, checkGraphRules(ctx)...)
	vs = append(vs, checkProvides(ctx)...)
	vs = append(vs, checkInvariants(ctx)...)
	if len(rs.Rules) > 0 {
		reg, err := ext.LoadDir(rs.Root, ext.Options{
			CacheDir: filepath.Join(rs.Root, ".arclint", "cache"),
		})
		if err != nil {
			return nil, err
		}
		evs, err := checkExtensions(ctx, reg)
		if err != nil {
			return nil, err
		}
		vs = append(vs, evs...)
	}
	fillCapabilities(rs, vs)
	vs, suppressed := applyExcepts(rs, vs)
	report.Sort(vs)
	sort.Strings(ctx.warnings)
	return &Result{Violations: vs, Warnings: ctx.warnings, FilesScanned: len(t.Files), Suppressed: suppressed}, nil
}
