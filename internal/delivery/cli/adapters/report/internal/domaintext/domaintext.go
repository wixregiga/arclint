// Package domaintext renders the domain reports for the text renderers.
// The plain renderer and the Lipgloss renderer print the same words in
// the same order; only the Style differs, so the wording lives here
// once and each adapter lends its styling.
package domaintext

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// Style is what a renderer contributes: how to spell a heading, a
// secondary line, a success, a path, and a warning. Plain renderers
// leave every function the identity.
type Style struct {
	Bold  func(string) string
	Muted func(string) string
	OK    func(string) string
	Path  func(string) string
	Warn  func(string) string
}

// Plain is the Style that changes nothing.
func Plain() Style {
	id := func(s string) string { return s }
	return Style{Bold: id, Muted: id, OK: id, Path: id, Warn: id}
}

// Init reports domain init: the file created for the project, or the
// file that was already there.
func Init(p *out.Printer, st Style, r application.InitDomainResult) {
	if r.Created {
		p.Printf("%s %s for project %s.\n", st.OK("Initialized"), r.Source, st.Bold(r.Project))
		return
	}
	p.Printf("%s already exists (project %s); left unchanged.\n", r.Source, r.Project)
}

// Missing is the guidance printed when no domain file is recorded.
func Missing(p *out.Printer, st Style) {
	p.Println(st.Bold("No recorded Ubiquitous Language found at " + vocab.UbiquitousLanguageFileName + "."))
	p.Println()
	p.Println("Initialize an empty model:")
	p.Println("  arclint domain init")
	p.Println()
	p.Println("Record the first bounded context:")
	p.Println("  arclint domain define bounded_context <name> --definition <text>")
	p.Println()
	p.Println("Start guided authoring:")
	p.Println("  arclint domain define --guided")
}

