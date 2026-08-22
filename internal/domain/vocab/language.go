package vocab

import (
	"fmt"
	"strings"
)

// UbiquitousLanguageFileName is the committed filename of the project's
// recorded Ubiquitous Language, resolved beside rules.yaml.
const UbiquitousLanguageFileName = "ubiquitous-language.yaml"

// UbiquitousLanguageVersion is the only document version this package
// accepts.
const UbiquitousLanguageVersion = 1

// UbiquitousLanguage is the project's recorded Ubiquitous Language: a
// value with invariants, not a second aggregate root. Rule remains the
// sole aggregate.
type UbiquitousLanguage struct {
	Entities      []Entity
	ValueObjects  []Definition
	BusinessRules []Definition
	Events        []Definition
}

// NewUbiquitousLanguage validates and returns a UbiquitousLanguage.
// Every name must be non-empty after TrimSpace and unique within its
// own section (exact match).
func NewUbiquitousLanguage(entities []Entity, valueObjects, businessRules, events []Definition) (UbiquitousLanguage, error) {
	if err := validateEntities(entities); err != nil {
		return UbiquitousLanguage{}, err
	}
	if err := validateSection("value_objects", valueObjects); err != nil {
		return UbiquitousLanguage{}, err
	}
	if err := validateSection("business_rules", businessRules); err != nil {
		return UbiquitousLanguage{}, err
	}
	if err := validateSection("events", events); err != nil {
		return UbiquitousLanguage{}, err
	}
	return UbiquitousLanguage{
		Entities:      cloneEntities(entities),
		ValueObjects:  cloneDefs(valueObjects),
		BusinessRules: cloneDefs(businessRules),
		Events:        cloneDefs(events),
	}, nil
}

func validateEntities(entities []Entity) error {
	seen := make(map[string]struct{}, len(entities))
	for _, e := range entities {
		if strings.TrimSpace(e.Name) == "" {
			return fmt.Errorf("entities: name must be non-empty")
		}
		if _, dup := seen[e.Name]; dup {
			return fmt.Errorf("entities: duplicate name %q", e.Name)
		}
		seen[e.Name] = struct{}{}
	}
	return nil
}

func validateSection(section string, defs []Definition) error {
	seen := make(map[string]struct{}, len(defs))
	for _, d := range defs {
		if strings.TrimSpace(d.Name) == "" {
			return fmt.Errorf("%s: name must be non-empty", section)
		}
		if _, dup := seen[d.Name]; dup {
			return fmt.Errorf("%s: duplicate name %q", section, d.Name)
		}
		seen[d.Name] = struct{}{}
	}
	return nil
}

// Define create-or-updates a definition. ConceptAggregate operates on
// Entities and is equivalent to Define(ConceptEntity, name, ch with
// SetAggregate:true, Aggregate:true). ConceptEntity never clears an
// existing Aggregate designation unless Change.SetAggregate is set.
// New definitions append to their section. Unchanged compares final
// field values and reports OutcomeUnchanged when nothing differs.
func (l UbiquitousLanguage) Define(c Concept, name string, ch Change) (UbiquitousLanguage, DefineResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return l, DefineResult{}, fmt.Errorf("name must be non-empty")
	}
	if ch.SetAliases && ch.ClearAliases {
		return l, DefineResult{}, fmt.Errorf("aliases and clear-aliases are mutually exclusive")
	}

	// Aggregate define is Entity define with designation forced on.
	if c == ConceptAggregate {
		ch.SetAggregate = true
		ch.Aggregate = true
		c = ConceptEntity
	}

	switch c {
	case ConceptEntity:
		return l.defineEntity(name, ch)
	case ConceptValueObject, ConceptBusinessRule, ConceptEvent:
		return l.defineInSection(c, name, ch)
	default:
		return l, DefineResult{}, fmt.Errorf("unsupported domain concept %q", c)
	}
}

func (l UbiquitousLanguage) defineEntity(name string, ch Change) (UbiquitousLanguage, DefineResult, error) {
	out := l.clone()
	idx := indexOfEntity(out.Entities, name)

	var before Entity
	created := idx < 0
	if created {
		before = Entity{Definition: Definition{Name: name}}
	} else {
		before = out.Entities[idx]
	}

	after := before
	if ch.SetDefinition {
		after.Definition.Definition = ch.DefinitionText
	}
	if ch.ClearAliases {
		after.Aliases = nil
	} else if ch.SetAliases {
		after.Aliases = cloneStrings(ch.Aliases)
	}
	if ch.SetAggregate {
		after.Aggregate = ch.Aggregate
	}
	// ConceptEntity without SetAggregate leaves designation intact.

	// Compare against a zero-valued baseline for creates so designation
	// and supplied fields appear in Changed; against the prior record
	// for updates so omitted fields do not.
	changed := fieldDiffs(before, after)

	var outcome Outcome
	switch {
	case created:
		outcome = OutcomeCreated
		out.Entities = append(out.Entities, after)
	case len(changed) == 0:
		outcome = OutcomeUnchanged
	default:
		outcome = OutcomeUpdated
		out.Entities[idx] = after
	}

	return out, DefineResult{Outcome: outcome, Changed: changed}, nil
}

