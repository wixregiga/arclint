package application

import (
	"fmt"
	"path"
	"strings"
)

// selectorHits returns the qualified Rule ids one selector matches. A
// selector is an exact qualified id, an id prefix, or a path.Match
// pattern — and an exact id wins completely: when the selector names a
// configured id, prefix and pattern expansion never widen it. The
// same semantics serve rules, explain, and check --only/--exclude.
func selectorHits(selector string, ids []string) ([]string, error) {
	for _, id := range ids {
		if id == selector {
			return []string{id}, nil
		}
	}
	var out []string
	for _, id := range ids {
		if strings.HasPrefix(id, selector) {
			out = append(out, id)
			continue
		}
		ok, err := path.Match(selector, id)
		if err != nil {
			return nil, fmt.Errorf("rule selector %q: %v", selector, err)
		}
		if ok {
			out = append(out, id)
		}
	}
	return out, nil
}
