package vocab

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrChangeIncomplete is returned by Define and Remove when the request
// does not say enough to carry out: no name, no bounded context, no
// owner where the concept is nested, or a required property left
// unset when the entry is first recorded. Nothing about the recorded
// language is wrong; the request is.
var ErrChangeIncomplete = errors.New("the change is incomplete")

// ErrChangeMisapplied is returned by Define and Remove when the request
// says something the concept cannot take: a property the concept's
// building block does not record, or an owner for a concept the
// context records directly.
var ErrChangeMisapplied = errors.New("the change is misapplied")

// Locator names one recorded entry of the language: the bounded
// context it is in, the owner it is recorded under when the concept is
// nested (the aggregate of a member entity, the aggregate or value
// object of an invariant, the aggregate of an assertion), and its name
// or key. A bounded context is located by Name alone. Owner may be left
// empty for an entry that already exists when the name finds it alone.
type Locator struct {
	Context string
	Owner   string
	Name    string
}

// Change is one recording decision: the properties to set on an entry.
// Each property has a Set flag so that an empty value can be recorded
// deliberately (clearing an event's raised_by, an aggregate's aliases)
// and an untouched property stays as it is. The property names are the
// meta-model's recorded properties of the concept; a property the
// concept does not record is refused.
type Change struct {
	SetDefinition bool
	Definition    string
	SetStatement  bool
	Statement     string
	SetOn         bool
	On            string
	SetText       bool
	Text          string
	SetIdentity   bool
	Identity      string
	SetAliases    bool
	Aliases       []string
	SetRepository bool
	Repository    string
	SetFactory    bool
	Factory       string
	SetRaisedBy   bool
	RaisedBy      string
}

// The recorded property names, as the meta-model spells them.
const (
	propDefinition = "definition"
	propStatement  = "statement"
	propOn         = "on"
	propText       = "text"
	propIdentity   = "identity"
	propAliases    = "aliases"
	propRepository = "repository"
	propFactory    = "factory"
	propRaisedBy   = "raised_by"
	propName       = "name"
	propKey        = "key"
)

// set lists the properties the change sets, in meta-model order.
func (ch Change) set() []string {
	var out []string
	add := func(on bool, prop string) {
		if on {
			out = append(out, prop)
		}
	}
	add(ch.SetDefinition, propDefinition)
	add(ch.SetIdentity, propIdentity)
	add(ch.SetAliases, propAliases)
	add(ch.SetRepository, propRepository)
	add(ch.SetFactory, propFactory)
	add(ch.SetRaisedBy, propRaisedBy)
	add(ch.SetOn, propOn)
	add(ch.SetStatement, propStatement)
	add(ch.SetText, propText)
	return out
}

// applicable refuses a property the concept's building block does not
// record.
func (ch Change) applicable(c Concept) error {
	block, ok := DDD().Block(string(c))
	if !ok {
		return fmt.Errorf("%s has no building block", c)
	}
	recorded := make([]string, 0, len(block.Records.Properties))
	for _, p := range block.Records.Properties {
		recorded = append(recorded, p.Name)
	}
	for _, p := range ch.set() {
		if !slices.Contains(recorded, p) {
			return fmt.Errorf("%w: %s %s records no %s; it records %s", ErrChangeMisapplied,
				article(block.Title), strings.ToLower(block.Title), p, strings.Join(recorded, ", "))
		}
	}
	return nil
}

