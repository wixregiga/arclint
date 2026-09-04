package vocab

import (
	"fmt"
	"strings"
)

// UbiquitousLanguageFileName is the committed filename of the project's
// recorded Ubiquitous Language, resolved beside the repository ruleset
// and named for the command group that owns it: arclint domain.
const UbiquitousLanguageFileName = "domain.arclint.yaml"

// UbiquitousLanguageVersion is the only document version this package
// accepts.
const UbiquitousLanguageVersion = 1

// UbiquitousLanguage is the project's recorded Ubiquitous Language
// organized by bounded context: a value with invariants, not a second
// aggregate root. Rule remains the sole aggregate.
type UbiquitousLanguage struct {
	Contexts  []BoundedContext
	Relations []ContextRelation
}

// NewUbiquitousLanguage validates and returns a UbiquitousLanguage.
// Every context name, term name, invariant statement, and owner must be
// non-empty after TrimSpace. Context names are unique. Term names are
// unique within their section within their context. Invariant and
// assertion statements are unique within a context. Cluster and
// assertion ids are unique within a context. A name may not appear in
// both value_objects and specifications. An invariant id is forbidden
// when the owner is a value object and is rejected when the owner is
// not an aggregate. Assertion requires id and on. Relation From/To
// must name declared contexts; Kind must be a valid RelationKind.
func NewUbiquitousLanguage(contexts []BoundedContext, relations []ContextRelation) (UbiquitousLanguage, error) {
	if err := validateContexts(contexts); err != nil {
		return UbiquitousLanguage{}, err
	}
	if err := validateRelations(contexts, relations); err != nil {
		return UbiquitousLanguage{}, err
	}
	return UbiquitousLanguage{
		Contexts:  cloneContexts(contexts),
		Relations: cloneRelations(relations),
	}, nil
}

func validateContexts(contexts []BoundedContext) error {
	seen := make(map[string]struct{}, len(contexts))
	for _, c := range contexts {
		name := strings.TrimSpace(c.Name)
		if name == "" {
			return fmt.Errorf("contexts: name must be non-empty")
		}
		if name != c.Name {
			return fmt.Errorf("contexts: name must be non-empty")
		}
		if _, dup := seen[c.Name]; dup {
			return fmt.Errorf("contexts: duplicate name %q", c.Name)
		}
		if c.Line < 0 {
			return fmt.Errorf("contexts %q: negative line", c.Name)
		}
		seen[c.Name] = struct{}{}
		if err := validateEntities(c.Name, c.Entities); err != nil {
			return err
		}
		if err := validateSection(c.Name, "value_objects", c.ValueObjects); err != nil {
			return err
		}
		if err := validateInvariants(c.Name, c); err != nil {
			return err
		}
		if err := validateAssertions(c.Name, c.Assertions); err != nil {
			return err
		}
		if err := validateSpecifications(c.Name, c); err != nil {
			return err
		}
		if err := validateContractIDs(c.Name, c); err != nil {
			return err
		}
		if err := validateSection(c.Name, "events", c.Events); err != nil {
			return err
		}
	}
	return nil
}

func validateEntities(contextName string, entities []Entity) error {
	seen := make(map[string]struct{}, len(entities))
	for _, e := range entities {
		if strings.TrimSpace(e.Name) == "" {
			return fmt.Errorf("contexts %q entities: name must be non-empty", contextName)
		}
		if _, dup := seen[e.Name]; dup {
			return fmt.Errorf("contexts %q entities: duplicate name %q", contextName, e.Name)
		}
		if e.Line < 0 {
			return fmt.Errorf("contexts %q entities: negative line on %q", contextName, e.Name)
		}
		if err := validateAliases(contextName, "entities", e.Definition); err != nil {
			return err
		}
		seen[e.Name] = struct{}{}
	}
	return nil
}

// validateAliases rejects the same alias recorded twice on one term:
// aliases are a set of synonyms, and a repetition is a slip, not a
// second synonym.
func validateAliases(contextName, section string, d Definition) error {
	seen := make(map[string]struct{}, len(d.Aliases))
	for _, alias := range d.Aliases {
		if _, dup := seen[alias]; dup {
			return fmt.Errorf("contexts %q %s: %q lists alias %q twice", contextName, section, d.Name, alias)
		}
		seen[alias] = struct{}{}
	}
	return nil
}