func (l UbiquitousLanguage) defineInSection(c Concept, name string, ch Change) (UbiquitousLanguage, DefineResult, error) {
	out := l.clone()
	section := out.section(c)
	idx := indexOf(*section, name)

	var before Definition
	created := idx < 0
	if created {
		before = Definition{Name: name}
	} else {
		before = (*section)[idx]
	}

	after := before
	if ch.SetDefinition {
		after.Definition = ch.DefinitionText
	}
	if ch.ClearAliases {
		after.Aliases = nil
	} else if ch.SetAliases {
		after.Aliases = cloneStrings(ch.Aliases)
	}

	// Non-entity concepts have no Aggregate designation; compare only
	// definition and aliases.
	changed := defFieldDiffs(before, after)

	var outcome Outcome
	switch {
	case created:
		outcome = OutcomeCreated
		*section = append(*section, after)
	case len(changed) == 0:
		outcome = OutcomeUnchanged
	default:
		outcome = OutcomeUpdated
		(*section)[idx] = after
	}

	return out, DefineResult{Outcome: outcome, Changed: changed}, nil
}

// fieldDiffs returns the subset of definition/aliases/aggregate that
// differ between two Entities, in that fixed order.
func fieldDiffs(before, after Entity) []string {
	changed := defFieldDiffs(before.Definition, after.Definition)
	if before.Aggregate != after.Aggregate {
		changed = append(changed, "aggregate")
	}
	return changed
}

// defFieldDiffs returns the subset of definition/aliases that differ,
// in that fixed order.
func defFieldDiffs(before, after Definition) []string {
	var changed []string
	if before.Definition != after.Definition {
		changed = append(changed, "definition")
	}
	if !stringSlicesEqual(before.Aliases, after.Aliases) {
		changed = append(changed, "aliases")
	}
	return changed
}

// Remove deletes a definition or clears an Aggregate designation.
// Missing names (or remove aggregate on a non-designated Entity) wrap
// ErrDefinitionNotFound with the vocabulary "no <type> named %q is
// defined in the project domain model".
func (l UbiquitousLanguage) Remove(c Concept, name string) (UbiquitousLanguage, RemoveResult, error) {
	out := l.clone()
	notFound := func() error {
		return fmt.Errorf("no %s named %q is defined in the project domain model: %w", c, name, ErrDefinitionNotFound)
	}

	switch c {
	case ConceptAggregate:
		idx := indexOfEntity(out.Entities, name)
		if idx < 0 || !out.Entities[idx].Aggregate {
			return l, RemoveResult{}, notFound()
		}
		out.Entities[idx].Aggregate = false
		return out, RemoveResult{EntityPreserved: true}, nil
	case ConceptEntity:
		idx := indexOfEntity(out.Entities, name)
		if idx < 0 {
			return l, RemoveResult{}, notFound()
		}
		out.Entities = append(out.Entities[:idx], out.Entities[idx+1:]...)
		return out, RemoveResult{}, nil
	case ConceptValueObject, ConceptBusinessRule, ConceptEvent:
		section := out.section(c)
		idx := indexOf(*section, name)
		if idx < 0 {
			return l, RemoveResult{}, notFound()
		}
		*section = append((*section)[:idx], (*section)[idx+1:]...)
		return out, RemoveResult{}, nil
	default:
		return l, RemoveResult{}, fmt.Errorf("unsupported domain concept %q", c)
	}
}

// FindEntity returns the Entity with the exact name. Matching is
// case-sensitive and does not trim.
func (l UbiquitousLanguage) FindEntity(name string) (Entity, bool) {
	for _, e := range l.Entities {
		if e.Name == name {
			return cloneEntity(e), true
		}
	}
	return Entity{}, false
}

