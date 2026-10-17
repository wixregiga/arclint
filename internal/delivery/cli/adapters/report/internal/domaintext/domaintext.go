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
	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
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

// Export reports a translation of the project's recorded language into
// another file.
func Export(p *out.Printer, st Style, r application.ExportDomainRecordingResult) {
	p.Printf("%s the domain from %s into %s (%s): %s.\n",
		st.OK("Translated"), r.Source.Path(), st.Bold(r.Target.Path()), strings.ToUpper(string(r.Target.Format())), CountsLine(r.Counts))
	p.Println("Read back, it records the same language.")
	if r.Updated {
		p.Printf("%s already recorded a language; it was edited in place and keeps everything else it holds.\n", r.Target.Path())
	}
}

// Convert reports the project moving its recorded language into another
// file.
func Convert(p *out.Printer, st Style, r application.ConvertDomainRecordingResult) {
	p.Printf("%s the domain from %s into %s (%s): %s.\n",
		st.OK("Moved"), r.Source.Path(), st.Bold(r.Target.Path()), strings.ToUpper(string(r.Target.Format())), CountsLine(r.Counts))
	p.Println("Read back, it records the same language.")
	if r.Updated {
		p.Printf("%s already recorded a language; it was edited in place and keeps everything else it holds.\n", r.Target.Path())
	}
	switch {
	case r.Named.IsZero() && r.RulesetChanged:
		p.Printf("%s no longer names a domain file: %s is found by its name.\n", rule.RulesetFileName, r.Target.Path())
	case r.Named.IsZero():
		p.Printf("%s names no domain file: %s is found by its name.\n", rule.RulesetFileName, r.Target.Path())
	case r.RulesetChanged:
		p.Printf("%s now names %s under domain.file.\n", rule.RulesetFileName, r.Target.Path())
	default:
		p.Printf("%s already named %s.\n", rule.RulesetFileName, r.Target.Path())
	}
	p.Printf("Removed %s.\n", r.Source.Path())
}