// Overview prints the whole recorded model, context by context, with
// each contract's source when the overview located them.
func Overview(p *out.Printer, st Style, o application.DomainOverview) {
	lang := o.Language
	p.Printf("%s %s\n", st.Bold("Project domain:"), lang.Project)
	p.Printf("%s %s\n", st.Bold("Source:"), o.Source)
	if lang.Description != "" {
		p.Println(lang.Description)
	}
	p.Println()
	p.Println(CountsLine(o.Counts))
	for _, ctx := range lang.Contexts {
		p.Println()
		p.Printf("%s %s\n", st.Bold("Context"), st.Bold(ctx.Name))
		if ctx.Definition != "" {
			p.Printf("  %s\n", ctx.Definition)
		}
		for _, a := range ctx.Aggregates {
			p.Println()
			p.Printf("  %s %s\n", st.Bold("Aggregate"), st.Bold(a.Name))
			p.Printf("    %s %s\n", st.Muted("identity:"), a.Identity)
			if a.Definition != "" {
				p.Printf("    %s\n", a.Definition)
			}
			if len(a.Aliases) > 0 {
				p.Printf("    %s %s\n", st.Muted("aliases:"), strings.Join(a.Aliases, ", "))
			}
			if a.Repository != "" {
				p.Printf("    %s %s\n", st.Muted("repository:"), a.Repository)
			}
			if a.Factory != "" {
				p.Printf("    %s %s\n", st.Muted("factory:"), a.Factory)
			}
			if len(a.Entities) > 0 {
				p.Printf("    %s\n", st.Muted("entities:"))
				for _, e := range a.Entities {
					p.Printf("      %s\n", namedLine(e.Name, e.Definition))
					if e.Identity != "" {
						p.Printf("        %s %s\n", st.Muted("identity:"), e.Identity)
					}
					if len(e.Aliases) > 0 {
						p.Printf("        %s %s\n", st.Muted("aliases:"), strings.Join(e.Aliases, ", "))
					}
				}
			}
			if len(a.Invariants) > 0 {
				p.Printf("    %s\n", st.Muted("invariants:"))
				for _, inv := range a.Invariants {
					p.Printf("      %s\n", namedLine(inv.Key, inv.Statement))
					writeSourceLine(p, st, "        ", contractSource(o.Matrix, ctx.Name, application.ContractInvariant, a.Name, inv.Key))
				}
			}
			if len(a.Assertions) > 0 {
				p.Printf("    %s\n", st.Muted("assertions:"))
				for _, as := range a.Assertions {
					p.Printf("      %s\n", namedLine(as.Key+" (on "+as.On+")", as.Statement))
					writeSourceLine(p, st, "        ", contractSource(o.Matrix, ctx.Name, application.ContractAssertion, a.Name, as.Key))
				}
			}
		}
		if len(ctx.ValueObjects) > 0 {
			p.Println()
			p.Printf("  %s\n", st.Bold("Value objects"))
			for _, v := range ctx.ValueObjects {
				p.Printf("    %s\n", namedLine(v.Name, v.Definition))
				if len(v.Aliases) > 0 {
					p.Printf("      %s %s\n", st.Muted("aliases:"), strings.Join(v.Aliases, ", "))
				}
				for _, inv := range v.Invariants {
					p.Printf("      %s %s\n", st.Muted("invariant"), namedLine(inv.Key, inv.Statement))
					writeSourceLine(p, st, "        ", contractSource(o.Matrix, ctx.Name, application.ContractInvariant, v.Name, inv.Key))
				}
			}
		}
		if len(ctx.Events) > 0 {
			p.Println()
			p.Printf("  %s\n", st.Bold("Domain events"))
			for _, e := range ctx.Events {
				name := e.Name
				if e.RaisedBy != "" {
					name += " (raised by " + e.RaisedBy + ")"
				}
				p.Printf("    %s\n", namedLine(name, e.Definition))
			}
		}
		if len(ctx.Services) > 0 {
			p.Println()
			p.Printf("  %s\n", st.Bold("Domain services"))
			for _, s := range ctx.Services {
				p.Printf("    %s\n", namedLine(s.Name, s.Definition))
			}
		}
		if len(ctx.Specifications) > 0 {
			p.Println()
			p.Printf("  %s\n", st.Bold("Specifications"))
			for _, s := range ctx.Specifications {
				p.Printf("    %s\n", namedLine(s.Name, s.Definition))
				writeSourceLine(p, st, "      ", contractSource(o.Matrix, ctx.Name, application.ContractSpecification, "", s.Name))
			}
		}
		if len(ctx.Questions) > 0 {
			p.Println()
			p.Printf("  %s\n", st.Bold("Questions"))
			for _, q := range ctx.Questions {
				p.Printf("    %s\n", namedLine(q.Key, q.Text))
			}
		}
	}
	if len(lang.Relations) > 0 {
		p.Println()
		p.Println(st.Bold("Relations"))
		for _, rel := range lang.Relations {
			p.Printf("  %s\n", relationLine(rel))
		}
	}
}

// CountsLine spells the tallies of a model: the contexts always, then
// every kind of entry the model records at least one of.
func CountsLine(c vocab.Counts) string {
	parts := []string{countPhrase(c.Contexts, "context", "contexts")}
	for _, t := range []struct {
		n                int
		singular, plural string
	}{
		{c.Aggregates, "aggregate", "aggregates"},
		{c.Entities, "entity", "entities"},
		{c.ValueObjects, "value object", "value objects"},
		{c.Invariants, "invariant", "invariants"},
		{c.Assertions, "assertion", "assertions"},
		{c.Specifications, "specification", "specifications"},
		{c.Events, "event", "events"},
		{c.Services, "service", "services"},
		{c.Questions, "question", "questions"},
		{c.Relations, "relation", "relations"},
	} {
		if t.n > 0 {
			parts = append(parts, countPhrase(t.n, t.singular, t.plural))
		}
	}
	return strings.Join(parts, " · ")
}

