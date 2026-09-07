package vocab

import (
	"errors"
	"fmt"
	"strings"
)

// UbiquitousLanguageFileName is the committed filename of the project's
// recorded Ubiquitous Language, resolved beside the ruleset root: the
// whole of the project's domain, one file per repository.
const UbiquitousLanguageFileName = "domain.arclint.yaml"

// UbiquitousLanguageVersion is the only document version this package
// accepts.
const UbiquitousLanguageVersion = 1

// UbiquitousLanguage is the project's recorded domain: the software's
// name, the domain in a few sentences, one model per bounded context,
// and the context map between them. It is a value with invariants, not
// a second aggregate root: Rule stays the sole aggregate. A
// representation that violates a meta-model invariant the loader
// enforces never becomes a value; every constructor failure names the
// invariant it applies, so a project that records its language hears
// the meta-model at once.
type UbiquitousLanguage struct {
	Project     string
	Description string
	Contexts    []BoundedContext
	Relations   []ContextRelation
}

// ErrDefinitionNotFound is returned by lookups and mutations that name
// a recorded entry the language does not hold.
var ErrDefinitionNotFound = errors.New("domain definition not found")

// NewUbiquitousLanguage builds a validated language. The loader-level
// invariants of the meta-model are applied together: every bounded
// context, its aggregates, value objects, events, services,
// specifications, and questions, then the context map.
func NewUbiquitousLanguage(project, description string, contexts []BoundedContext, relations []ContextRelation) (UbiquitousLanguage, error) {
	lang := UbiquitousLanguage{
		Project:     strings.TrimSpace(project),
		Description: strings.TrimSpace(description),
		Contexts:    cloneContexts(contexts),
		Relations:   cloneRelations(relations),
	}
	if err := lang.validate(); err != nil {
		return UbiquitousLanguage{}, err
	}
	return lang, nil
}

// Empty reports a language that records nothing: no contexts and no
// relations. A project name alone is not a recorded domain.
func (l UbiquitousLanguage) Empty() bool {
	return len(l.Contexts) == 0 && len(l.Relations) == 0
}

// Context finds one bounded context by name.
func (l UbiquitousLanguage) Context(name string) (BoundedContext, bool) {
	for _, c := range l.Contexts {
		if c.Name == name {
			return c, true
		}
	}
	return BoundedContext{}, false
}

// ContextNames returns the recorded context names in file order.
func (l UbiquitousLanguage) ContextNames() []string {
	out := make([]string, 0, len(l.Contexts))
	for _, c := range l.Contexts {
		out = append(out, c.Name)
	}
	return out
}

// Relation finds the one relation recorded between two contexts, in
// either direction.
func (l UbiquitousLanguage) Relation(a, b string) (ContextRelation, bool) {
	for _, r := range l.Relations {
		if (r.From == a && r.To == b) || (r.From == b && r.To == a) {
			return r, true
		}
	}
	return ContextRelation{}, false
}

// Counts is the recorded language in numbers, for summaries.
type Counts struct {
	Contexts       int
	Aggregates     int
	Entities       int
	ValueObjects   int
	Invariants     int
	Assertions     int
	Specifications int
	Events         int
	Services       int
	Questions      int
	Relations      int
}

// Counts totals every recorded section. Entities counts member
// entities only; aggregates are counted under Aggregates.
func (l UbiquitousLanguage) Counts() Counts {
	c := Counts{Contexts: len(l.Contexts), Relations: len(l.Relations)}
	for _, ctx := range l.Contexts {
		c.Aggregates += len(ctx.Aggregates)
		c.ValueObjects += len(ctx.ValueObjects)
		c.Specifications += len(ctx.Specifications)
		c.Events += len(ctx.Events)
		c.Services += len(ctx.Services)
		c.Questions += len(ctx.Questions)
		for _, a := range ctx.Aggregates {
			c.Entities += len(a.Entities)
			c.Invariants += len(a.Invariants)
			c.Assertions += len(a.Assertions)
		}
		for _, v := range ctx.ValueObjects {
			c.Invariants += len(v.Invariants)
		}
	}
	return c
}

// validate applies the loader-level meta-model invariants. Each
// failure is prefixed with the invariant's id so the finding reads as
// the meta-model speaking, and with the line when the value came from
// a file.
func (l UbiquitousLanguage) validate() error {
	if l.Project == "" {
		return errors.New("domain: project is empty; the file names the software whose domain it records")
	}
	seen := map[string]int{}
	for i, c := range l.Contexts {
		if err := c.validate(); err != nil {
			return err
		}
		if j, dup := seen[c.Name]; dup {
			return fmt.Errorf("bounded_context/named-once: %scontext %q is recorded twice (also line %d)",
				at(c.Line), c.Name, l.Contexts[j].Line)
		}
		seen[c.Name] = i
	}
	return l.validateRelations()
}

func (l UbiquitousLanguage) validateRelations() error {
	pairs := map[[2]string]ContextRelation{}
	for _, r := range l.Relations {
		if r.From == "" || r.To == "" {
			return fmt.Errorf("context_map/relations-name-recorded-contexts: %srelation names an empty context", at(r.Line))
		}
		if _, ok := l.Context(r.From); !ok {
			return fmt.Errorf("context_map/relations-name-recorded-contexts: %srelation from %q: no such context", at(r.Line), r.From)
		}
		if _, ok := l.Context(r.To); !ok {
			return fmt.Errorf("context_map/relations-name-recorded-contexts: %srelation to %q: no such context", at(r.Line), r.To)
		}
		if r.From == r.To {
			return fmt.Errorf("context_map/one-relation-per-pair: %scontext %q is related to itself", at(r.Line), r.From)
		}
		if _, err := ParseRelationKind(string(r.Kind)); err != nil {
			return fmt.Errorf("context_relation/published-kind: %s%v", at(r.Line), err)
		}
		key := [2]string{r.From, r.To}
		if r.To < r.From {
			key = [2]string{r.To, r.From}
		}
		if prev, dup := pairs[key]; dup {
			return fmt.Errorf("context_map/one-relation-per-pair: %s%s and %s are already related (%s, line %d)",
				at(r.Line), r.From, r.To, prev.Kind, prev.Line)
		}
		pairs[key] = r
	}
	return nil
}

// at renders a line prefix for validation messages; nothing when the
// value did not come from a file.
func at(line int) string {
	if line <= 0 {
		return ""
	}
	return fmt.Sprintf("line %d: ", line)
}

// requireDefinition applies ubiquitous_language/terms-carry-definitions
// to one recorded term.
func requireDefinition(line int, what, name, definition string) error {
	if strings.TrimSpace(definition) == "" {
		return fmt.Errorf("ubiquitous_language/terms-carry-definitions: %s%s %q carries no definition", at(line), what, name)
	}
	return nil
}

func requireName(line int, what, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%s: %s%s has an empty name", "ubiquitous_language/terms-carry-definitions", at(line), what)
	}
	if name != strings.TrimSpace(name) {
		return fmt.Errorf("ubiquitous_language/one-meaning-per-name: %s%s %q has surrounding whitespace", at(line), what, name)
	}
	return nil
}

func cloneContexts(in []BoundedContext) []BoundedContext {
	if in == nil {
		return nil
	}
	out := make([]BoundedContext, len(in))
	for i, c := range in {
		out[i] = c.clone()
	}
	return out
}

func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	return append([]string(nil), in...)
}