// Missing is the guidance printed when no domain file is recorded at
// source.
func Missing(p *out.Printer, st Style, source string) {
	p.Println(st.Bold("No recorded Ubiquitous Language found at " + source + "."))
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
					writeVerifiedBy(p, st, "        ", inv.VerifiedBy)
					writeSourceLine(p, st, "        ", contractSource(o.Matrix, ctx.Name, application.ContractInvariant, a.Name, inv.Key))
				}
			}
			if len(a.Assertions) > 0 {
				p.Printf("    %s\n", st.Muted("assertions:"))
				for _, as := range a.Assertions {
					p.Printf("      %s\n", namedLine(as.Key+" (on "+as.On+")", as.Statement))
					writeVerifiedBy(p, st, "        ", as.VerifiedBy)
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
					writeVerifiedBy(p, st, "        ", inv.VerifiedBy)
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
				if len(s.Assertions) > 0 {
					p.Printf("      %s\n", st.Muted("assertions:"))
					for _, as := range s.Assertions {
						p.Printf("        %s\n", namedLine(as.Key+" (on "+as.On+")", as.Statement))
						writeVerifiedBy(p, st, "          ", as.VerifiedBy)
						writeSourceLine(p, st, "          ", contractSource(o.Matrix, ctx.Name, application.ContractAssertion, s.Name, as.Key))
					}
				}
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
		{c.Aggregates, aggregateNoun, aggregatesNoun},
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
				writeVerifiedBy(p, st, "    ", inv.VerifiedBy)
				ruleDetail(p, st, v.Rules, vocab.ConceptInvariant, inv.Key, "    ")
			}
		}
		if len(a.Assertions) > 0 {
			p.Println(st.Bold("Assertions:"))
			for _, as := range a.Assertions {
				p.Printf("  %s\n", namedLine(as.Key+" (on "+as.On+")", as.Statement))
				writeVerifiedBy(p, st, "    ", as.VerifiedBy)
				ruleDetail(p, st, v.Rules, vocab.ConceptAssertion, as.Key, "    ")
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
				writeVerifiedBy(p, st, "    ", inv.VerifiedBy)
				ruleDetail(p, st, v.Rules, vocab.ConceptInvariant, inv.Key, "    ")
			}
		}
	case vocab.ConceptInvariant:
		p.Printf("%s %s\n", st.Bold("Owner:"), v.Owner)
		p.Printf("%s %s\n", st.Bold("Statement:"), v.Invariant.Statement)
		writeVerifiedBy(p, st, "", v.Invariant.VerifiedBy)
		ruleDetail(p, st, v.Rules, vocab.ConceptInvariant, v.Invariant.Key, "")
	case vocab.ConceptAssertion:
		p.Printf("%s %s\n", st.Bold("Owner:"), v.Owner)
		p.Printf("%s %s\n", st.Bold("On:"), v.Assertion.On)
		p.Printf("%s %s\n", st.Bold("Statement:"), v.Assertion.Statement)
		writeVerifiedBy(p, st, "", v.Assertion.VerifiedBy)
		ruleDetail(p, st, v.Rules, vocab.ConceptAssertion, v.Assertion.Key, "")
	case vocab.ConceptDomainEvent:
		if v.Event.RaisedBy != "" {
			p.Printf("%s %s\n", st.Bold("Raised by:"), v.Event.RaisedBy)
		}
		showDefinition(p, st, v.Event.Definition)
	case vocab.ConceptDomainService:
		showDefinition(p, st, v.Service.Definition)
		if len(v.Service.Assertions) > 0 {
			p.Println(st.Bold("Assertions:"))
			for _, as := range v.Service.Assertions {
				p.Printf("  %s\n", namedLine(as.Key+" (on "+as.On+")", as.Statement))
				writeVerifiedBy(p, st, "    ", as.VerifiedBy)
				ruleDetail(p, st, v.Rules, vocab.ConceptAssertion, as.Key, "    ")
			}
		}
	case vocab.ConceptSpecification:
		showDefinition(p, st, v.Specification.Definition)
	case vocab.ConceptQuestion:
		p.Printf("%s %s\n", st.Bold("Text:"), v.Question.Text)
	case vocab.ConceptAggregateRoot, vocab.ConceptBusinessRule:
		// Resolved to the aggregate, invariant, or assertion they name
		// before a view exists; no view carries them.
	}
	rulesSummary(p, st, v)
}

// ruleDetail prints, under one recorded invariant or assertion, the
// code enforcing it and the current check's status of it.
func ruleDetail(p *out.Printer, st Style, rules *application.DomainEntryRules, kind vocab.Concept, key, indent string) {
	if rules == nil || !rules.Checked {
		return
	}
	for _, c := range rules.Checks {
		if c.Kind != kind || c.Key != key {
			continue
		}
		label := func(s string) string {
			if indent == "" {
				return st.Bold(strings.ToUpper(s[:1]) + s[1:] + ":")
			}
			return st.Muted(s + ":")
		}
		switch {
		case c.Source != "":
			p.Printf("%s%s %s\n", indent, label("enforced at"), st.Path(c.Source))
		case c.Expected != "":
			p.Printf("%s%s %s\n", indent, label("enforced at"), st.Warn("not found; expected "+c.Expected))
		default:
			p.Printf("%s%s %s\n", indent, label("enforced at"), st.Warn("not located"))
		}
		p.Printf("%s%s %s\n", indent, label("status"), ruleStatus(st, c))
		for _, f := range c.Findings {
			line := f.Message
			if f.Path != "" {
				line += fmt.Sprintf(" (%s:%d)", f.Path, f.Line)
			}
			if f.Status != "" && f.Status != "active" {
				line = f.Status + ": " + line
			}
			p.Printf("%s  %s %s\n", indent, st.Warn("finding:"), line)
		}
		for _, r := range c.Reasons {
			p.Printf("%s  %s\n", indent, r)
		}
		for _, r := range c.NotChecked {
			p.Printf("%s  %s %s\n", indent, st.Muted("not checked:"), r)
		}
	}
}

