// Package rules is the arclint check engine. Evaluate runs every enabled
// rule from a loaded config against a walked file tree and returns the
// violations that survive ignore and baseline suppression
// (docs/design/rules.md).
package rules

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/walk"
)

// Violation is one rule finding, shaped per rules.md §1. Path is
// slash-separated and relative to the repo root. Line is nil when the
// violation is not tied to a line.
type Violation struct {
	RuleID   string
	Category config.Category
	Severity config.Severity
	Path     string
	Line     *int
	Message  string
	FixHint  string
}

// CategoryOrder is the fixed presentation order for grouped output.
var CategoryOrder = []config.Category{
	config.CategoryStructure,
	config.CategoryNaming,
	config.CategoryDependencies,
	config.CategoryContent,
	config.CategoryCustom,
}

// Jobs bounds the per-rule file-reading worker pools. The check command
// sets it from --jobs; zero or negative means GOMAXPROCS.
var Jobs = 0

func jobCount() int {
	if Jobs > 0 {
		return Jobs
	}
	return runtime.NumCPU()
}

// ruleFunc is one compiled rule ready to evaluate.
type ruleFunc func(c *evalCtx) ([]Violation, error)

// evalCtx is the shared, read-only evaluation context: the absolute repo
// root and every walked file as a sorted, slash-separated, root-relative
// path.
type evalCtx struct {
	root  string
	paths []string

	goModOnce sync.Once
	goModPath string
}

// read returns the content of a root-relative file.
func (c *evalCtx) read(rel string) ([]byte, error) {
	return os.ReadFile(filepath.Join(c.root, filepath.FromSlash(rel)))
}

// Evaluate merges extends, compiles every enabled rule (all regexes compile
// once, up front), evaluates rules in parallel (one goroutine per rule,
// results collected in deterministic order), and applies ignore and
// baseline suppression. Any returned error is a config/execution error —
// the caller maps it to exit 2.
func Evaluate(cfg *config.File, root string, paths []string) ([]Violation, error) {
	merged, err := MergedRules(cfg)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(merged))
	for id, r := range merged {
		if r.Severity == config.SeverityOff {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)

	funcs := make([]ruleFunc, len(ids))
	var compileErrs []error
	for i, id := range ids {
		f, err := compileRule(id, merged[id])
		if err != nil {
			compileErrs = append(compileErrs, err)
			continue
		}
		funcs[i] = f
	}
	if err := errors.Join(compileErrs...); err != nil {
		return nil, err
	}

	baseline, err := loadBaseline(cfg, root)
	if err != nil {
		return nil, err
	}

	ctx := &evalCtx{root: root, paths: paths}
	results := make([][]Violation, len(ids))
	errs := make([]error, len(ids))
	var wg sync.WaitGroup
	for i := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = funcs[i](ctx)
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	var out []Violation
	for _, vs := range results {
		for _, v := range vs {
			if ignored(cfg.Ignore, v) || baseline.has(v) {
				continue
			}
			out = append(out, v)
		}
	}
	sortViolations(out)
	return out, nil
}

// compileRule dispatches on the (exactly one) non-nil typed params block.
func compileRule(id string, r config.Rule) (ruleFunc, error) {
	switch {
	case r.Structure != nil:
		return compileStructure(id, r), nil
	case r.Naming != nil:
		return compileNaming(id, r)
	case r.Dependencies != nil:
		return compileDependencies(id, r), nil
	case r.Content != nil:
		return compileContent(id, r)
	case r.Custom != nil:
		return compileCustom(id, r), nil
	}
	return nil, fmt.Errorf("rule %q carries no params for type %q — add a params block", id, r.Type)
}

// ignored applies the config's per-path suppressions to one violation.
func ignored(ignores []config.Ignore, v Violation) bool {
	for _, ig := range ignores {
		if !walk.Match(ig.Path, v.Path) {
			continue
		}
		if len(ig.Rules) == 0 || slices.Contains(ig.Rules, v.RuleID) {
			return true
		}
	}
	return false
}

func sortViolations(vs []Violation) {
	rank := make(map[config.Category]int, len(CategoryOrder))
	for i, c := range CategoryOrder {
		rank[c] = i
	}
	sort.SliceStable(vs, func(i, j int) bool {
		a, b := vs[i], vs[j]
		if rank[a.Category] != rank[b.Category] {
			return rank[a.Category] < rank[b.Category]
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		al, bl := 0, 0
		if a.Line != nil {
			al = *a.Line
		}
		if b.Line != nil {
			bl = *b.Line
		}
		if al != bl {
			return al < bl
		}
		return a.RuleID < b.RuleID
	})
}

// targeted applies a rule's file filter: include defaults to everything,
// exclude is subtracted. Global excludes were already subtracted by the
// walk that produced paths.
func targeted(paths []string, f *config.FileFilter) []string {
	if f == nil || (len(f.Include) == 0 && len(f.Exclude) == 0) {
		return paths
	}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if len(f.Include) > 0 && !matchAny(f.Include, p) {
			continue
		}
		if matchAny(f.Exclude, p) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func matchAny(patterns []string, rel string) bool {
	for _, p := range patterns {
		if walk.Match(p, rel) {
			return true
		}
	}
	return false
}

// forFiles fans fn out over files with a bounded worker pool (no per-file
// goroutine explosion) and flattens results in file order.
func forFiles(files []string, fn func(rel string) ([]Violation, error)) ([]Violation, error) {
	if len(files) == 0 {
		return nil, nil
	}
	n := min(jobCount(), len(files))
	if n < 1 {
		n = 1
	}
	results := make([][]Violation, len(files))
	errs := make([]error, len(files))
	idx := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < n; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range idx {
				results[i], errs[i] = fn(files[i])
			}
		}()
	}
	for i := range files {
		idx <- i
	}
	close(idx)
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	var out []Violation
	for _, vs := range results {
		out = append(out, vs...)
	}
	return out, nil
}

func splitLines(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}