func validateSection(contextName, section string, defs []Definition) error {
	seen := make(map[string]struct{}, len(defs))
	for _, d := range defs {
		if strings.TrimSpace(d.Name) == "" {
			return fmt.Errorf("contexts %q %s: name must be non-empty", contextName, section)
		}
		if _, dup := seen[d.Name]; dup {
			return fmt.Errorf("contexts %q %s: duplicate name %q", contextName, section, d.Name)
		}
		if d.Line < 0 {
			return fmt.Errorf("contexts %q %s: negative line on %q", contextName, section, d.Name)
		}
		if err := validateAliases(contextName, section, d); err != nil {
			return err
		}
		seen[d.Name] = struct{}{}
	}
	return nil
}

func validateInvariants(contextName string, ctx BoundedContext) error {
	seen := make(map[string]struct{}, len(ctx.Invariants))
	for _, inv := range ctx.Invariants {
		if strings.TrimSpace(inv.Statement) == "" {
			return fmt.Errorf("contexts %q invariants: statement must be non-empty", contextName)
		}
		if strings.TrimSpace(inv.Owner) == "" {
			return fmt.Errorf("contexts %q invariants: owner must be non-empty", contextName)
		}
		if _, dup := seen[inv.Statement]; dup {
			return fmt.Errorf("contexts %q invariants: duplicate statement %q", contextName, inv.Statement)
		}
		if inv.Line < 0 {
			return fmt.Errorf("contexts %q invariants: negative line on %q", contextName, inv.Statement)
		}
		id := strings.TrimSpace(inv.ID)
		if id != inv.ID {
			return fmt.Errorf("contexts %q invariants: id must be non-empty", contextName)
		}
		agg, vo := classifyOwner(ctx, inv.Owner)
		if vo && id != "" {
			return fmt.Errorf("contexts %q invariants: id is forbidden when owner %q is a value object", contextName, inv.Owner)
		}
		if id != "" && !agg {
			return fmt.Errorf("contexts %q invariants: id is only legal when owner %q is an aggregate", contextName, inv.Owner)
		}
		seen[inv.Statement] = struct{}{}
	}
	return nil
}

func validateAssertions(contextName string, assertions []Assertion) error {
	seen := make(map[string]struct{}, len(assertions))
	for _, a := range assertions {
		if strings.TrimSpace(a.Statement) == "" {
			return fmt.Errorf("contexts %q assertions: statement must be non-empty", contextName)
		}
		if strings.TrimSpace(a.Owner) == "" {
			return fmt.Errorf("contexts %q assertions: owner must be non-empty", contextName)
		}
		if strings.TrimSpace(a.ID) == "" {
			return fmt.Errorf("contexts %q assertions: id must be non-empty", contextName)
		}
		if strings.TrimSpace(a.On) == "" {
			return fmt.Errorf("contexts %q assertions: on must be non-empty", contextName)
		}
		if _, dup := seen[a.Statement]; dup {
			return fmt.Errorf("contexts %q assertions: duplicate statement %q", contextName, a.Statement)
		}
		if a.Line < 0 {
			return fmt.Errorf("contexts %q assertions: negative line on %q", contextName, a.Statement)
		}
		seen[a.Statement] = struct{}{}
	}
	return nil
}

func validateSpecifications(contextName string, ctx BoundedContext) error {
	seen := make(map[string]struct{}, len(ctx.Specifications))
	vo := make(map[string]struct{}, len(ctx.ValueObjects))
	for _, d := range ctx.ValueObjects {
		vo[d.Name] = struct{}{}
	}
	for _, s := range ctx.Specifications {
		if strings.TrimSpace(s.Name) == "" {
			return fmt.Errorf("contexts %q specifications: name must be non-empty", contextName)
		}
		if strings.TrimSpace(s.Definition) == "" {
			return fmt.Errorf("contexts %q specifications: definition must be non-empty", contextName)
		}
		if _, dup := seen[s.Name]; dup {
			return fmt.Errorf("contexts %q specifications: duplicate name %q", contextName, s.Name)
		}
		if _, clash := vo[s.Name]; clash {
			return fmt.Errorf("contexts %q: %q is both a value object and a specification", contextName, s.Name)
		}
		if s.Line < 0 {
			return fmt.Errorf("contexts %q specifications: negative line on %q", contextName, s.Name)
		}
		seen[s.Name] = struct{}{}
	}
	return nil
}