func ruleStatus(st Style, c application.DomainRuleCheck) string {
	switch c.Status {
	case application.RuleConforms:
		return st.OK("conforms")
	case application.RuleFinding:
		return st.Warn("finding")
	case application.RuleNotObservable:
		return st.Warn("not observable in the languages ArcLint analyzes")
	case application.RuleUndetermined:
		return st.Warn("undetermined")
	case application.RuleCheckFailed:
		return st.Warn("check failed")
	default:
		return st.Warn("not checked")
	}
}

// rulesSummary closes a show: what an entry with no rules can do, why
// no status was given, and what the structural checks do not prove.
func rulesSummary(p *out.Printer, st Style, v application.DomainEntryView) {
	rules := v.Rules
	switch {
	case rules == nil:
	case rules.Aggregate != "":
		p.Printf("%s none of its own; its aggregate %s records them: %s\n", st.Bold("Rules:"), rules.Aggregate, rules.ShowAggregate)
	case len(rules.Checks) == 0:
		p.Printf("%s %s records no %s; record one with: %s\n", st.Bold("Rules:"), v.Name, recordableRules(v.Concept), rules.Record)
	case rules.CheckError != "":
		p.Printf("%s not checked: %s\n", st.Bold("Rule status:"), rules.CheckError)
	case !rules.Checked:
		p.Printf("%s not checked: no check was run\n", st.Bold("Rule status:"))
	default:
		p.Println(st.Muted("Statuses come from the structural checks of the current code: they find the enforcing method and its calls, not whether its body enforces the statement."))
	}
}

// recordableRules names the rules a concept records.
func recordableRules(c vocab.Concept) string {
	switch c {
	case vocab.ConceptValueObject:
		return string(vocab.ConceptInvariant)
	case vocab.ConceptDomainService:
		return string(vocab.ConceptAssertion)
	default:
		return string(vocab.ConceptInvariant) + " or " + string(vocab.ConceptAssertion)
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
	if result.Target != "" {
		subject = result.Target + " " + result.Name
	}
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
			p.Printf("  %s %s\n", st.Muted(property+":"), defineValue(property, result, change))
		}
	default:
		p.Printf("Unchanged %s%s.\n", subject, where)
	}
}