// List prints names only: every group of every selected context, or
// the one group the listing filters to.
func List(p *out.Printer, st Style, l application.DomainListing) {
	p.Printf("%s %s\n", st.Bold("Project domain:"), l.Language.Project)
	if l.Filtered && l.Concept == vocab.ConceptBoundedContext {
		p.Println()
		p.Println(st.Bold("Bounded contexts"))
		for _, ctx := range selectedContexts(l) {
			p.Printf("  %s\n", namedLine(ctx.Name, ctx.Definition))
		}
		return
	}
	for _, ctx := range selectedContexts(l) {
		p.Println()
		p.Printf("%s %s\n", st.Bold("Context"), st.Bold(ctx.Name))
		if !l.Filtered {
			listAggregates(p, st, ctx)
			listValueObjects(p, st, ctx)
			listInvariants(p, st, ctx)
			listAssertions(p, st, ctx)
			listSpecifications(p, st, ctx)
			listEvents(p, st, ctx)
			listServices(p, st, ctx)
			listQuestions(p, st, ctx)
			continue
		}
		switch l.Concept {
		case vocab.ConceptAggregate, vocab.ConceptAggregateRoot:
			listAggregates(p, st, ctx)
		case vocab.ConceptEntity:
			listEntities(p, st, ctx)
		case vocab.ConceptValueObject:
			listValueObjects(p, st, ctx)
		case vocab.ConceptInvariant:
			listInvariants(p, st, ctx)
		case vocab.ConceptAssertion:
			listAssertions(p, st, ctx)
		case vocab.ConceptBusinessRule:
			listInvariants(p, st, ctx)
			listAssertions(p, st, ctx)
		case vocab.ConceptSpecification:
			listSpecifications(p, st, ctx)
		case vocab.ConceptDomainEvent:
			listEvents(p, st, ctx)
		case vocab.ConceptDomainService:
			listServices(p, st, ctx)
		case vocab.ConceptQuestion:
			listQuestions(p, st, ctx)
		case vocab.ConceptBoundedContext:
			// Listed above as the contexts themselves, never per context.
		}
	}
	if l.Context == "" && len(l.Language.Relations) > 0 {
		p.Println()
		p.Println(st.Bold("Relations"))
		for _, rel := range l.Language.Relations {
			p.Printf("  %s\n", relationLine(rel))
		}
	}
}

func selectedContexts(l application.DomainListing) []vocab.BoundedContext {
	if l.Context == "" {
		return l.Language.Contexts
	}
	ctx, ok := l.Language.Context(l.Context)
	if !ok {
		return nil
	}
	return []vocab.BoundedContext{ctx}
}

func listAggregates(p *out.Printer, st Style, ctx vocab.BoundedContext) {
	if len(ctx.Aggregates) == 0 {
		return
	}
	p.Printf("  %s\n", st.Muted("Aggregates"))
	for _, a := range ctx.Aggregates {
		p.Printf("    %s (%s)\n", a.Name, a.Identity)
		for _, e := range a.Entities {
			p.Printf("      %s\n", e.Name)
		}
	}
}

func listEntities(p *out.Printer, st Style, ctx vocab.BoundedContext) {
	var lines []string
	for _, a := range ctx.Aggregates {
		for _, e := range a.Entities {
			lines = append(lines, fmt.Sprintf("    %s (%s)", e.Name, a.Name))
		}
	}
	if len(lines) == 0 {
		return
	}
	p.Printf("  %s\n", st.Muted("Entities"))
	for _, line := range lines {
		p.Println(line)
	}
}

func listValueObjects(p *out.Printer, st Style, ctx vocab.BoundedContext) {
	if len(ctx.ValueObjects) == 0 {
		return
	}
	p.Printf("  %s\n", st.Muted("Value objects"))
	for _, v := range ctx.ValueObjects {
		p.Printf("    %s\n", v.Name)
	}
}

