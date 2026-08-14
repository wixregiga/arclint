package engine

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/lang"
	"github.com/wixregiga/arclint/internal/lang/golang"
	"github.com/wixregiga/arclint/internal/lang/jsts"
	"github.com/wixregiga/arclint/internal/lang/python"
	"github.com/wixregiga/arclint/internal/tree"
)

// Facts returns declaration facts for one file, computed lazily and
// cached: at 1-7ms per tree-sitter parse, facts are paid for only by the
// files rules actually ask about (M8 ADR). Files outside the tree or
// outside every active language target return nil.
func (c *Context) Facts(path string) *lang.FileFacts {
	f := c.fileByPath(path)
	if f == nil {
		return nil
	}
	target := lang.TargetOf(path)
	if target == "" || !contains(c.RS.Runtime, target) {
		return nil
	}
	src := c.Content(f)

	c.factsMu.Lock()
	defer c.factsMu.Unlock()
	if cached, ok := c.facts[path]; ok {
		return cached
	}
	var facts *lang.FileFacts
	switch target {
	case "go":
		facts = golang.Facts(path, src)
	case "ts":
		facts = jsts.Facts(path, src)
	case "py":
		facts = python.Facts(path, src)
	}
	c.facts[path] = facts
	return facts
}

// FileFactsFor walks the tree and returns declaration facts for one
// repo-relative file. Read-only: no import analysis, no rules run. The
// errors say WHY facts are unavailable, because this feeds the
// `arclint facts` debug affordance.
func FileFactsFor(rs *config.RuleSet, path string) (*lang.FileFacts, error) {
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
	if ctx.fileByPath(path) == nil {
		return nil, fmt.Errorf("%s is not in the scanned tree under %s (check scan.exclude)", path, rs.Root)
	}
	target := lang.TargetOf(path)
	if target == "" {
		return nil, fmt.Errorf("%s has no language target; facts exist for go, ts/tsx/js, and py files", path)
	}
	if !contains(rs.Runtime, target) {
		return nil, fmt.Errorf("target %q is not active in rules.yaml (targets: %v)", target, rs.Runtime)
	}
	return ctx.Facts(path), nil
}
