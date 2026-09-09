package distribution

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// Selection names one Pattern by exact reference, qualified name, or bare
// name. Without an exact version it resolves to the highest carried version;
// a bare name must belong to just one namespace.
type Selection struct {
	namespace string
	name      string
	version   string
}

// NewSelection validates a command's spelling before it can be resolved.
func NewSelection(spelling string) (Selection, error) {
	spelling = strings.TrimSpace(spelling)
	if spelling == "" {
		return Selection{}, fmt.Errorf("pattern selection: spelling required")
	}
	if strings.Contains(spelling, "@") {
		ref, err := rule.ParsePatternReference(spelling)
		if err != nil {
			return Selection{}, fmt.Errorf("pattern selection %q: expected namespace/name@version", spelling)
		}
		return Selection{namespace: ref.Namespace(), name: ref.Name(), version: ref.Version()}, nil
	}
	if strings.Contains(spelling, "/") {
		parts := strings.Split(spelling, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return Selection{}, fmt.Errorf("pattern selection %q: expected namespace/name[@version] or name", spelling)
		}
		return Selection{namespace: parts[0], name: parts[1]}, nil
	}
	return Selection{name: spelling}, nil
}

// String returns the command spelling of the Selection.
func (s Selection) String() string {
	spelling := s.name
	if s.namespace != "" {
		spelling = s.namespace + "/" + spelling
	}
	if s.version != "" {
		spelling += "@" + s.version
	}
	return spelling
}

// Resolve finds the selected reference, or zero when none is carried.
// Ambiguous bare names fail here, with every highest matching reference,
// so callers cannot silently select one namespace on the user's behalf.
func (s Selection) Resolve(refs []rule.PatternReference) (rule.PatternReference, error) {
	if s.name == "" {
		return rule.PatternReference{}, fmt.Errorf("pattern selection: spelling required")
	}
	var matches []rule.PatternReference
	for _, ref := range refs {
		if ref.Name() != s.name || (s.namespace != "" && ref.Namespace() != s.namespace) ||
			(s.version != "" && ref.Version() != s.version) {
			continue
		}
		matches = append(matches, ref)
	}
	matches = highestVersions(matches)
	if len(matches) > 1 {
		spellings := make([]string, 0, len(matches))
		for _, ref := range matches {
			spellings = append(spellings, ref.String())
		}
		return rule.PatternReference{}, fmt.Errorf("pattern name %q is ambiguous; use one of %s", s.String(), strings.Join(spellings, ", "))
	}
	if len(matches) == 0 {
		return rule.PatternReference{}, nil
	}
	return matches[0], nil
}

// highestVersions keeps, per namespace/name, the highest version by
// semantic-version ordering, preserving first-seen order of names.
func highestVersions(refs []rule.PatternReference) []rule.PatternReference {
	var order []string
	best := map[string]rule.PatternReference{}
	for _, r := range refs {
		key := r.Namespace() + "/" + r.Name()
		cur, ok := best[key]
		if !ok {
			order = append(order, key)
			best[key] = r
			continue
		}
		if CompareVersions(r.Version(), cur.Version()) > 0 {
			best[key] = r
		}
	}
	out := make([]rule.PatternReference, 0, len(order))
	for _, key := range order {
		out = append(out, best[key])
	}
	return out
}