func listInvariants(p *out.Printer, st Style, ctx vocab.BoundedContext) {
	owned := ctx.Invariants()
	if len(owned) == 0 {
		return
	}
	p.Printf("  %s\n", st.Muted("Invariants"))
	for _, oi := range owned {
		p.Printf("    %s (%s)\n", oi.Invariant.Key, oi.Owner)
	}
}

func listAssertions(p *out.Printer, st Style, ctx vocab.BoundedContext) {
	owned := ctx.Assertions()
	if len(owned) == 0 {
		return
	}
	p.Printf("  %s\n", st.Muted("Assertions"))
	for _, oa := range owned {
		p.Printf("    %s (%s, on %s)\n", oa.Assertion.Key, oa.Owner, oa.Assertion.On)
	}
}

func listSpecifications(p *out.Printer, st Style, ctx vocab.BoundedContext) {
	if len(ctx.Specifications) == 0 {
		return
	}
	p.Printf("  %s\n", st.Muted("Specifications"))
	for _, s := range ctx.Specifications {
		p.Printf("    %s\n", s.Name)
	}
}

func listEvents(p *out.Printer, st Style, ctx vocab.BoundedContext) {
	if len(ctx.Events) == 0 {
		return
	}
	p.Printf("  %s\n", st.Muted("Domain events"))
	for _, e := range ctx.Events {
		if e.RaisedBy != "" {
			p.Printf("    %s (raised by %s)\n", e.Name, e.RaisedBy)
			continue
		}
		p.Printf("    %s\n", e.Name)
	}
}

func listServices(p *out.Printer, st Style, ctx vocab.BoundedContext) {
	if len(ctx.Services) == 0 {
		return
	}
	p.Printf("  %s\n", st.Muted("Domain services"))
	for _, s := range ctx.Services {
		p.Printf("    %s\n", s.Name)
	}
}

func listQuestions(p *out.Printer, st Style, ctx vocab.BoundedContext) {
	if len(ctx.Questions) == 0 {
		return
	}
	p.Printf("  %s\n", st.Muted("Questions"))
	for _, q := range ctx.Questions {
		p.Printf("    %s\n", q.Key)
	}
}