func validateContractIDs(contextName string, ctx BoundedContext) error {
	seen := make(map[string]string)
	for _, inv := range ctx.Invariants {
		id := strings.TrimSpace(inv.ID)
		if id == "" {
			continue
		}
		if prev, ok := seen[id]; ok {
			return fmt.Errorf("contexts %q: duplicate id %q (%s and invariants)", contextName, id, prev)
		}
		seen[id] = "invariants"
	}
	for _, a := range ctx.Assertions {
		id := strings.TrimSpace(a.ID)
		if id == "" {
			continue
		}
		if prev, ok := seen[id]; ok {
			return fmt.Errorf("contexts %q: duplicate id %q (%s and assertions)", contextName, id, prev)
		}
		seen[id] = "assertions"
	}
	return nil
}

func classifyOwner(ctx BoundedContext, owner string) (aggregate, valueObject bool) {
	for _, e := range ctx.Entities {
		if e.Name == owner {
			return e.Aggregate, false
		}
	}
	for _, v := range ctx.ValueObjects {
		if v.Name == owner {
			return false, true
		}
	}
	return false, false
}

func validateRelations(contexts []BoundedContext, relations []ContextRelation) error {
	names := make(map[string]struct{}, len(contexts))
	for _, c := range contexts {
		names[c.Name] = struct{}{}
	}
	seen := make(map[ContextRelation]struct{}, len(relations))
	for _, r := range relations {
		if strings.TrimSpace(r.From) == "" {
			return fmt.Errorf("relations: from must be non-empty")
		}
		if strings.TrimSpace(r.To) == "" {
			return fmt.Errorf("relations: to must be non-empty")
		}
		if _, ok := names[r.From]; !ok {
			return fmt.Errorf("relations: from %q is not a declared context", r.From)
		}
		if _, ok := names[r.To]; !ok {
			return fmt.Errorf("relations: to %q is not a declared context", r.To)
		}
		if _, err := ParseRelationKind(string(r.Kind)); err != nil {
			return fmt.Errorf("relations: %w", err)
		}
		if r.Line < 0 {
			return fmt.Errorf("relations: negative line on %q -> %q", r.From, r.To)
		}
		edge := ContextRelation{From: r.From, To: r.To, Kind: r.Kind}
		if _, dup := seen[edge]; dup {
			return fmt.Errorf("relations: %q -> %q (%s) is recorded twice", r.From, r.To, r.Kind)
		}
		seen[edge] = struct{}{}
	}
	return nil
}

// Define create-or-updates a definition inside contextName. An unknown
// context is created empty before the definition is applied.
// ConceptAggregate and ConceptAggregateRoot operate on Entities and
// force Aggregate designation on. ConceptEntity never clears an
// existing Aggregate designation unless Change.SetAggregate is set.
// ConceptInvariant and ConceptBusinessRule resolve into the invariants
// section (name is the statement) and require an owner on create.
// ConceptAssertion records into assertions and requires owner, id, and
// on at create. ConceptSpecification records into specifications.
// ConceptBoundedContext ensures the named context exists (name is the
// context name; contextName should match name). ConceptDomainEvent
// records into events. New definitions append.
// Unchanged compares final field values and reports OutcomeUnchanged
// when nothing differs.
func (l UbiquitousLanguage) Define(c Concept, contextName, name string, ch Change) (UbiquitousLanguage, DefineResult, error) {
	contextName = strings.TrimSpace(contextName)
	name = strings.TrimSpace(name)
	if ch.SetAliases && ch.ClearAliases {
		return l, DefineResult{}, fmt.Errorf("aliases and clear-aliases are mutually exclusive")
	}

	if c == ConceptBoundedContext {
		if name == "" {
			return l, DefineResult{}, fmt.Errorf("name must be non-empty")
		}
		// bounded_context: the term name is the context name.
		if contextName == "" {
			contextName = name
		}
		if contextName != name {
			return l, DefineResult{}, fmt.Errorf("bounded_context name %q must match context %q", name, contextName)
		}
		out := l.clone()
		if _, ok := out.contextIndex(contextName); ok {
			return out, DefineResult{Outcome: OutcomeUnchanged}, nil
		}
		out.Contexts = append(out.Contexts, BoundedContext{Name: contextName})
		return out, DefineResult{Outcome: OutcomeCreated}, nil
	}

	if contextName == "" {
		return l, DefineResult{}, fmt.Errorf("context name must be non-empty")
	}
	if name == "" {
		return l, DefineResult{}, fmt.Errorf("name must be non-empty")
	}

	// Aggregate / aggregate_root define is Entity define with designation forced on.
	if c == ConceptAggregate || c == ConceptAggregateRoot {
		ch.SetAggregate = true
		ch.Aggregate = true
		c = ConceptEntity
	}

	out := l.clone()
	ci := out.ensureContext(contextName)

	switch c {
	case ConceptEntity:
		return out.defineEntity(ci, name, ch)
	case ConceptValueObject, ConceptDomainEvent:
		return out.defineInSection(ci, c, name, ch)
	case ConceptSpecification:
		return out.defineSpecification(ci, name, ch)
	case ConceptInvariant, ConceptBusinessRule:
		return out.defineInvariant(ci, name, ch)
	case ConceptAssertion:
		return out.defineAssertion(ci, name, ch)
	default:
		return l, DefineResult{}, fmt.Errorf("unsupported domain concept %q", c)
	}
}

