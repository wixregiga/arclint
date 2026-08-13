package engine

import (
	"github.com/wixregiga/arclint/internal/lang"
	"github.com/wixregiga/arclint/internal/lang/golang"
	"github.com/wixregiga/arclint/internal/lang/jsts"
	"github.com/wixregiga/arclint/internal/lang/python"
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