// Show prints one entry with every property it records.
func Show(p *out.Printer, st Style, v application.DomainEntryView) {
	title := v.Concept.Doc().Title
	p.Printf("%s %s\n", st.Bold(title+":"), v.Name)
	if v.Concept != vocab.ConceptBoundedContext {
		p.Printf("%s %s\n", st.Bold("Context:"), v.Context)
	}
	switch v.Concept {
	case vocab.ConceptBoundedContext:
		showContext(p, st, v)
	case vocab.ConceptAggregate:
		a := v.Aggregate
		p.Printf("%s %s\n", st.Bold("Identity:"), a.Identity)
		showDefinition(p, st, a.Definition)
		showAliases(p, st, a.Aliases)
		if a.Repository != "" {
			p.Printf("%s %s\n", st.Bold("Repository:"), a.Repository)
		}
		if a.Factory != "" {
			p.Printf("%s %s\n", st.Bold("Factory:"), a.Factory)
		}
		if len(a.Entities) > 0 {
			p.Println(st.Bold("Entities:"))
			for _, e := range a.Entities {
				p.Printf("  %s\n", namedLine(e.Name, e.Definition))
			}
		}
		if len(a.Invariants) > 0 {
			p.Println(st.Bold("Invariants:"))
			for _, inv := range a.Invariants {
				p.Printf("  %s\n", namedLine(inv.Key, inv.Statement))
			}
		}
		if len(a.Assertions) > 0 {
			p.Println(st.Bold("Assertions:"))
			for _, as := range a.Assertions {
				p.Printf("  %s\n", namedLine(as.Key+" (on "+as.On+")", as.Statement))
			}
		}
	case vocab.ConceptEntity:
		p.Printf("%s %s\n", st.Bold("Aggregate:"), v.Owner)
		if v.Entity.Identity != "" {
			p.Printf("%s %s\n", st.Bold("Identity:"), v.Entity.Identity)
		}
		showDefinition(p, st, v.Entity.Definition)
		showAliases(p, st, v.Entity.Aliases)
	case vocab.ConceptValueObject:
		showDefinition(p, st, v.ValueObject.Definition)
		showAliases(p, st, v.ValueObject.Aliases)
		if len(v.ValueObject.Invariants) > 0 {
			p.Println(st.Bold("Invariants:"))
			for _, inv := range v.ValueObject.Invariants {
				p.Printf("  %s\n", namedLine(inv.Key, inv.Statement))
			}
		}
	case vocab.ConceptInvariant:
		p.Printf("%s %s\n", st.Bold("Owner:"), v.Owner)
		p.Printf("%s %s\n", st.Bold("Statement:"), v.Invariant.Statement)
	case vocab.ConceptAssertion:
		p.Printf("%s %s\n", st.Bold("Aggregate:"), v.Owner)
		p.Printf("%s %s\n", st.Bold("On:"), v.Assertion.On)
		p.Printf("%s %s\n", st.Bold("Statement:"), v.Assertion.Statement)
	case vocab.ConceptDomainEvent:
		if v.Event.RaisedBy != "" {
			p.Printf("%s %s\n", st.Bold("Raised by:"), v.Event.RaisedBy)
		}
		showDefinition(p, st, v.Event.Definition)
	case vocab.ConceptDomainService:
		showDefinition(p, st, v.Service.Definition)
	case vocab.ConceptSpecification:
		showDefinition(p, st, v.Specification.Definition)
	case vocab.ConceptQuestion:
		p.Printf("%s %s\n", st.Bold("Text:"), v.Question.Text)
	case vocab.ConceptAggregateRoot, vocab.ConceptBusinessRule:
		// Resolved to the aggregate, invariant, or assertion they name
		// before a view exists; no view carries them.
	}
}

func showContext(p *out.Printer, st Style, v application.DomainEntryView) {
	ctx := v.BoundedContext
	showDefinition(p, st, ctx.Definition)
	showNames(p, st, "Aggregates:", func(yield func(string)) {
		for _, a := range ctx.Aggregates {
			yield(a.Name)
		}
	})
	showNames(p, st, "Value objects:", func(yield func(string)) {
		for _, x := range ctx.ValueObjects {
			yield(x.Name)
		}
	})
	showNames(p, st, "Domain events:", func(yield func(string)) {
		for _, x := range ctx.Events {
			yield(x.Name)
		}
	})
	showNames(p, st, "Domain services:", func(yield func(string)) {
		for _, x := range ctx.Services {
			yield(x.Name)
		}
	})
	showNames(p, st, "Specifications:", func(yield func(string)) {
		for _, x := range ctx.Specifications {
			yield(x.Name)
		}
	})
	showNames(p, st, "Questions:", func(yield func(string)) {
		for _, x := range ctx.Questions {
			yield(x.Key)
		}
	})
	if len(v.Relations) > 0 {
		p.Println(st.Bold("Relations:"))
		for _, rel := range v.Relations {
			p.Printf("  %s\n", relationLine(rel))
		}
	}
}

func showNames(p *out.Printer, st Style, label string, each func(func(string))) {
	var names []string
	each(func(n string) { names = append(names, n) })
	if len(names) == 0 {
		return
	}
	p.Printf("%s %s\n", st.Bold(label), strings.Join(names, ", "))
}

func showDefinition(p *out.Printer, st Style, definition string) {
	if definition != "" {
		p.Printf("%s %s\n", st.Bold("Definition:"), definition)
	}
}

func showAliases(p *out.Printer, st Style, aliases []string) {
	if len(aliases) > 0 {
		p.Printf("%s %s\n", st.Bold("Aliases:"), strings.Join(aliases, ", "))
	}
}