// requiredToRecord refuses to create an entry without every property
// the meta-model marks required, the name or key aside.
func (ch Change) requiredToRecord(c Concept, what, name string) error {
	block, _ := DDD().Block(string(c))
	set := ch.set()
	var missing []string
	for _, p := range block.Records.Properties {
		if !p.Required || p.Name == propName || p.Name == propKey {
			continue
		}
		if !slices.Contains(set, p.Name) {
			missing = append(missing, p.Name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s %q is not recorded; recording one needs %s", ErrChangeIncomplete, what, name, strings.Join(missing, " and "))
}

func article(title string) string {
	switch strings.ToLower(title[:1]) {
	case "a", "e", "i", "o", "u":
		return "an"
	default:
		return "a"
	}
}

// Outcome says what Define did to the entry it was pointed at.
type Outcome string

// The three outcomes of one Define: the entry did not exist and now
// does, it existed and a property changed, or nothing changed.
const (
	OutcomeCreated   Outcome = "created"
	OutcomeUpdated   Outcome = "updated"
	OutcomeUnchanged Outcome = "unchanged"
)

// DefineResult reports one Define: the concept the entry was recorded
// as (an aggregate root is its aggregate; a business rule is an
// invariant or an assertion), the owner it sits under when nested, and
// the properties that changed.
type DefineResult struct {
	Outcome Outcome
	Concept Concept
	Owner   string
	Changed []string
}

// RemoveResult reports one Remove: the concept removed, its owner when
// nested, and what else the removal carried away or altered, one
// sentence each.
type RemoveResult struct {
	Concept Concept
	Owner   string
	Also    []string
}

// Define records an entry, creating it when the context does not hold
// it and applying the change when it does. The result is a new
// language; the receiver is untouched. An aggregate root is recorded
// as its aggregate. A business rule is recorded as an assertion when
// the change names the operation it is on, an invariant otherwise. The
// loader-level invariants of the meta-model are applied to the whole
// result, so a change that would break one is refused with the
// invariant named.
func (l UbiquitousLanguage) Define(c Concept, at Locator, ch Change) (UbiquitousLanguage, DefineResult, error) {
	c = resolveDefine(c, ch)
	if err := ch.applicable(c); err != nil {
		return UbiquitousLanguage{}, DefineResult{}, err
	}
	at = at.trimmed()
	if at.Name == "" {
		return UbiquitousLanguage{}, DefineResult{}, fmt.Errorf("%w: %s: a name is required", ErrChangeIncomplete, c)
	}
	if err := at.ownerFits(c); err != nil {
		return UbiquitousLanguage{}, DefineResult{}, err
	}
	next := l.clone()
	var res DefineResult
	var err error
	if c == ConceptBoundedContext {
		res, err = next.defineContext(at, ch)
	} else {
		var i int
		i, err = next.contextIndex(at.Context)
		if err == nil {
			res, err = next.Contexts[i].define(c, at, ch)
		}
	}
	if err != nil {
		return UbiquitousLanguage{}, DefineResult{}, err
	}
	validated, err := NewUbiquitousLanguage(next.Project, next.Description, next.Contexts, next.Relations)
	if err != nil {
		return UbiquitousLanguage{}, DefineResult{}, err
	}
	res.Concept = c
	return validated, res, nil
}

// Remove takes an entry out of the language, with everything recorded
// under it. Removing a bounded context removes the relations that name
// it; removing an aggregate removes its members, invariants, and
// assertions, and an event it raised no longer names it; removing a
// value object that an aggregate or member names as its identity leaves
// the identity implied. A business rule is looked up among invariants,
// then assertions.
func (l UbiquitousLanguage) Remove(c Concept, at Locator) (UbiquitousLanguage, RemoveResult, error) {
	if c == ConceptAggregateRoot {
		c = ConceptAggregate
	}
	at = at.trimmed()
	if at.Name == "" {
		return UbiquitousLanguage{}, RemoveResult{}, fmt.Errorf("%w: %s: a name is required", ErrChangeIncomplete, c)
	}
	if err := at.ownerFits(c); err != nil {
		return UbiquitousLanguage{}, RemoveResult{}, err
	}
	next := l.clone()
	var res RemoveResult
	var err error
	if c == ConceptBoundedContext {
		res, err = next.removeContext(at.Name)
	} else {
		var i int
		i, err = next.contextIndex(at.Context)
		if err == nil {
			res, err = next.Contexts[i].remove(c, at)
		}
	}
	if err != nil {
		return UbiquitousLanguage{}, RemoveResult{}, err
	}
	validated, err := NewUbiquitousLanguage(next.Project, next.Description, next.Contexts, next.Relations)
	if err != nil {
		return UbiquitousLanguage{}, RemoveResult{}, err
	}
	return validated, res, nil
}

// resolveDefine maps the two alias concepts to the entry they are
// recorded as.
func resolveDefine(c Concept, ch Change) Concept {
	switch c {
	case ConceptAggregateRoot:
		return ConceptAggregate
	case ConceptBusinessRule:
		if ch.SetOn {
			return ConceptAssertion
		}
		return ConceptInvariant
	default:
		return c
	}
}

func (at Locator) trimmed() Locator {
	return Locator{
		Context: strings.TrimSpace(at.Context),
		Owner:   strings.TrimSpace(at.Owner),
		Name:    strings.TrimSpace(at.Name),
	}
}

// ownerFits refuses an owner on a concept the context records
// directly, and a context on a bounded context that is not itself.
func (at Locator) ownerFits(c Concept) error {
	switch c {
	case ConceptEntity, ConceptInvariant, ConceptAssertion, ConceptBusinessRule:
		return nil
	case ConceptBoundedContext:
		if at.Context != "" && at.Context != at.Name {
			return fmt.Errorf("%w: context %q is located by its own name, not inside context %q", ErrChangeMisapplied, at.Name, at.Context)
		}
	default:
		// Recorded directly under the context: an owner is a misapplication.
	}
	if at.Owner != "" {
		return fmt.Errorf("%w: %s %q is recorded under the context, not under %q", ErrChangeMisapplied, c, at.Name, at.Owner)
	}
	return nil
}

func (l UbiquitousLanguage) clone() UbiquitousLanguage {
	return UbiquitousLanguage{
		Project:     l.Project,
		Description: l.Description,
		Contexts:    cloneContexts(l.Contexts),
		Relations:   cloneRelations(l.Relations),
	}
}

func (l UbiquitousLanguage) contextIndex(name string) (int, error) {
	if name == "" {
		return -1, fmt.Errorf("%w: name the bounded context the entry belongs to; recorded: %s", ErrChangeIncomplete, strings.Join(l.ContextNames(), ", "))
	}
	i := slices.IndexFunc(l.Contexts, func(c BoundedContext) bool { return c.Name == name })
	if i < 0 {
		return -1, fmt.Errorf("%w: context %q is not recorded; recorded: %s", ErrDefinitionNotFound, name, strings.Join(l.ContextNames(), ", "))
	}
	return i, nil
}

func (l *UbiquitousLanguage) defineContext(at Locator, ch Change) (DefineResult, error) {
	i := slices.IndexFunc(l.Contexts, func(c BoundedContext) bool { return c.Name == at.Name })
	created := i < 0
	if created {
		if err := ch.requiredToRecord(ConceptBoundedContext, "context", at.Name); err != nil {
			return DefineResult{}, err
		}
		l.Contexts = append(l.Contexts, BoundedContext{Name: at.Name})
		i = len(l.Contexts) - 1
	}
	c := &l.Contexts[i]
	var changed []string
	setString(&c.Definition, ch.SetDefinition, ch.Definition, propDefinition, &changed)
	return outcome(created, changed), nil
}

func (l *UbiquitousLanguage) removeContext(name string) (RemoveResult, error) {
	i := slices.IndexFunc(l.Contexts, func(c BoundedContext) bool { return c.Name == name })
	if i < 0 {
		return RemoveResult{}, fmt.Errorf("%w: context %q is not recorded", ErrDefinitionNotFound, name)
	}
	l.Contexts = slices.Delete(l.Contexts, i, i+1)
	res := RemoveResult{Concept: ConceptBoundedContext}
	l.Relations = slices.DeleteFunc(l.Relations, func(r ContextRelation) bool {
		if r.From != name && r.To != name {
			return false
		}
		res.Also = append(res.Also, fmt.Sprintf("relation %s -> %s (%s) removed", r.From, r.To, r.Kind))
		return true
	})
	return res, nil
}

// define records one entry of the context.
func (c *BoundedContext) define(k Concept, at Locator, ch Change) (DefineResult, error) {
	if err := c.nameFree(k, at.Name); err != nil {
		return DefineResult{}, err
	}
	switch k {
	case ConceptAggregate:
		return c.defineAggregate(at.Name, ch)
	case ConceptEntity:
		return c.defineEntity(at, ch)
	case ConceptValueObject:
		return c.defineValueObject(at.Name, ch)
	case ConceptInvariant:
		return c.defineInvariant(at, ch)
	case ConceptAssertion:
		return c.defineAssertion(at, ch)
	case ConceptDomainEvent:
		return c.defineEvent(at.Name, ch)
	case ConceptDomainService:
		return c.defineService(at.Name, ch)
	case ConceptSpecification:
		return c.defineSpecification(at.Name, ch)
	case ConceptQuestion:
		return c.defineQuestion(at.Name, ch)
	default:
		return DefineResult{}, fmt.Errorf("%s has no entry of its own in the domain file", k)
	}
}

// nameFree refuses to record a term under a concept when the context
// already records the name as another concept; the entry is corrected
// by removing it and recording it again, never by silent reclassing.
// Keys of invariants, assertions, and questions are not terms.
func (c BoundedContext) nameFree(k Concept, name string) error {
	if k == ConceptInvariant || k == ConceptAssertion || k == ConceptQuestion {
		return nil
	}
	t, ok := c.Term(name)
	if !ok || t.Implied || t.Concept == k {
		return nil
	}
	return fmt.Errorf("ubiquitous_language/one-meaning-per-name: context %q records %q as %s; remove it to record it as %s",
		c.Name, name, describe(t), k)
}

func describe(t Term) string {
	if t.Concept == ConceptEntity {
		return fmt.Sprintf("an entity of %s", t.Aggregate)
	}
	return article(string(t.Concept)) + " " + string(t.Concept)
}

func (c *BoundedContext) defineAggregate(name string, ch Change) (DefineResult, error) {
	i := slices.IndexFunc(c.Aggregates, func(a Aggregate) bool { return a.Name == name })
	created := i < 0
	if created {
		if err := ch.requiredToRecord(ConceptAggregate, "aggregate", name); err != nil {
			return DefineResult{}, err
		}
		c.Aggregates = append(c.Aggregates, Aggregate{Name: name})
		i = len(c.Aggregates) - 1
	}
	a := &c.Aggregates[i]
	var changed []string
	setString(&a.Definition, ch.SetDefinition, ch.Definition, propDefinition, &changed)
	setString(&a.Identity, ch.SetIdentity, ch.Identity, propIdentity, &changed)
	setStrings(&a.Aliases, ch.SetAliases, ch.Aliases, propAliases, &changed)
	setString(&a.Repository, ch.SetRepository, ch.Repository, propRepository, &changed)
	setString(&a.Factory, ch.SetFactory, ch.Factory, propFactory, &changed)
	return outcome(created, changed), nil
}

func (c *BoundedContext) defineEntity(at Locator, ch Change) (DefineResult, error) {
	ai, ei := c.entityIndex(at.Name)
	created := ai < 0
	if created {
		if at.Owner == "" {
			return DefineResult{}, fmt.Errorf("%w: entity %q is not recorded; recording one names the aggregate it belongs to", ErrChangeIncomplete, at.Name)
		}
		ai = c.aggregateIndex(at.Owner)
		if ai < 0 {
			return DefineResult{}, fmt.Errorf("%w: context %q records no aggregate %q for entity %q to belong to",
				ErrDefinitionNotFound, c.Name, at.Owner, at.Name)
		}
		if err := ch.requiredToRecord(ConceptEntity, "entity", at.Name); err != nil {
			return DefineResult{}, err
		}
		c.Aggregates[ai].Entities = append(c.Aggregates[ai].Entities, Entity{Name: at.Name})
		ei = len(c.Aggregates[ai].Entities) - 1
	} else if at.Owner != "" && at.Owner != c.Aggregates[ai].Name {
		return DefineResult{}, fmt.Errorf("entity %q belongs to aggregate %q; remove it and record it under %q to move it",
			at.Name, c.Aggregates[ai].Name, at.Owner)
	}
	e := &c.Aggregates[ai].Entities[ei]
	var changed []string
	setString(&e.Definition, ch.SetDefinition, ch.Definition, propDefinition, &changed)
	setString(&e.Identity, ch.SetIdentity, ch.Identity, propIdentity, &changed)
	setStrings(&e.Aliases, ch.SetAliases, ch.Aliases, propAliases, &changed)
	res := outcome(created, changed)
	res.Owner = c.Aggregates[ai].Name
	return res, nil
}

func (c *BoundedContext) defineValueObject(name string, ch Change) (DefineResult, error) {
	i := slices.IndexFunc(c.ValueObjects, func(v ValueObject) bool { return v.Name == name })
	created := i < 0
	if created {
		if err := ch.requiredToRecord(ConceptValueObject, "value object", name); err != nil {
			return DefineResult{}, err
		}
		c.ValueObjects = append(c.ValueObjects, ValueObject{Name: name})
		i = len(c.ValueObjects) - 1
	}
	v := &c.ValueObjects[i]
	var changed []string
	setString(&v.Definition, ch.SetDefinition, ch.Definition, propDefinition, &changed)
	setStrings(&v.Aliases, ch.SetAliases, ch.Aliases, propAliases, &changed)
	return outcome(created, changed), nil
}

func (c *BoundedContext) defineInvariant(at Locator, ch Change) (DefineResult, error) {
	owner := at.Owner
	if owner == "" {
		found, ok, err := c.ownerOfInvariant(at.Name)
		if err != nil {
			return DefineResult{}, err
		}
		if !ok {
			return DefineResult{}, fmt.Errorf("%w: invariant %q is not recorded; recording one names the aggregate or value object that owns it", ErrChangeIncomplete, at.Name)
		}
		owner = found
	}
	invs, err := c.invariantsOf(owner)
	if err != nil {
		return DefineResult{}, err
	}
	i := slices.IndexFunc(*invs, func(inv Invariant) bool { return inv.Key == at.Name })
	created := i < 0
	if created {
		if err := ch.requiredToRecord(ConceptInvariant, "invariant", at.Name); err != nil {
			return DefineResult{}, err
		}
		*invs = append(*invs, Invariant{Key: at.Name})
		i = len(*invs) - 1
	}
	inv := &(*invs)[i]
	var changed []string
	setString(&inv.Statement, ch.SetStatement, ch.Statement, propStatement, &changed)
	res := outcome(created, changed)
	res.Owner = owner
	return res, nil
}

func (c *BoundedContext) defineAssertion(at Locator, ch Change) (DefineResult, error) {
	owner := at.Owner
	if owner == "" {
		found, ok, err := c.ownerOfAssertion(at.Name)
		if err != nil {
			return DefineResult{}, err
		}
		if !ok {
			return DefineResult{}, fmt.Errorf("%w: assertion %q is not recorded; recording one names the aggregate that owns it", ErrChangeIncomplete, at.Name)
		}
		owner = found
	}
	ai, err := c.assertionOwnerIndex(owner)
	if err != nil {
		return DefineResult{}, err
	}
	a := &c.Aggregates[ai]
	i := slices.IndexFunc(a.Assertions, func(as Assertion) bool { return as.Key == at.Name })
	created := i < 0
	if created {
		if err := ch.requiredToRecord(ConceptAssertion, "assertion", at.Name); err != nil {
			return DefineResult{}, err
		}
		a.Assertions = append(a.Assertions, Assertion{Key: at.Name})
		i = len(a.Assertions) - 1
	}
	as := &a.Assertions[i]
	var changed []string
	setString(&as.On, ch.SetOn, ch.On, propOn, &changed)
	setString(&as.Statement, ch.SetStatement, ch.Statement, propStatement, &changed)
	res := outcome(created, changed)
	res.Owner = owner
	return res, nil
}

func (c *BoundedContext) defineEvent(name string, ch Change) (DefineResult, error) {
	i := slices.IndexFunc(c.Events, func(e DomainEvent) bool { return e.Name == name })
	created := i < 0
	if created {
		if err := ch.requiredToRecord(ConceptDomainEvent, "event", name); err != nil {
			return DefineResult{}, err
		}
		c.Events = append(c.Events, DomainEvent{Name: name})
		i = len(c.Events) - 1
	}
	e := &c.Events[i]
	var changed []string
	setString(&e.Definition, ch.SetDefinition, ch.Definition, propDefinition, &changed)
	setString(&e.RaisedBy, ch.SetRaisedBy, ch.RaisedBy, propRaisedBy, &changed)
	return outcome(created, changed), nil
}

func (c *BoundedContext) defineService(name string, ch Change) (DefineResult, error) {
	i := slices.IndexFunc(c.Services, func(s DomainService) bool { return s.Name == name })
	created := i < 0
	if created {
		if err := ch.requiredToRecord(ConceptDomainService, "service", name); err != nil {
			return DefineResult{}, err
		}
		c.Services = append(c.Services, DomainService{Name: name})
		i = len(c.Services) - 1
	}
	var changed []string
	setString(&c.Services[i].Definition, ch.SetDefinition, ch.Definition, propDefinition, &changed)
	return outcome(created, changed), nil
}

func (c *BoundedContext) defineSpecification(name string, ch Change) (DefineResult, error) {
	i := slices.IndexFunc(c.Specifications, func(s Specification) bool { return s.Name == name })
	created := i < 0
	if created {
		if err := ch.requiredToRecord(ConceptSpecification, "specification", name); err != nil {
			return DefineResult{}, err
		}
		c.Specifications = append(c.Specifications, Specification{Name: name})
		i = len(c.Specifications) - 1
	}
	var changed []string
	setString(&c.Specifications[i].Definition, ch.SetDefinition, ch.Definition, propDefinition, &changed)
	return outcome(created, changed), nil
}

func (c *BoundedContext) defineQuestion(key string, ch Change) (DefineResult, error) {
	i := slices.IndexFunc(c.Questions, func(q Question) bool { return q.Key == key })
	created := i < 0
	if created {
		if err := ch.requiredToRecord(ConceptQuestion, "question", key); err != nil {
			return DefineResult{}, err
		}
		c.Questions = append(c.Questions, Question{Key: key})
		i = len(c.Questions) - 1
	}
	var changed []string
	setString(&c.Questions[i].Text, ch.SetText, ch.Text, propText, &changed)
	return outcome(created, changed), nil
}

// remove takes one entry out of the context.
func (c *BoundedContext) remove(k Concept, at Locator) (RemoveResult, error) {
	switch k {
	case ConceptAggregate:
		return c.removeAggregate(at.Name)
	case ConceptEntity:
		return c.removeEntity(at)
	case ConceptValueObject:
		return c.removeValueObject(at.Name)
	case ConceptInvariant:
		return c.removeInvariant(at)
	case ConceptAssertion:
		return c.removeAssertion(at)
	case ConceptBusinessRule:
		res, err := c.removeInvariant(at)
		if err == nil {
			return res, nil
		}
		if res, asErr := c.removeAssertion(at); asErr == nil {
			return res, nil
		}
		return RemoveResult{}, err
	case ConceptDomainEvent:
		i := slices.IndexFunc(c.Events, func(e DomainEvent) bool { return e.Name == at.Name })
		if i < 0 {
			return RemoveResult{}, c.notFound("event", at.Name)
		}
		c.Events = slices.Delete(c.Events, i, i+1)
		return RemoveResult{Concept: k}, nil
	case ConceptDomainService:
		i := slices.IndexFunc(c.Services, func(s DomainService) bool { return s.Name == at.Name })
		if i < 0 {
			return RemoveResult{}, c.notFound("service", at.Name)
		}
		c.Services = slices.Delete(c.Services, i, i+1)
		return RemoveResult{Concept: k}, nil
	case ConceptSpecification:
		i := slices.IndexFunc(c.Specifications, func(s Specification) bool { return s.Name == at.Name })
		if i < 0 {
			return RemoveResult{}, c.notFound("specification", at.Name)
		}
		c.Specifications = slices.Delete(c.Specifications, i, i+1)
		return RemoveResult{Concept: k}, nil
	case ConceptQuestion:
		i := slices.IndexFunc(c.Questions, func(q Question) bool { return q.Key == at.Name })
		if i < 0 {
			return RemoveResult{}, c.notFound("question", at.Name)
		}
		c.Questions = slices.Delete(c.Questions, i, i+1)
		return RemoveResult{Concept: k}, nil
	default:
		return RemoveResult{}, fmt.Errorf("%s has no entry of its own in the domain file", k)
	}
}

func (c *BoundedContext) removeAggregate(name string) (RemoveResult, error) {
	i := c.aggregateIndex(name)
	if i < 0 {
		return RemoveResult{}, c.notFound("aggregate", name)
	}
	a := c.Aggregates[i]
	res := RemoveResult{Concept: ConceptAggregate}
	for _, e := range a.Entities {
		res.Also = append(res.Also, fmt.Sprintf("entity %s removed with it", e.Name))
	}
	for _, inv := range a.Invariants {
		res.Also = append(res.Also, fmt.Sprintf("invariant %s removed with it", inv.Key))
	}
	for _, as := range a.Assertions {
		res.Also = append(res.Also, fmt.Sprintf("assertion %s removed with it", as.Key))
	}
	c.Aggregates = slices.Delete(c.Aggregates, i, i+1)
	for j := range c.Events {
		if c.Events[j].RaisedBy == name {
			c.Events[j].RaisedBy = ""
			res.Also = append(res.Also, fmt.Sprintf("event %s no longer names what raises it", c.Events[j].Name))
		}
	}
	return res, nil
}

func (c *BoundedContext) removeEntity(at Locator) (RemoveResult, error) {
	ai, ei := c.entityIndex(at.Name)
	if ai < 0 {
		return RemoveResult{}, c.notFound("entity", at.Name)
	}
	owner := c.Aggregates[ai].Name
	if at.Owner != "" && at.Owner != owner {
		return RemoveResult{}, fmt.Errorf("entity %q belongs to aggregate %q, not %q", at.Name, owner, at.Owner)
	}
	c.Aggregates[ai].Entities = slices.Delete(c.Aggregates[ai].Entities, ei, ei+1)
	return RemoveResult{Concept: ConceptEntity, Owner: owner}, nil
}

func (c *BoundedContext) removeValueObject(name string) (RemoveResult, error) {
	i := slices.IndexFunc(c.ValueObjects, func(v ValueObject) bool { return v.Name == name })
	if i < 0 {
		return RemoveResult{}, c.notFound("value object", name)
	}
	res := RemoveResult{Concept: ConceptValueObject}
	for _, inv := range c.ValueObjects[i].Invariants {
		res.Also = append(res.Also, fmt.Sprintf("invariant %s removed with it", inv.Key))
	}
	c.ValueObjects = slices.Delete(c.ValueObjects, i, i+1)
	for _, a := range c.Aggregates {
		if a.Identity == name {
			res.Also = append(res.Also, fmt.Sprintf("%s stays the identity of %s, implied", name, a.Name))
		}
		for _, e := range a.Entities {
			if e.Identity == name {
				res.Also = append(res.Also, fmt.Sprintf("%s stays the identity of %s, implied", name, e.Name))
			}
		}
	}
	return res, nil
}

func (c *BoundedContext) removeInvariant(at Locator) (RemoveResult, error) {
	owner := at.Owner
	if owner == "" {
		found, ok, err := c.ownerOfInvariant(at.Name)
		if err != nil {
			return RemoveResult{}, err
		}
		if !ok {
			return RemoveResult{}, c.notFound("invariant", at.Name)
		}
		owner = found
	}
	invs, err := c.invariantsOf(owner)
	if err != nil {
		return RemoveResult{}, err
	}
	i := slices.IndexFunc(*invs, func(inv Invariant) bool { return inv.Key == at.Name })
	if i < 0 {
		return RemoveResult{}, fmt.Errorf("%w: %s records no invariant %q", ErrDefinitionNotFound, owner, at.Name)
	}
	*invs = slices.Delete(*invs, i, i+1)
	return RemoveResult{Concept: ConceptInvariant, Owner: owner}, nil
}

func (c *BoundedContext) removeAssertion(at Locator) (RemoveResult, error) {
	owner := at.Owner
	if owner == "" {
		found, ok, err := c.ownerOfAssertion(at.Name)
		if err != nil {
			return RemoveResult{}, err
		}
		if !ok {
			return RemoveResult{}, c.notFound("assertion", at.Name)
		}
		owner = found
	}
	ai, err := c.assertionOwnerIndex(owner)
	if err != nil {
		return RemoveResult{}, err
	}
	a := &c.Aggregates[ai]
	i := slices.IndexFunc(a.Assertions, func(as Assertion) bool { return as.Key == at.Name })
	if i < 0 {
		return RemoveResult{}, fmt.Errorf("%w: %s records no assertion %q", ErrDefinitionNotFound, owner, at.Name)
	}
	a.Assertions = slices.Delete(a.Assertions, i, i+1)
	return RemoveResult{Concept: ConceptAssertion, Owner: owner}, nil
}

func (c BoundedContext) aggregateIndex(name string) int {
	return slices.IndexFunc(c.Aggregates, func(a Aggregate) bool { return a.Name == name })
}

// entityIndex locates a member entity: the index of its aggregate and
// its index within it, or -1, -1.
func (c BoundedContext) entityIndex(name string) (int, int) {
	for ai, a := range c.Aggregates {
		for ei, e := range a.Entities {
			if e.Name == name {
				return ai, ei
			}
		}
	}
	return -1, -1
}

// invariantsOf returns the invariants slice of an owner, which is an
// aggregate or a recorded value object; anything else is refused with
// the meta-model invariant that says so.
func (c *BoundedContext) invariantsOf(owner string) (*[]Invariant, error) {
	if ai := c.aggregateIndex(owner); ai >= 0 {
		return &c.Aggregates[ai].Invariants, nil
	}
	if vi := slices.IndexFunc(c.ValueObjects, func(v ValueObject) bool { return v.Name == owner }); vi >= 0 {
		return &c.ValueObjects[vi].Invariants, nil
	}
	if t, ok := c.Term(owner); ok {
		if t.Implied {
			return nil, fmt.Errorf("invariant/owned-by-aggregate-or-value-object: %q is the identity of %s and has no entry; record it as a value object to give it invariants", owner, t.Aggregate)
		}
		return nil, fmt.Errorf("invariant/owned-by-aggregate-or-value-object: context %q records %q as %s; an invariant is owned by an aggregate or a value object", c.Name, owner, describe(t))
	}
	return nil, fmt.Errorf("%w: context %q records no aggregate or value object %q", ErrDefinitionNotFound, c.Name, owner)
}

// assertionOwnerIndex returns the index of the aggregate an assertion
// is owned by; anything else is refused with the meta-model invariant
// that says so.
func (c BoundedContext) assertionOwnerIndex(owner string) (int, error) {
	if ai := c.aggregateIndex(owner); ai >= 0 {
		return ai, nil
	}
	if t, ok := c.Term(owner); ok {
		return -1, fmt.Errorf("assertion/owned-by-an-aggregate: context %q records %q as %s; an assertion is owned by an aggregate", c.Name, owner, describe(t))
	}
	return -1, fmt.Errorf("%w: context %q records no aggregate %q", ErrDefinitionNotFound, c.Name, owner)
}

// ownerOfInvariant finds the owner an invariant key is recorded under
// when the locator names none: the one owner holding it, or false when
// none does. Two owners holding the key leave the request incomplete.
func (c BoundedContext) ownerOfInvariant(key string) (string, bool, error) {
	var owners []string
	for _, oi := range c.Invariants() {
		if oi.Invariant.Key == key {
			owners = append(owners, oi.Owner)
		}
	}
	return oneOwner(c.Name, "invariant", key, owners)
}

// ownerOfAssertion finds the aggregate an assertion key is recorded
// under when the locator names none.
func (c BoundedContext) ownerOfAssertion(key string) (string, bool, error) {
	var owners []string
	for _, oa := range c.Assertions() {
		if oa.Assertion.Key == key {
			owners = append(owners, oa.Owner)
		}
	}
	return oneOwner(c.Name, "assertion", key, owners)
}

func oneOwner(context, what, key string, owners []string) (string, bool, error) {
	switch len(owners) {
	case 0:
		return "", false, nil
	case 1:
		return owners[0], true, nil
	default:
		return "", false, fmt.Errorf("%w: context %q records %s %q under %s; name the owner", ErrChangeIncomplete, context, what, key, strings.Join(owners, " and "))
	}
}

func (c BoundedContext) notFound(what, name string) error {
	return fmt.Errorf("%w: context %q records no %s %q", ErrDefinitionNotFound, c.Name, what, name)
}

func outcome(created bool, changed []string) DefineResult {
	switch {
	case created:
		return DefineResult{Outcome: OutcomeCreated, Changed: changed}
	case len(changed) > 0:
		return DefineResult{Outcome: OutcomeUpdated, Changed: changed}
	default:
		return DefineResult{Outcome: OutcomeUnchanged}
	}
}

func setString(dst *string, set bool, value, prop string, changed *[]string) {
	if !set {
		return
	}
	value = strings.TrimSpace(value)
	if *dst == value {
		return
	}
	*dst = value
	*changed = append(*changed, prop)
}

func setStrings(dst *[]string, set bool, values []string, prop string, changed *[]string) {
	if !set {
		return
	}
	trimmed := make([]string, 0, len(values))
	for _, v := range values {
		trimmed = append(trimmed, strings.TrimSpace(v))
	}
	if len(trimmed) == 0 {
		trimmed = nil
	}
	if slices.Equal(*dst, trimmed) {
		return
	}
	*dst = trimmed
	*changed = append(*changed, prop)
}
