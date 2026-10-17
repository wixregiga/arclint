package application

import (
	"errors"
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// resolveContext picks the bounded context a domain command records
// in. An explicit name wins and must be recorded. With none named, the
// one recorded context is the default; none or several recorded is a
// usage error that says what to do.
func resolveContext(lang vocab.UbiquitousLanguage, context string) (string, error) {
	context = strings.TrimSpace(context)
	if context != "" {
		if _, ok := lang.Context(context); !ok {
			if len(lang.Contexts) == 0 {
				return "", fmt.Errorf("%w: context %q is not recorded; record it first with: domain define bounded_context %s --definition <text>", ErrDomainUsage, context, context)
			}
			return "", fmt.Errorf("%w: context %q is not recorded; recorded: %s", ErrDomainUsage, context, strings.Join(lang.ContextNames(), ", "))
		}
		return context, nil
	}
	switch len(lang.Contexts) {
	case 1:
		return lang.Contexts[0].Name, nil
	case 0:
		return "", fmt.Errorf("%w: no bounded context is recorded yet; record one first with: domain define bounded_context <name> --definition <text>", ErrDomainUsage)
	default:
		return "", fmt.Errorf("%w: --context is required when the project records several bounded contexts (%s)", ErrDomainUsage, strings.Join(lang.ContextNames(), ", "))
	}
}

// locateEntry finds the recorded entry show and remove name. An
// explicit context is searched alone. With none named and exactly one
// context recorded, that one is searched. With several recorded, every
// context is searched; a unique hit wins, and hits in more than one
// context are a usage error asking for --context. A missing entry is
// reported against the context searched, or against the model when
// none was named.
func locateEntry(lang vocab.UbiquitousLanguage, c vocab.Concept, context, owner, name string) (DomainEntryView, error) {
	context = strings.TrimSpace(context)
	owner = strings.TrimSpace(owner)
	name = strings.TrimSpace(name)
	if name == "" {
		return DomainEntryView{}, fmt.Errorf("%w: %s: a name is required", ErrDomainUsage, c)
	}
	if c == vocab.ConceptBoundedContext {
		ctx, ok := lang.Context(name)
		if !ok {
			return DomainEntryView{}, definitionNotFound(c, "", name)
		}
		return contextView(lang, ctx), nil
	}
	if context != "" {
		ctx, ok := lang.Context(context)
		if !ok {
			return DomainEntryView{}, definitionNotFound(c, context, name)
		}
		view, found, err := entryOf(ctx, c, owner, name)
		if err != nil {
			return DomainEntryView{}, err
		}
		if !found {
			return DomainEntryView{}, definitionNotFound(c, context, name)
		}
		return view, nil
	}
	var hits []DomainEntryView
	for _, ctx := range lang.Contexts {
		view, found, err := entryOf(ctx, c, owner, name)
		if err != nil {
			return DomainEntryView{}, err
		}
		if found {
			hits = append(hits, view)
		}
	}
	switch len(hits) {
	case 0:
		return DomainEntryView{}, definitionNotFound(c, "", name)
	case 1:
		return hits[0], nil
	default:
		names := make([]string, 0, len(hits))
		for _, h := range hits {
			names = append(names, h.Context)
		}
		return DomainEntryView{}, fmt.Errorf("%w: %s %q is recorded in multiple contexts (%s); pass --context",
			ErrDomainUsage, c, name, strings.Join(names, ", "))
	}
}

// entryOf finds one entry of a context by concept and name, honouring
// an owner when one is named. An aggregate root is its aggregate; a
// business rule is an invariant, then an assertion. An invariant or
// assertion key recorded under two owners needs the owner named.
func entryOf(ctx vocab.BoundedContext, c vocab.Concept, owner, name string) (DomainEntryView, bool, error) {
	switch c {
	case vocab.ConceptEntity, vocab.ConceptInvariant, vocab.ConceptAssertion, vocab.ConceptBusinessRule:
	default:
		if owner != "" {
			return DomainEntryView{}, false, fmt.Errorf("%w: %s %q is recorded under the context, not under %q", ErrDomainUsage, c, name, owner)
		}
	}
	view := DomainEntryView{Concept: c, Context: ctx.Name, Name: name}
	switch c {
	case vocab.ConceptAggregate, vocab.ConceptAggregateRoot:
		a, ok := ctx.Aggregate(name)
		view.Concept, view.Aggregate = vocab.ConceptAggregate, a
		return view, ok, nil
	case vocab.ConceptEntity:
		e, a, ok := ctx.Entity(name)
		if ok && owner != "" && owner != a.Name {
			return DomainEntryView{}, false, nil
		}
		view.Entity, view.Owner = e, a.Name
		return view, ok, nil
	case vocab.ConceptValueObject:
		v, ok := ctx.ValueObject(name)
		view.ValueObject = v
		return view, ok, nil
	case vocab.ConceptInvariant:
		return invariantEntry(ctx, owner, name)
	case vocab.ConceptAssertion:
		return assertionEntry(ctx, owner, name)
	case vocab.ConceptBusinessRule:
		if view, ok, err := invariantEntry(ctx, owner, name); ok || err != nil {
			return view, ok, err
		}
		return assertionEntry(ctx, owner, name)
	case vocab.ConceptDomainEvent:
		e, ok := ctx.Event(name)
		view.Event = e
		return view, ok, nil
	case vocab.ConceptDomainService:
		s, ok := ctx.Service(name)
		view.Service = s
		return view, ok, nil
	case vocab.ConceptSpecification:
		s, ok := ctx.Specification(name)
		view.Specification = s
		return view, ok, nil
	case vocab.ConceptQuestion:
		q, ok := ctx.Question(name)
		view.Question = q
		return view, ok, nil
	default:
		return DomainEntryView{}, false, fmt.Errorf("%w: %s has no entry of its own in the domain file", ErrDomainUsage, c)
	}
}

func invariantEntry(ctx vocab.BoundedContext, owner, key string) (DomainEntryView, bool, error) {
	var hits []vocab.OwnedInvariant
	for _, oi := range ctx.Invariants() {
		if oi.Invariant.Key == key && (owner == "" || oi.Owner == owner) {
			hits = append(hits, oi)
		}
	}
	switch len(hits) {
	case 0:
		return DomainEntryView{}, false, nil
	case 1:
		return DomainEntryView{
			Concept:   vocab.ConceptInvariant,
			Context:   ctx.Name,
			Owner:     hits[0].Owner,
			Name:      key,
			Invariant: hits[0].Invariant,
		}, true, nil
	default:
		owners := make([]string, 0, len(hits))
		for _, h := range hits {
			owners = append(owners, h.Owner)
		}
		return DomainEntryView{}, false, fmt.Errorf("%w: context %q records invariant %q under %s; pass --owner",
			ErrDomainUsage, ctx.Name, key, strings.Join(owners, " and "))
	}
}

func assertionEntry(ctx vocab.BoundedContext, owner, key string) (DomainEntryView, bool, error) {
	var hits []vocab.OwnedAssertion
	for _, oa := range ctx.Assertions() {
		if oa.Assertion.Key == key && (owner == "" || oa.Owner == owner) {
			hits = append(hits, oa)
		}
	}
	switch len(hits) {
	case 0:
		return DomainEntryView{}, false, nil
	case 1:
		return DomainEntryView{
			Concept:   vocab.ConceptAssertion,
			Context:   ctx.Name,
			Owner:     hits[0].Owner,
			Name:      key,
			Assertion: hits[0].Assertion,
		}, true, nil
	default:
		owners := make([]string, 0, len(hits))
		for _, h := range hits {
			owners = append(owners, h.Owner)
		}
		return DomainEntryView{}, false, fmt.Errorf("%w: context %q records assertion %q under %s; pass --owner",
			ErrDomainUsage, ctx.Name, key, strings.Join(owners, " and "))
	}
}

// contextView is the show view of a bounded context, with the
// relations of the context map that name it.
func contextView(lang vocab.UbiquitousLanguage, ctx vocab.BoundedContext) DomainEntryView {
	view := DomainEntryView{
		Concept:        vocab.ConceptBoundedContext,
		Context:        ctx.Name,
		Name:           ctx.Name,
		BoundedContext: ctx,
	}
	for _, r := range lang.Relations {
		if r.From == ctx.Name || r.To == ctx.Name {
			view.Relations = append(view.Relations, r)
		}
	}
	return view
}

func definitionNotFound(c vocab.Concept, context, name string) error {
	if context == "" {
		return fmt.Errorf("no %s named %q is recorded in the project domain model: %w", c, name, vocab.ErrDefinitionNotFound)
	}
	return fmt.Errorf("no %s named %q is recorded in context %q of the project domain model: %w", c, name, context, vocab.ErrDefinitionNotFound)
}

// domainRequestError marks a refusal of the request itself, one the
// language reports as incomplete or misapplied, as usage. The
// language's own refusals (a meta-model invariant that would break)
// and missing entries pass through unchanged.
func domainRequestError(err error) error {
	if errors.Is(err, vocab.ErrChangeIncomplete) || errors.Is(err, vocab.ErrChangeMisapplied) {
		return fmt.Errorf("%w: %w", ErrDomainUsage, err)
	}
	return err
}

// entryCandidate is one recorded entry a bare name may mean.
type entryCandidate struct {
	concept vocab.Concept
	context string
	owner   string
	name    string
}

// command is the show command naming the candidate alone: its concept
// and canonical name, with --owner for an entry nested under one and
// --context when the project records several contexts.
func (c entryCandidate) command(contexts int) string {
	cmd := fmt.Sprintf("arclint domain show %s %s", c.concept, c.name)
	if c.owner != "" {
		cmd += " --owner " + c.owner
	}
	if contexts > 1 && c.concept != vocab.ConceptBoundedContext {
		cmd += " --context " + c.context
	}
	return cmd
}

// locateNamedEntry finds the entry a name, key, or alias means without
// its concept being named. Exact spellings win; with none, a spelling
// equal but for case is accepted. One candidate is the entry; several
// are a usage error listing the command that shows each one.
func locateNamedEntry(lang vocab.UbiquitousLanguage, context, owner, name string) (DomainEntryView, error) {
	context = strings.TrimSpace(context)
	owner = strings.TrimSpace(owner)
	if context != "" {
		if _, ok := lang.Context(context); !ok {
			return DomainEntryView{}, fmt.Errorf("no entry named %q is recorded in context %q of the project domain model: %w",
				name, context, vocab.ErrDefinitionNotFound)
		}
	}
	all := entryCandidates(lang, context, owner)
	hits := matchCandidates(all, func(s string) bool { return s == name })
	if len(hits) == 0 {
		hits = matchCandidates(all, func(s string) bool { return strings.EqualFold(s, name) })
	}
	switch len(hits) {
	case 0:
		if c, err := vocab.ParseConcept(name); err == nil {
			return DomainEntryView{}, fmt.Errorf("%w: %q is a concept, not a recorded entry; name one: arclint domain show %s <name>",
				ErrDomainUsage, name, c)
		}
		return DomainEntryView{}, fmt.Errorf("no entry named %q is recorded in the project domain model; arclint domain list names every entry: %w",
			name, vocab.ErrDefinitionNotFound)
	case 1:
		h := hits[0]
		return locateEntry(lang, h.concept, h.context, h.owner, h.name)
	default:
		lines := make([]string, 0, len(hits))
		for _, h := range hits {
			lines = append(lines, "  "+h.command(len(lang.Contexts)))
		}
		return DomainEntryView{}, fmt.Errorf("%w: %q names %d recorded entries; show one of them:\n%s",
			ErrDomainUsage, name, len(hits), strings.Join(lines, "\n"))
	}
}

// matchedCandidate pairs a candidate with every spelling that finds
// it: its name or key, and its aliases.
type matchedCandidate struct {
	entryCandidate
	spellings []string
}

func matchCandidates(all []matchedCandidate, matches func(string) bool) []entryCandidate {
	var out []entryCandidate
	for _, m := range all {
		for _, s := range m.spellings {
			if matches(s) {
				out = append(out, m.entryCandidate)
				break
			}
		}
	}
	return out
}

// entryCandidates lists every entry a show can name, in recording
// order, narrowed to a context and an owner when they are given.
func entryCandidates(lang vocab.UbiquitousLanguage, context, owner string) []matchedCandidate {
	var out []matchedCandidate
	add := func(c vocab.Concept, ctx, entryOwner, name string, aliases ...string) {
		if owner != "" && entryOwner != owner {
			return
		}
		out = append(out, matchedCandidate{
			entryCandidate: entryCandidate{concept: c, context: ctx, owner: entryOwner, name: name},
			spellings:      append([]string{name}, aliases...),
		})
	}
	for _, ctx := range lang.Contexts {
		if context != "" && ctx.Name != context {
			continue
		}
		add(vocab.ConceptBoundedContext, ctx.Name, "", ctx.Name)
		for _, a := range ctx.Aggregates {
			add(vocab.ConceptAggregate, ctx.Name, "", a.Name, a.Aliases...)
			for _, e := range a.Entities {
				add(vocab.ConceptEntity, ctx.Name, a.Name, e.Name, e.Aliases...)
			}
		}
		for _, v := range ctx.ValueObjects {
			add(vocab.ConceptValueObject, ctx.Name, "", v.Name, v.Aliases...)
		}
		for _, oi := range ctx.Invariants() {
			add(vocab.ConceptInvariant, ctx.Name, oi.Owner, oi.Invariant.Key)
		}
		for _, oa := range ctx.Assertions() {
			add(vocab.ConceptAssertion, ctx.Name, oa.Owner, oa.Assertion.Key)
		}
		for _, e := range ctx.Events {
			add(vocab.ConceptDomainEvent, ctx.Name, "", e.Name)
		}
		for _, s := range ctx.Services {
			add(vocab.ConceptDomainService, ctx.Name, "", s.Name)
		}
		for _, s := range ctx.Specifications {
			add(vocab.ConceptSpecification, ctx.Name, "", s.Name)
		}
		for _, q := range ctx.Questions {
			add(vocab.ConceptQuestion, ctx.Name, "", q.Key)
		}
	}
	return out
}