// Explain prints the concept documentation: meaning, sources, the
// questions that recognize the concept, and what the project supplies.
func Explain(p *out.Printer, st Style, docs []vocab.ConceptDoc) {
	for i, doc := range docs {
		if i > 0 {
			p.Println()
		}
		p.Println(st.Bold(doc.Title))
		p.Println()
		p.Println(doc.Meaning)
		p.Println()
		if len(doc.Sources) > 0 {
			p.Println(st.Muted("Sources:"))
			p.Println()
			for _, s := range doc.Sources {
				p.Printf("  %s\n", s)
			}
			p.Println()
		}
		p.Println(st.Muted("Ask:"))
		p.Println()
		for _, q := range doc.Questions {
			p.Printf("  %s\n", q)
		}
		p.Println()
		p.Println(doc.Supplies)
		p.Printf("ArcLint supplies the meaning of %s.\n", doc.Title)
	}
}

// Define reports what a define did: the entry recorded, updated, or
// left as it was, and for an update each property that changed.
func Define(p *out.Printer, st Style, result application.DomainDefineResult, change vocab.Change) {
	subject := Subject(result.Concept, result.Name)
	where := ""
	if result.Owner != "" {
		where += " under " + result.Owner
	}
	if result.Concept != vocab.ConceptBoundedContext && result.Context != "" {
		where += " in context " + result.Context
	}
	switch result.Outcome {
	case vocab.OutcomeCreated:
		p.Printf("%s %s%s.\n", st.OK("Defined"), subject, where)
	case vocab.OutcomeUpdated:
		p.Printf("%s %s%s.\n", st.OK("Updated"), subject, where)
		for _, property := range result.Changed {
			p.Printf("  %s %s\n", st.Muted(property+":"), changedValue(property, change))
		}
	default:
		p.Printf("Unchanged %s%s.\n", subject, where)
	}
}

// changedValue spells what a changed property now holds: the value for
// short ones, "changed" or "cleared" for prose.
func changedValue(property string, ch vocab.Change) string {
	switch property {
	case "definition":
		return proseValue(ch.Definition)
	case "statement":
		return proseValue(ch.Statement)
	case "text":
		return proseValue(ch.Text)
	case "on":
		return shortValue(ch.On)
	case "identity":
		return shortValue(ch.Identity)
	case "aliases":
		return listValue(ch.Aliases)
	case "repository":
		return shortValue(ch.Repository)
	case "factory":
		return shortValue(ch.Factory)
	case "raised_by":
		return shortValue(ch.RaisedBy)
	default:
		return "changed"
	}
}

// cleared spells a property the change emptied.
const cleared = "cleared"

func proseValue(v string) string {
	if v == "" {
		return cleared
	}
	return "changed"
}

func shortValue(v string) string {
	if v == "" {
		return cleared
	}
	return v
}

func listValue(v []string) string {
	if len(v) == 0 {
		return cleared
	}
	return strings.Join(v, ", ")
}

// Remove reports what a remove took out of the model, and what went
// with it.
func Remove(p *out.Printer, st Style, r application.DomainRemoveResult) {
	p.Printf("%s %s from the project domain model.\n", st.OK("Removed"), Subject(r.Concept, r.Name))
	for _, also := range r.Also {
		p.Printf("  %s\n", also)
	}
	p.Println(st.Muted("Source files were not changed."))
}

// Subject spells an entry the way a sentence names it: the concept in
// words, then the name.
func Subject(c vocab.Concept, name string) string {
	return strings.ToLower(c.Doc().Title) + " " + name
}