func (l *UbiquitousLanguage) ensureContext(name string) int {
	if i, ok := l.contextIndex(name); ok {
		return i
	}
	l.Contexts = append(l.Contexts, BoundedContext{Name: name})
	return len(l.Contexts) - 1
}

func (l UbiquitousLanguage) contextIndex(name string) (int, bool) {
	for i, c := range l.Contexts {
		if c.Name == name {
			return i, true
		}
	}
	return -1, false
}

func (l UbiquitousLanguage) defineEntity(ci int, name string, ch Change) (UbiquitousLanguage, DefineResult, error) {
	ctx := &l.Contexts[ci]
	idx := indexOfEntity(ctx.Entities, name)

	var before Entity
	created := idx < 0
	if created {
		before = Entity{Definition: Definition{Name: name}}
	} else {
		before = ctx.Entities[idx]
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

	changed := fieldDiffs(before, after)

	var outcome Outcome
	switch {
	case created:
		outcome = OutcomeCreated
		ctx.Entities = append(ctx.Entities, after)
	case len(changed) == 0:
		outcome = OutcomeUnchanged
	default:
		outcome = OutcomeUpdated
		ctx.Entities[idx] = after
	}
	return l, DefineResult{Outcome: outcome, Changed: changed}, nil
}

func (l UbiquitousLanguage) defineInSection(ci int, c Concept, name string, ch Change) (UbiquitousLanguage, DefineResult, error) {
	section := l.Contexts[ci].section(c)
	if section == nil {
		return l, DefineResult{}, fmt.Errorf("unsupported domain concept %q", c)
	}
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
	return l, DefineResult{Outcome: outcome, Changed: changed}, nil
}

func (l UbiquitousLanguage) defineInvariant(ci int, statement string, ch Change) (UbiquitousLanguage, DefineResult, error) {
	ctx := &l.Contexts[ci]
	idx := indexOfInvariant(ctx.Invariants, statement)

	var before Invariant
	created := idx < 0
	if created {
		before = Invariant{Statement: statement}
	} else {
		before = ctx.Invariants[idx]
	}

	after, err := applyInvariantChange(before, created, ch)
	if err != nil {
		return l, DefineResult{}, err
	}

	var changed []string
	if before.Owner != after.Owner {
		changed = append(changed, "owner")
	}
	if before.ID != after.ID {
		changed = append(changed, "id")
	}

	var outcome Outcome
	switch {
	case created:
		outcome = OutcomeCreated
		ctx.Invariants = append(ctx.Invariants, after)
	case len(changed) == 0:
		outcome = OutcomeUnchanged
	default:
		outcome = OutcomeUpdated
		ctx.Invariants[idx] = after
	}
	return l, DefineResult{Outcome: outcome, Changed: changed}, nil
}

func (l UbiquitousLanguage) defineAssertion(ci int, statement string, ch Change) (UbiquitousLanguage, DefineResult, error) {
	ctx := &l.Contexts[ci]
	idx := indexOfAssertion(ctx.Assertions, statement)

	var before Assertion
	created := idx < 0
	if created {
		before = Assertion{Statement: statement}
	} else {
		before = ctx.Assertions[idx]
	}

	after, err := applyAssertionChange(before, created, ch)
	if err != nil {
		return l, DefineResult{}, err
	}

	var changed []string
	if before.Owner != after.Owner {
		changed = append(changed, "owner")
	}
	if before.ID != after.ID {
		changed = append(changed, "id")
	}
	if before.On != after.On {
		changed = append(changed, "on")
	}

	var outcome Outcome
	switch {
	case created:
		outcome = OutcomeCreated
		ctx.Assertions = append(ctx.Assertions, after)
	case len(changed) == 0:
		outcome = OutcomeUnchanged
	default:
		outcome = OutcomeUpdated
		ctx.Assertions[idx] = after
	}
	return l, DefineResult{Outcome: outcome, Changed: changed}, nil
}

func applyInvariantChange(before Invariant, created bool, ch Change) (Invariant, error) {
	after := before
	if created {
		owner := strings.TrimSpace(ch.Owner)
		if owner == "" {
			return Invariant{}, fmt.Errorf("owner must be non-empty")
		}
		after.Owner = owner
		after.ID = strings.TrimSpace(ch.ID)
		return after, nil
	}
	if ch.SetOwner {
		owner := strings.TrimSpace(ch.Owner)
		if owner == "" {
			return Invariant{}, fmt.Errorf("owner must be non-empty")
		}
		after.Owner = owner
	}
	if ch.SetID {
		after.ID = strings.TrimSpace(ch.ID)
	}
	return after, nil
}

func applyAssertionChange(before Assertion, created bool, ch Change) (Assertion, error) {
	after := before
	if created {
		owner := strings.TrimSpace(ch.Owner)
		if owner == "" {
			return Assertion{}, fmt.Errorf("owner must be non-empty")
		}
		id := strings.TrimSpace(ch.ID)
		if id == "" {
			return Assertion{}, fmt.Errorf("id must be non-empty")
		}
		on := strings.TrimSpace(ch.On)
		if on == "" {
			return Assertion{}, fmt.Errorf("on must be non-empty")
		}
		after.Owner = owner
		after.ID = id
		after.On = on
		return after, nil
	}
	if ch.SetOwner {
		owner := strings.TrimSpace(ch.Owner)
		if owner == "" {
			return Assertion{}, fmt.Errorf("owner must be non-empty")
		}
		after.Owner = owner
	}
	if ch.SetID {
		id := strings.TrimSpace(ch.ID)
		if id == "" {
			return Assertion{}, fmt.Errorf("id must be non-empty")
		}
		after.ID = id
	}
	if ch.SetOn {
		on := strings.TrimSpace(ch.On)
		if on == "" {
			return Assertion{}, fmt.Errorf("on must be non-empty")
		}
		after.On = on
	}
	return after, nil
}

func (l UbiquitousLanguage) defineSpecification(ci int, name string, ch Change) (UbiquitousLanguage, DefineResult, error) {
	ctx := &l.Contexts[ci]
	idx := indexOfSpecification(ctx.Specifications, name)

	var before Specification
	created := idx < 0
	if created {
		before = Specification{Name: name}
	} else {
		before = ctx.Specifications[idx]
	}

	after := before
	if ch.SetDefinition {
		after.Definition = ch.DefinitionText
	}

	var changed []string
	if before.Definition != after.Definition {
		changed = append(changed, "definition")
	}

	var outcome Outcome
	switch {
	case created:
		outcome = OutcomeCreated
		ctx.Specifications = append(ctx.Specifications, after)
	case len(changed) == 0:
		outcome = OutcomeUnchanged
	default:
		outcome = OutcomeUpdated
		ctx.Specifications[idx] = after
	}
	return l, DefineResult{Outcome: outcome, Changed: changed}, nil
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

// removeContextAt deletes the context at idx when it is empty and
// unreferenced by relations; failures return the original language.
func removeContextAt(orig, out UbiquitousLanguage, idx int) (UbiquitousLanguage, RemoveResult, error) {
	target := out.Contexts[idx]
	if !contextIsEmpty(target) {
		return orig, RemoveResult{}, fmt.Errorf("bounded_context %q is not empty", target.Name)
	}
	for _, r := range out.Relations {
		if r.From == target.Name || r.To == target.Name {
			return orig, RemoveResult{}, fmt.Errorf("bounded_context %q is referenced by a relation", target.Name)
		}
	}
	out.Contexts = append(out.Contexts[:idx], out.Contexts[idx+1:]...)
	return out, RemoveResult{}, nil
}

// Remove deletes a definition or clears an Aggregate designation inside
// contextName. Missing names (or remove aggregate on a non-designated
// Entity) wrap ErrDefinitionNotFound with the vocabulary
// "no <type> named %q is defined in the project domain model".
// For invariants/assertions/business_rules, name is the statement.
func (l UbiquitousLanguage) Remove(c Concept, contextName, name string) (UbiquitousLanguage, RemoveResult, error) {
	out := l.clone()
	notFound := func() error {
		return fmt.Errorf("no %s named %q is defined in the project domain model: %w", c, name, ErrDefinitionNotFound)
	}

	if c == ConceptBoundedContext {
		key := name
		idx, ok := out.contextIndex(key)
		if !ok && contextName != "" {
			key = contextName
			idx, ok = out.contextIndex(key)
		}
		if !ok {
			return l, RemoveResult{}, fmt.Errorf("no %s named %q is defined in the project domain model: %w", c, key, ErrDefinitionNotFound)
		}
		return removeContextAt(l, out, idx)
	}

	ci, ok := out.contextIndex(contextName)
	if !ok {
		return l, RemoveResult{}, notFound()
	}
	ctx := &out.Contexts[ci]

	switch c {
	case ConceptAggregate, ConceptAggregateRoot:
		idx := indexOfEntity(ctx.Entities, name)
		if idx < 0 || !ctx.Entities[idx].Aggregate {
			return l, RemoveResult{}, notFound()
		}
		ctx.Entities[idx].Aggregate = false
		return out, RemoveResult{EntityPreserved: true}, nil
	case ConceptEntity:
		idx := indexOfEntity(ctx.Entities, name)
		if idx < 0 {
			return l, RemoveResult{}, notFound()
		}
		ctx.Entities = append(ctx.Entities[:idx], ctx.Entities[idx+1:]...)
		return out, RemoveResult{}, nil
	case ConceptValueObject, ConceptDomainEvent:
		section := ctx.section(c)
		idx := indexOf(*section, name)
		if idx < 0 {
			return l, RemoveResult{}, notFound()
		}
		*section = append((*section)[:idx], (*section)[idx+1:]...)
		return out, RemoveResult{}, nil
	case ConceptInvariant, ConceptBusinessRule:
		idx := indexOfInvariant(ctx.Invariants, name)
		if idx < 0 {
			return l, RemoveResult{}, notFound()
		}
		ctx.Invariants = append(ctx.Invariants[:idx], ctx.Invariants[idx+1:]...)
		return out, RemoveResult{}, nil
	case ConceptAssertion:
		idx := indexOfAssertion(ctx.Assertions, name)
		if idx < 0 {
			return l, RemoveResult{}, notFound()
		}
		ctx.Assertions = append(ctx.Assertions[:idx], ctx.Assertions[idx+1:]...)
		return out, RemoveResult{}, nil
	case ConceptSpecification:
		idx := indexOfSpecification(ctx.Specifications, name)
		if idx < 0 {
			return l, RemoveResult{}, notFound()
		}
		ctx.Specifications = append(ctx.Specifications[:idx], ctx.Specifications[idx+1:]...)
		return out, RemoveResult{}, nil
	default:
		return l, RemoveResult{}, fmt.Errorf("unsupported domain concept %q", c)
	}
}

// FindEntity returns the Entity with the exact name inside contextName.
// Matching is case-sensitive and does not trim.
func (l UbiquitousLanguage) FindEntity(contextName, name string) (Entity, bool) {
	ci, ok := l.contextIndex(contextName)
	if !ok {
		return Entity{}, false
	}
	for _, e := range l.Contexts[ci].Entities {
		if e.Name == name {
			return cloneEntity(e), true
		}
	}
	return Entity{}, false
}

// FindInvariant returns the Invariant with the exact statement inside
// contextName. Matching is case-sensitive and does not trim.
func (l UbiquitousLanguage) FindInvariant(contextName, statement string) (Invariant, bool) {
	ci, ok := l.contextIndex(contextName)
	if !ok {
		return Invariant{}, false
	}
	for _, inv := range l.Contexts[ci].Invariants {
		if inv.Statement == statement {
			return inv, true
		}
	}
	return Invariant{}, false
}

// Find returns the definition with the exact name inside contextName.
// ConceptEntity, ConceptAggregate, and ConceptAggregateRoot return the
// embedded Definition; aggregate kinds match only designated Entities.
// ConceptInvariant, ConceptAssertion, and ConceptBusinessRule return
// Definition{Name: statement, Definition: owner}. Matching is
// case-sensitive and does not trim.
func (l UbiquitousLanguage) Find(c Concept, contextName, name string) (Definition, bool) {
	if c == ConceptBoundedContext {
		if i, ok := l.contextIndex(name); ok {
			return Definition{Name: name, Line: l.Contexts[i].Line}, true
		}
		if i, ok := l.contextIndex(contextName); ok && (name == "" || name == contextName) {
			return Definition{Name: contextName, Line: l.Contexts[i].Line}, true
		}
		return Definition{}, false
	}

	ci, ok := l.contextIndex(contextName)
	if !ok {
		return Definition{}, false
	}
	ctx := l.Contexts[ci]

	switch c {
	case ConceptAggregate, ConceptAggregateRoot:
		for _, e := range ctx.Entities {
			if e.Name == name && e.Aggregate {
				return cloneDef(e.Definition), true
			}
		}
		return Definition{}, false
	case ConceptEntity:
		if e, ok := l.FindEntity(contextName, name); ok {
			return e.Definition, true
		}
		return Definition{}, false
	case ConceptValueObject, ConceptDomainEvent:
		section := ctx.sectionRef(c)
		for _, d := range section {
			if d.Name == name {
				return cloneDef(d), true
			}
		}
		return Definition{}, false
	case ConceptInvariant, ConceptBusinessRule:
		if inv, ok := l.FindInvariant(contextName, name); ok {
			return Definition{Name: inv.Statement, Definition: inv.Owner, Line: inv.Line}, true
		}
		return Definition{}, false
	case ConceptAssertion:
		if a, ok := l.FindAssertion(contextName, name); ok {
			return Definition{Name: a.Statement, Definition: a.Owner, Line: a.Line}, true
		}
		return Definition{}, false
	case ConceptSpecification:
		for _, s := range ctx.Specifications {
			if s.Name == name {
				return Definition{Name: s.Name, Definition: s.Definition, Line: s.Line}, true
			}
		}
		return Definition{}, false
	default:
		return Definition{}, false
	}
}

// ListContexts returns every BoundedContext in file order.
func (l UbiquitousLanguage) ListContexts() []BoundedContext {
	return cloneContexts(l.Contexts)
}

// ListEntities returns every Entity in contextName in file order.
func (l UbiquitousLanguage) ListEntities(contextName string) []Entity {
	ci, ok := l.contextIndex(contextName)
	if !ok {
		return nil
	}
	return cloneEntities(l.Contexts[ci].Entities)
}

// ListAggregates returns designated Entities in contextName in file order.
func (l UbiquitousLanguage) ListAggregates(contextName string) []Entity {
	ci, ok := l.contextIndex(contextName)
	if !ok {
		return nil
	}
	out := make([]Entity, 0, len(l.Contexts[ci].Entities))
	for _, e := range l.Contexts[ci].Entities {
		if e.Aggregate {
			out = append(out, cloneEntity(e))
		}
	}
	return out
}

// ListInvariants returns invariants in contextName in file order.
func (l UbiquitousLanguage) ListInvariants(contextName string) []Invariant {
	ci, ok := l.contextIndex(contextName)
	if !ok {
		return nil
	}
	return cloneInvariants(l.Contexts[ci].Invariants)
}

// FindAssertion returns the Assertion with the exact statement inside
// contextName. Matching is case-sensitive and does not trim.
func (l UbiquitousLanguage) FindAssertion(contextName, statement string) (Assertion, bool) {
	ci, ok := l.contextIndex(contextName)
	if !ok {
		return Assertion{}, false
	}
	for _, a := range l.Contexts[ci].Assertions {
		if a.Statement == statement {
			return a, true
		}
	}
	return Assertion{}, false
}

// ListAssertions returns assertions in contextName in file order.
func (l UbiquitousLanguage) ListAssertions(contextName string) []Assertion {
	ci, ok := l.contextIndex(contextName)
	if !ok {
		return nil
	}
	return cloneAssertions(l.Contexts[ci].Assertions)
}

// FindSpecification returns the Specification with the exact name
// inside contextName. Matching is case-sensitive and does not trim.
func (l UbiquitousLanguage) FindSpecification(contextName, name string) (Specification, bool) {
	ci, ok := l.contextIndex(contextName)
	if !ok {
		return Specification{}, false
	}
	for _, s := range l.Contexts[ci].Specifications {
		if s.Name == name {
			return s, true
		}
	}
	return Specification{}, false
}

// ListSpecifications returns specifications in contextName in file order.
func (l UbiquitousLanguage) ListSpecifications(contextName string) []Specification {
	ci, ok := l.contextIndex(contextName)
	if !ok {
		return nil
	}
	return cloneSpecifications(l.Contexts[ci].Specifications)
}

// List returns definitions for the concept in contextName in file order.
// For ConceptEntity it returns the embedded Definition of every Entity;
// for aggregate kinds, designated Entities only. For invariant kinds,
// each entry is Definition{Name: statement, Definition: owner}.
// ConceptBoundedContext returns a single-element slice when the context
// exists.
func (l UbiquitousLanguage) List(c Concept, contextName string) []Definition {
	if c == ConceptBoundedContext {
		if contextName == "" {
			out := make([]Definition, len(l.Contexts))
			for i, ctx := range l.Contexts {
				out[i] = Definition{Name: ctx.Name, Line: ctx.Line}
			}
			return out
		}
		if i, ok := l.contextIndex(contextName); ok {
			return []Definition{{Name: contextName, Line: l.Contexts[i].Line}}
		}
		return nil
	}

	ci, ok := l.contextIndex(contextName)
	if !ok {
		return nil
	}
	ctx := l.Contexts[ci]

	switch c {
	case ConceptAggregate, ConceptAggregateRoot:
		out := make([]Definition, 0, len(ctx.Entities))
		for _, e := range ctx.Entities {
			if e.Aggregate {
				out = append(out, cloneDef(e.Definition))
			}
		}
		return out
	case ConceptEntity:
		out := make([]Definition, len(ctx.Entities))
		for i, e := range ctx.Entities {
			out[i] = cloneDef(e.Definition)
		}
		return out
	case ConceptValueObject, ConceptDomainEvent:
		return cloneDefs(ctx.sectionRef(c))
	case ConceptInvariant, ConceptBusinessRule:
		out := make([]Definition, len(ctx.Invariants))
		for i, inv := range ctx.Invariants {
			out[i] = Definition{Name: inv.Statement, Definition: inv.Owner, Line: inv.Line}
		}
		return out
	case ConceptAssertion:
		out := make([]Definition, len(ctx.Assertions))
		for i, a := range ctx.Assertions {
			out[i] = Definition{Name: a.Statement, Definition: a.Owner, Line: a.Line}
		}
		return out
	case ConceptSpecification:
		out := make([]Definition, len(ctx.Specifications))
		for i, s := range ctx.Specifications {
			out[i] = Definition{Name: s.Name, Definition: s.Definition, Line: s.Line}
		}
		return out
	default:
		return nil
	}
}

// Counts returns the tallies for this UbiquitousLanguage.
func (l UbiquitousLanguage) Counts() Counts {
	var entities, aggregates, valueObjects, invariants, assertions, specifications, events int
	for _, c := range l.Contexts {
		entities += len(c.Entities)
		for _, e := range c.Entities {
			if e.Aggregate {
				aggregates++
			}
		}
		valueObjects += len(c.ValueObjects)
		invariants += len(c.Invariants)
		assertions += len(c.Assertions)
		specifications += len(c.Specifications)
		events += len(c.Events)
	}
	return Counts{
		Contexts:       len(l.Contexts),
		Entities:       entities,
		Aggregates:     aggregates,
		ValueObjects:   valueObjects,
		Invariants:     invariants,
		Assertions:     assertions,
		Specifications: specifications,
		Events:         events,
		Relations:      len(l.Relations),
	}
}

// Empty reports whether the UbiquitousLanguage records no contexts and
// no relations.
func (l UbiquitousLanguage) Empty() bool {
	return len(l.Contexts) == 0 && len(l.Relations) == 0
}

func (l UbiquitousLanguage) clone() UbiquitousLanguage {
	return UbiquitousLanguage{
		Contexts:  cloneContexts(l.Contexts),
		Relations: cloneRelations(l.Relations),
	}
}

func (c *BoundedContext) section(concept Concept) *[]Definition {
	switch concept {
	case ConceptValueObject:
		return &c.ValueObjects
	case ConceptDomainEvent:
		return &c.Events
	default:
		return nil
	}
}

func (c BoundedContext) sectionRef(concept Concept) []Definition {
	switch concept {
	case ConceptValueObject:
		return c.ValueObjects
	case ConceptDomainEvent:
		return c.Events
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

func indexOfInvariant(invs []Invariant, statement string) int {
	for i, inv := range invs {
		if inv.Statement == statement {
			return i
		}
	}
	return -1
}

func indexOfAssertion(assertions []Assertion, statement string) int {
	for i, a := range assertions {
		if a.Statement == statement {
			return i
		}
	}
	return -1
}

func indexOfSpecification(specs []Specification, name string) int {
	for i, s := range specs {
		if s.Name == name {
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