func defineValue(property string, result application.DomainDefineResult, change vocab.Change) string {
	if value, ok := result.Values[property]; ok {
		if property == "description" {
			return proseValue(value)
		}
		return shortValue(value)
	}
	return changedValue(property, change)
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
	case "verified_by":
		return listValue(ch.VerifiedBy)
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
	subject := Subject(r.Concept, r.Name)
	if r.Target != "" {
		subject = r.Target + " " + r.Name
	}
	p.Printf("%s %s from the project domain model.\n", st.OK("Removed"), subject)
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
// events, and services, declared here when the listing is scoped, the
// relations, the entries the worksite uses with the lines that reach
// them, the references resolution could not follow, and last the
// contracts no declaration carries so they cannot be skimmed past.
func Knowledge(p *out.Printer, st Style, d *application.DomainKnowledge) {
	p.Printf("\n%s (%s): %s\n", st.Bold("project domain"), d.Source, headline(d))
	heading := "context"
	if d.Scoped {
		heading = "declared here, context"
	}
	for _, ctx := range d.Contexts {
		p.Printf("  %s %s:\n", st.Bold(heading), st.Bold(ctx.Name))
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
				writeVerifiedBy(p, st, "        ", inv.VerifiedBy)
			}
		}
		if len(ctx.Assertions) > 0 {
			p.Println("    assertions:")
			for _, a := range ctx.Assertions {
				p.Printf("      %s (%s, on %s): %s%s\n", a.Key, a.Owner, a.On, a.Statement, anchorSuffix(st, a.Source, a.Anchor))
				writeVerifiedBy(p, st, "        ", a.VerifiedBy)
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
	writeUsed(p, st, d)
	writeUndetermined(p, st, d)
	writeUnanchored(p, st, d)
}

// writeVerifiedBy prints the tests an invariant or assertion names
// under verified_by, and what arclint checks of them: that each exists,
// never that it proves the statement.
func writeVerifiedBy(p *out.Printer, st Style, indent string, tests []string) {
	if len(tests) == 0 {
		return
	}
	p.Printf("%s%s %s %s\n", indent, st.Muted("verified by:"), strings.Join(tests, ", "),
		st.Muted("(arclint checks that each test exists, not what it proves)"))
}

// writeUsed prints the entries the worksite uses, context by context:
// each entry with the lines that reach it and the contracts that bind
// the code reaching it.
func writeUsed(p *out.Printer, st Style, d *application.DomainKnowledge) {
	var contexts []string
	byContext := map[string][]application.DomainUseRef{}
	for _, u := range d.Used {
		if _, seen := byContext[u.Context]; !seen {
			contexts = append(contexts, u.Context)
		}
		byContext[u.Context] = append(byContext[u.Context], u)
	}
	for _, name := range contexts {
		p.Printf("  %s %s:\n", st.Bold("used here, context"), st.Bold(name))
		for _, u := range byContext[name] {
			p.Printf("    %s: %s\n", usedLabel(u), st.Path(siteLines(u.Sites)))
			for _, inv := range u.Invariants {
				p.Printf("      invariant %s (%s): %s%s\n", inv.Key, inv.Owner, inv.Statement, anchorSuffix(st, inv.Source, inv.Anchor))
				writeVerifiedBy(p, st, "        ", inv.VerifiedBy)
			}
			for _, a := range u.Assertions {
				p.Printf("      assertion %s (%s, on %s): %s%s\n", a.Key, a.Owner, a.On, a.Statement, anchorSuffix(st, a.Source, a.Anchor))
				writeVerifiedBy(p, st, "        ", a.VerifiedBy)
			}
			if s := u.Specification; s != nil {
				p.Printf("      satisfaction method%s\n", anchorSuffix(st, s.Source, s.Anchor))
			}
		}
	}
}

// usedLabel names a used entry by its role, with the aggregates it
// belongs to, identifies, or serves.
func usedLabel(u application.DomainUseRef) string {
	switch u.Role {
	case conformance.UsedEntity:
		return fmt.Sprintf("entity %s of aggregate %s", u.Name, andList(u.Aggregates))
	case conformance.UsedRepository:
		return fmt.Sprintf("repository %s of %s %s", u.Name, aggregateWord(u.Aggregates), andList(u.Aggregates))
	case conformance.UsedFactory:
		return fmt.Sprintf("factory %s of %s %s", u.Name, aggregateWord(u.Aggregates), andList(u.Aggregates))
	case conformance.UsedValueObject:
		if len(u.Aggregates) > 0 {
			return fmt.Sprintf("value object %s, identity of %s", u.Name, andList(u.Aggregates))
		}
		return "value object " + u.Name
	case conformance.UsedDomainEvent:
		return "event " + u.Name
	case conformance.UsedDomainService:
		return "service " + u.Name
	case conformance.UsedSpecification:
		return "specification " + u.Name
	case conformance.UsedAggregate:
		return "aggregate " + u.Name
	}
	return string(u.Role) + " " + u.Name
}

// The noun for one aggregate and for several, in counts and labels.
const (
	aggregateNoun  = "aggregate"
	aggregatesNoun = "aggregates"
)

// aggregateWord is the noun for one aggregate name or for several.
func aggregateWord(names []string) string {
	if len(names) > 1 {
		return aggregatesNoun
	}
	return aggregateNoun
}

// andList joins names as prose: A, A and B, A, B and C.
func andList(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// siteLines spells the sites reaching one entry as each file once with
// its lines: a.go:3,9; b.go:4.
func siteLines(sites []application.DomainUseSite) string {
	var files []string
	lines := map[string][]string{}
	for _, s := range sites {
		if _, seen := lines[s.Path]; !seen {
			files = append(files, s.Path)
		}
		l := fmt.Sprint(s.Line)
		if n := len(lines[s.Path]); n == 0 || lines[s.Path][n-1] != l {
			lines[s.Path] = append(lines[s.Path], l)
		}
	}
	parts := make([]string, len(files))
	for i, f := range files {
		parts[i] = f + ":" + strings.Join(lines[f], ",")
	}
	return strings.Join(parts, "; ")
}

// writeUndetermined prints the references of the worksite that may be
// uses resolution cannot confirm, grouped by file and reason with their
// lines, so a reader knows where a use may go unlisted.
func writeUndetermined(p *out.Printer, st Style, d *application.DomainKnowledge) {
	if len(d.Undetermined) == 0 {
		return
	}
	phrase := "1 reference may be a use that resolution cannot confirm"
	if n := len(d.Undetermined); n != 1 {
		phrase = fmt.Sprintf("%d references may be uses that resolution cannot confirm", n)
	}
	p.Printf("  %s %s\n", st.Bold("undetermined here:"), phrase)
	type group struct{ path, reason string }
	var order []group
	lines := map[group][]string{}
	for _, u := range d.Undetermined {
		g := group{u.Path, u.Reason}
		if _, seen := lines[g]; !seen {
			order = append(order, g)
			lines[g] = nil
		}
		if u.Line > 0 {
			lines[g] = append(lines[g], fmt.Sprint(u.Line))
		}
	}
	for _, g := range order {
		at := g.path
		if len(lines[g]) > 0 {
			at += ":" + strings.Join(lines[g], ",")
		}
		p.Printf("    %s: %s\n", st.Path(at), g.reason)
	}
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
	uses := usesPhrase(d)
	if !d.Scoped {
		if uses == "" {
			return CountsLine(d.Counts)
		}
		return CountsLine(d.Counts) + "; " + uses
	}
	if d.Shown.Contexts == 0 {
		switch {
		case uses == "":
			return fmt.Sprintf("nothing recorded is declared here; --full shows the whole model (%s)", CountsLine(d.Counts))
		case len(d.Used) == 0 && len(d.Undetermined) == 0:
			return fmt.Sprintf("nothing recorded is declared or used here; --full shows the whole model (%s)", CountsLine(d.Counts))
		}
		return fmt.Sprintf("nothing recorded is declared here; %s; --full shows the whole model (%s)", uses, CountsLine(d.Counts))
	}
	declared := shownPhrase(d) + " declared here"
	if uses != "" {
		declared += "; " + uses
	}
	return declared + "; --full shows the whole model"
}

// usesPhrase counts the entries the worksite uses and the references
// left undetermined; empty when the uses were not looked for.
func usesPhrase(d *application.DomainKnowledge) string {
	if !d.UsesLocated {
		return ""
	}
	phrase := countPhrase(len(d.Used), "entry", "entries") + " used here"
	if len(d.Used) == 0 {
		phrase = "no entry used here"
	}
	if n := len(d.Undetermined); n > 0 {
		phrase += ", " + countPhrase(n, "reference", "references") + " undetermined"
	}
	return phrase
}

func shownPhrase(d *application.DomainKnowledge) string {
	s, c := d.Shown, d.Counts
	parts := []string{fmt.Sprintf("%d of %s", s.Contexts, countPhrase(c.Contexts, "context", "contexts"))}
	for _, t := range []struct {
		shown, total     int
		singular, plural string
	}{
		{s.Aggregates, c.Aggregates, aggregateNoun, aggregatesNoun},
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