// Knowledge prints the domain block of `arclint context`: the counts
// line (a scoped listing counts against the whole model), each context
// with its aggregates, value objects, contracts with their anchors,
// events, and services, the relations, and last the contracts no
// declaration carries so they cannot be skimmed past.
func Knowledge(p *out.Printer, st Style, d *application.DomainKnowledge) {
	p.Printf("\n%s (%s): %s\n", st.Bold("project domain"), d.Source, headline(d))
	for _, ctx := range d.Contexts {
		p.Printf("  %s %s:\n", st.Bold("context"), st.Bold(ctx.Name))
		if len(ctx.Aggregates) > 0 {
			names := make([]string, len(ctx.Aggregates))
			for i, a := range ctx.Aggregates {
				names[i] = aggregateRef(a)
			}
			p.Printf("    aggregates: %s\n", strings.Join(names, ", "))
		}
		if len(ctx.ValueObjects) > 0 {
			p.Printf("    value objects: %s\n", strings.Join(ctx.ValueObjects, ", "))
		}
		if len(ctx.Invariants) > 0 {
			p.Println("    invariants:")
			for _, inv := range ctx.Invariants {
				p.Printf("      %s (%s): %s%s\n", inv.Key, inv.Owner, inv.Statement, anchorSuffix(st, inv.Source, inv.Anchor))
			}
		}
		if len(ctx.Assertions) > 0 {
			p.Println("    assertions:")
			for _, a := range ctx.Assertions {
				p.Printf("      %s (%s, on %s): %s%s\n", a.Key, a.Owner, a.On, a.Statement, anchorSuffix(st, a.Source, a.Anchor))
			}
		}
		if len(ctx.Specifications) > 0 {
			p.Println("    specifications:")
			for _, s := range ctx.Specifications {
				p.Printf("      %s%s\n", s.Name, anchorSuffix(st, s.Source, s.Anchor))
			}
		}
		if len(ctx.Events) > 0 {
			p.Printf("    events: %s\n", strings.Join(ctx.Events, ", "))
		}
		if len(ctx.Services) > 0 {
			p.Printf("    services: %s\n", strings.Join(ctx.Services, ", "))
		}
	}
	for _, rel := range d.Relations {
		p.Printf("  relation: %s -[%s]-> %s\n", rel.From, rel.Kind, rel.To)
	}
	writeUnanchored(p, st, d)
}

func aggregateRef(a application.DomainAggregateRef) string {
	var b strings.Builder
	b.WriteString(a.Name)
	if a.Identity != "" || len(a.Entities) > 0 {
		b.WriteString(" (")
		b.WriteString(a.Identity)
		if len(a.Entities) > 0 {
			if a.Identity != "" {
				b.WriteString("; ")
			}
			b.WriteString(strings.Join(a.Entities, ", "))
		}
		b.WriteString(")")
	}
	return b.String()
}

// headline phrases the counts line: the whole model for a whole
// listing, the shown-of-recorded tallies for a scoped one, and the
// pointer to --full whenever the listing is scoped.
func headline(d *application.DomainKnowledge) string {
	if !d.Scoped {
		return CountsLine(d.Counts)
	}
	if d.Shown.Contexts == 0 {
		return fmt.Sprintf("nothing recorded anchors into this scope; --full shows the whole model (%s)", CountsLine(d.Counts))
	}
	return fmt.Sprintf("%s anchor into this scope; --full shows the whole model", shownPhrase(d))
}

func shownPhrase(d *application.DomainKnowledge) string {
	s, c := d.Shown, d.Counts
	parts := []string{fmt.Sprintf("%d of %s", s.Contexts, countPhrase(c.Contexts, "context", "contexts"))}
	for _, t := range []struct {
		shown, total     int
		singular, plural string
	}{
		{s.Aggregates, c.Aggregates, "aggregate", "aggregates"},
		{s.Entities, c.Entities, "entity", "entities"},
		{s.ValueObjects, c.ValueObjects, "value object", "value objects"},
		{s.Invariants, c.Invariants, "invariant", "invariants"},
		{s.Assertions, c.Assertions, "assertion", "assertions"},
		{s.Specifications, c.Specifications, "specification", "specifications"},
		{s.Events, c.Events, "event", "events"},
		{s.Services, c.Services, "service", "services"},
	} {
		if t.total > 0 {
			parts = append(parts, fmt.Sprintf("%d of %s", t.shown, countPhrase(t.total, t.singular, t.plural)))
		}
	}
	return strings.Join(parts, ", ")
}