// Find returns the definition with the exact name. ConceptEntity and
// ConceptAggregate return the embedded Definition; ConceptAggregate
// matches only designated Entities. Matching is case-sensitive and does
// not trim.
func (l UbiquitousLanguage) Find(c Concept, name string) (Definition, bool) {
	switch c {
	case ConceptAggregate:
		for _, e := range l.Entities {
			if e.Name == name && e.Aggregate {
				return cloneDef(e.Definition), true
			}
		}
		return Definition{}, false
	case ConceptEntity:
		if e, ok := l.FindEntity(name); ok {
			return e.Definition, true
		}
		return Definition{}, false
	case ConceptValueObject, ConceptBusinessRule, ConceptEvent:
		section := l.sectionRef(c)
		for _, d := range section {
			if d.Name == name {
				return cloneDef(d), true
			}
		}
		return Definition{}, false
	default:
		return Definition{}, false
	}
}

// ListEntities returns every Entity in file order.
func (l UbiquitousLanguage) ListEntities() []Entity {
	return cloneEntities(l.Entities)
}

// ListAggregates returns designated Entities in file order.
func (l UbiquitousLanguage) ListAggregates() []Entity {
	out := make([]Entity, 0, len(l.Entities))
	for _, e := range l.Entities {
		if e.Aggregate {
			out = append(out, cloneEntity(e))
		}
	}
	return out
}

// List returns definitions for the concept in file order. For
// ConceptEntity it returns the embedded Definition of every Entity; for
// ConceptAggregate, designated Entities only.
func (l UbiquitousLanguage) List(c Concept) []Definition {
	switch c {
	case ConceptAggregate:
		out := make([]Definition, 0, len(l.Entities))
		for _, e := range l.Entities {
			if e.Aggregate {
				out = append(out, cloneDef(e.Definition))
			}
		}
		return out
	case ConceptEntity:
		out := make([]Definition, len(l.Entities))
		for i, e := range l.Entities {
			out[i] = cloneDef(e.Definition)
		}
		return out
	case ConceptValueObject, ConceptBusinessRule, ConceptEvent:
		return cloneDefs(l.sectionRef(c))
	default:
		return nil
	}
}

// Counts returns the tallies for this UbiquitousLanguage.
func (l UbiquitousLanguage) Counts() Counts {
	aggregates := 0
	for _, e := range l.Entities {
		if e.Aggregate {
			aggregates++
		}
	}
	return Counts{
		Entities:      len(l.Entities),
		Aggregates:    aggregates,
		ValueObjects:  len(l.ValueObjects),
		BusinessRules: len(l.BusinessRules),
		Events:        len(l.Events),
	}
}

// Empty reports whether the UbiquitousLanguage records no definitions.
func (l UbiquitousLanguage) Empty() bool {
	return len(l.Entities) == 0 &&
		len(l.ValueObjects) == 0 &&
		len(l.BusinessRules) == 0 &&
		len(l.Events) == 0
}

func (l UbiquitousLanguage) clone() UbiquitousLanguage {
	return UbiquitousLanguage{
		Entities:      cloneEntities(l.Entities),
		ValueObjects:  cloneDefs(l.ValueObjects),
		BusinessRules: cloneDefs(l.BusinessRules),
		Events:        cloneDefs(l.Events),
	}
}

func (l *UbiquitousLanguage) section(c Concept) *[]Definition {
	switch c {
	case ConceptValueObject:
		return &l.ValueObjects
	case ConceptBusinessRule:
		return &l.BusinessRules
	case ConceptEvent:
		return &l.Events
	default:
		// Callers validate the concept before mutating; Entity has its
		// own slice and an unknown concept has no section.
		return nil
	}
}

func (l UbiquitousLanguage) sectionRef(c Concept) []Definition {
	switch c {
	case ConceptValueObject:
		return l.ValueObjects
	case ConceptBusinessRule:
		return l.BusinessRules
	case ConceptEvent:
		return l.Events
	default:
		return nil
	}
}

func indexOfEntity(entities []Entity, name string) int {
	for i, e := range entities {
		if e.Name == name {
			return i
		}
	}
	return -1
}

func indexOf(defs []Definition, name string) int {
	for i, d := range defs {
		if d.Name == name {
			return i
		}
	}
	return -1
}

func cloneEntities(in []Entity) []Entity {
	if in == nil {
		return nil
	}
	out := make([]Entity, len(in))
	for i, e := range in {
		out[i] = cloneEntity(e)
	}
	return out
}

func cloneEntity(e Entity) Entity {
	e.Definition = cloneDef(e.Definition)
	return e
}

func cloneDefs(in []Definition) []Definition {
	if in == nil {
		return nil
	}
	out := make([]Definition, len(in))
	for i, d := range in {
		out[i] = cloneDef(d)
	}
	return out
}

func cloneDef(d Definition) Definition {
	d.Aliases = cloneStrings(d.Aliases)
	return d
}

func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
