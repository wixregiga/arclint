package application

import (
	"fmt"
	"path"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// selectorHits returns the qualified ids of the Rules one selector
// matches. A selector is an exact qualified id, an id prefix, a
// path.Match pattern, or a Pattern spelling by provenance: an exact
// namespace/name@version selects the Rules that Pattern distributed,
// and namespace/name selects them at whatever version is extended. An
// exact id wins completely: when the selector names a configured id,
// no other expansion widens it. The same semantics serve rules and
// check --only/--exclude.
func selectorHits(selector string, rules []rule.Rule) ([]string, error) {
	for _, r := range rules {
		if r.ID().Qualified() == selector {
			return []string{selector}, nil
		}
	}
	var out []string
	for _, r := range rules {
		id := r.ID().Qualified()
		if strings.HasPrefix(id, selector) {
			out = append(out, id)
			continue
		}
		ok, err := path.Match(selector, id)
		if err != nil {
			return nil, fmt.Errorf("rule selector %q: %v", selector, err)
		}
		if ok || distributedBy(r, selector) {
			out = append(out, id)
		}
	}
	return out, nil
}

// distributedBy reports whether the selector spells the Pattern the
// Rule came from, exactly or by namespace/name.
func distributedBy(r rule.Rule, selector string) bool {
	ref, ok := r.Provenance()
	if !ok {
		return false
	}
	if strings.Contains(selector, "@") {
		return ref.String() == selector
	}
	return ref.Qualifier() == selector
}