// anchorSuffix renders a contract's anchor after its line: the source
// when found, the word "missing" otherwise, nothing when the contracts
// were never located.
func anchorSuffix(st Style, source string, anchor application.ContractAnchor) string {
	switch anchor {
	case application.AnchorFound:
		return " " + st.Path(source)
	case application.AnchorMissing:
		return " " + st.Warn(string(anchor))
	}
	return ""
}

// writeUnanchored prints the contracts no declaration carries, one
// line each with the declaration the recording expects, and the rule
// that turns each into a Violation.
func writeUnanchored(p *out.Printer, st Style, d *application.DomainKnowledge) {
	if len(d.Unanchored) == 0 {
		return
	}
	p.Printf("  %s %s\n", st.Bold("unanchored contracts:"), countPhrase(len(d.Unanchored), "missing", "missing"))
	for _, u := range d.Unanchored {
		p.Printf("    %s: %s\n", st.Warn("missing"), unanchoredLabel(u))
		p.Printf("      expected %s\n", u.Expected)
	}
	p.Println(st.Muted("    arclint check reports each as a Violation of the built-in rule of its block"))
}

func unanchoredLabel(u application.UnanchoredContract) string {
	switch u.Kind {
	case application.ContractSpecification:
		return fmt.Sprintf("specification %s (context %s)", u.Name, u.Context)
	case application.ContractAssertion:
		return fmt.Sprintf("assertion %s of %s (context %s)", u.Key, u.Owner, u.Context)
	default:
		return fmt.Sprintf("invariant %s of %s (context %s)", u.Key, u.Owner, u.Context)
	}
}

// contractSource finds one contract in the overview matrix and phrases
// its anchor: the source when found, "missing" otherwise; empty when
// no matrix was built.
func contractSource(matrix *application.DomainKnowledge, context string, kind application.ContractKind, owner, key string) string {
	if matrix == nil {
		return ""
	}
	for _, ctx := range matrix.Contexts {
		if ctx.Name != context {
			continue
		}
		switch kind {
		case application.ContractInvariant:
			for _, inv := range ctx.Invariants {
				if inv.Owner == owner && inv.Key == key {
					return anchorPhrase(inv.Source, inv.Anchor)
				}
			}
		case application.ContractAssertion:
			for _, a := range ctx.Assertions {
				if a.Owner == owner && a.Key == key {
					return anchorPhrase(a.Source, a.Anchor)
				}
			}
		case application.ContractSpecification:
			for _, s := range ctx.Specifications {
				if s.Name == key {
					return anchorPhrase(s.Source, s.Anchor)
				}
			}
		}
	}
	return ""
}

func anchorPhrase(source string, anchor application.ContractAnchor) string {
	switch anchor {
	case application.AnchorFound:
		return source
	case application.AnchorMissing:
		return string(anchor)
	}
	return ""
}

func writeSourceLine(p *out.Printer, st Style, indent, source string) {
	switch source {
	case "":
	case string(application.AnchorMissing):
		p.Printf("%s%s %s\n", indent, st.Muted("source:"), st.Warn(source))
	default:
		p.Printf("%s%s %s\n", indent, st.Muted("source:"), st.Path(source))
	}
}

// namedLine pairs a name or key with its text, or stands alone when
// the text is empty.
func namedLine(name, text string) string {
	if text == "" {
		return name
	}
	return name + "  " + text
}

func relationLine(rel vocab.ContextRelation) string {
	line := fmt.Sprintf("%s -[%s]-> %s", rel.From, rel.Kind, rel.To)
	if rel.Description != "" {
		line += "  " + rel.Description
	}
	return line
}

func countPhrase(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
}
