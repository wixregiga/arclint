package lipgloss

import (
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

func writeDomainInit(p *out.Printer, th Theme, result application.InitDomainResult) {
	if result.Created {
		p.Printf("%s %s.\n", th.OK.Render("Initialized"), result.Source)
		return
	}
	p.Printf("%s already exists; left unchanged.\n", result.Source)
}

func writeDomainMissing(p *out.Printer, th Theme) {
	p.Println("No recorded Ubiquitous Language found at " + vocab.UbiquitousLanguageFileName + ".")
	p.Println()
	p.Println(th.Bold.Render("Initialize an empty model:"))
	p.Println("  arclint domain init")
	p.Println()
	p.Println(th.Bold.Render("Define one item:"))
	p.Println("  arclint domain define entity <name> --context <context> --definition <text>")
	p.Println()
	p.Println(th.Bold.Render("Start guided authoring:"))
	p.Println("  arclint domain define --guided")
}

func writeDomainOverview(p *out.Printer, th Theme, result application.DomainOverview) {
	lang := result.Language
	counts := lang.Counts()
	p.Println(th.Bold.Render("Project domain"))
	p.Printf("%s %s\n", th.Muted.Render("Source:"), result.Source)
	p.Println()
	p.Printf("%s · %s · %s · %s · %s · %s · %s · %s\n",
		countPhrase(counts.Contexts, "Context", "Contexts"),
		countPhrase(counts.Entities, "Entity", "Entities"),
		countPhrase(counts.Aggregates, "Aggregate", "Aggregates"),
		countPhrase(counts.ValueObjects, "Value Object", "Value Objects"),
		countPhrase(counts.Invariants, "Invariant", "Invariants"),
		countPhrase(counts.Assertions, "Assertion", "Assertions"),
		countPhrase(counts.Specifications, "Specification", "Specifications"),
		countPhrase(counts.Events, "Event", "Events"),
	)

	for _, ctx := range lang.ListContexts() {
		p.Println()
		p.Printf("%s %s\n", th.Bold.Render("Context"), th.Bold.Render(ctx.Name))

		var aggregates, plain []vocab.Entity
		for _, e := range ctx.Entities {
			if e.Aggregate {
				aggregates = append(aggregates, e)
			} else {
				plain = append(plain, e)
			}
		}
		if len(aggregates) > 0 {
			p.Println()
			if len(aggregates) == 1 {
				p.Println(th.Bold.Render("  Aggregate"))
			} else {
				p.Println(th.Bold.Render("  Aggregates"))
			}
			p.Println()
			writeTwoLineEntities(p, aggregates, "  ")
		}
		if len(plain) > 0 {
			p.Println()
			p.Println(th.Bold.Render("  Entities"))
			p.Println()
			writePaddedEntityOneLiners(p, plain, "  ")
		}
		if len(ctx.Invariants) > 0 {
			p.Println()
			p.Println(th.Bold.Render("  Invariants"))
			p.Println()
			for i, inv := range ctx.Invariants {
				if i > 0 {
					p.Println()
				}
				p.Printf("    %s\n", inv.Statement)
				p.Printf("    %s %s\n", th.Muted.Render("owner:"), inv.Owner)
				if inv.ID != "" {
					p.Printf("    %s %s\n", th.Muted.Render("id:"), inv.ID)
				}
			}
		}
		if len(ctx.Assertions) > 0 {
			p.Println()
			p.Println(th.Bold.Render("  Assertions"))
			p.Println()
			for i, a := range ctx.Assertions {
				if i > 0 {
					p.Println()
				}
				p.Printf("    %s\n", a.Statement)
				p.Printf("    %s %s\n", th.Muted.Render("owner:"), a.Owner)
				p.Printf("    %s %s\n", th.Muted.Render("id:"), a.ID)
				p.Printf("    %s %s\n", th.Muted.Render("on:"), a.On)
			}
		}
		if len(ctx.Specifications) > 0 {
			p.Println()
			p.Println(th.Bold.Render("  Specifications"))
			p.Println()
			for _, s := range ctx.Specifications {
				p.Printf("    %s\n", s.Name)
				if s.Definition != "" {
					p.Printf("    %s\n", s.Definition)
				}
			}
		}
		if len(ctx.ValueObjects) > 0 {
			p.Println()
			p.Println(th.Bold.Render("  Value objects"))
			p.Println()
			writePaddedOneLiners(p, ctx.ValueObjects, "  ")
		}
		if len(ctx.Events) > 0 {
			p.Println()
			p.Println(th.Bold.Render("  Domain events"))
			p.Println()
			writePaddedOneLiners(p, ctx.Events, "  ")
		}
	}

	if len(lang.Relations) > 0 {
		p.Println()
		p.Println(th.Bold.Render("Relations"))
		p.Println()
		for _, rel := range lang.Relations {
			p.Printf("  %s -[%s]-> %s\n", rel.From, rel.Kind, rel.To)
		}
	}
}

func writeDomainList(p *out.Printer, th Theme, result application.DomainListing) {
	p.Println(th.Bold.Render("Project domain"))
	contexts := selectedContexts(result)
	if result.Filtered {
		for _, ctx := range contexts {
			p.Println()
			p.Printf("%s %s\n", th.Bold.Render("Context"), th.Bold.Render(ctx.Name))
			switch result.Concept {
			case vocab.ConceptEntity:
				writeEntityListGroup(p, th, "  Entities", ctx.Entities, true)
			case vocab.ConceptAggregate, vocab.ConceptAggregateRoot:
				var aggs []vocab.Entity
				for _, e := range ctx.Entities {
					if e.Aggregate {
						aggs = append(aggs, e)
					}
				}
				writeEntityListGroup(p, th, "  "+listGroupHeader(result.Concept), aggs, false)
			case vocab.ConceptValueObject:
				writeListGroup(p, th, "  Value objects", ctx.ValueObjects)
			case vocab.ConceptInvariant, vocab.ConceptBusinessRule:
				writeInvariantListGroup(p, th, "  "+listGroupHeader(result.Concept), ctx.Invariants)
			case vocab.ConceptAssertion:
				writeAssertionListGroup(p, th, "  "+listGroupHeader(result.Concept), ctx.Assertions)
			case vocab.ConceptSpecification:
				writeSpecificationListGroup(p, th, "  "+listGroupHeader(result.Concept), ctx.Specifications)
			case vocab.ConceptDomainEvent:
				writeListGroup(p, th, "  Domain events", ctx.Events)
			case vocab.ConceptBoundedContext:
				p.Printf("  %s\n", ctx.Name)
			default:
				writeListGroup(p, th, "  "+listGroupHeader(result.Concept), nil)
			}
		}
		return
	}
	for _, ctx := range contexts {
		p.Println()
		p.Printf("%s %s\n", th.Bold.Render("Context"), th.Bold.Render(ctx.Name))
		writeEntityListGroup(p, th, "  Entities", ctx.Entities, true)
		writeListGroup(p, th, "  Value objects", ctx.ValueObjects)
		writeInvariantListGroup(p, th, "  Invariants", ctx.Invariants)
		writeAssertionListGroup(p, th, "  Assertions", ctx.Assertions)
		writeSpecificationListGroup(p, th, "  Specifications", ctx.Specifications)
		writeListGroup(p, th, "  Domain events", ctx.Events)
	}
	if result.Context == "" && len(result.Language.Relations) > 0 {
		p.Println()
		p.Println(th.Bold.Render("Relations"))
		for _, rel := range result.Language.Relations {
			p.Printf("  %s -[%s]-> %s\n", rel.From, rel.Kind, rel.To)
		}
	}
}

func selectedContexts(result application.DomainListing) []vocab.BoundedContext {
	all := result.Language.ListContexts()
	if result.Context == "" {
		return all
	}
	for _, ctx := range all {
		if ctx.Name == result.Context {
			return []vocab.BoundedContext{ctx}
		}
	}
	return nil
}

func writeListGroup(p *out.Printer, th Theme, header string, defs []vocab.Definition) {
	if len(defs) == 0 {
		return
	}
	p.Println()
	p.Println(th.Bold.Render(header))
	for _, d := range sortedDefs(defs) {
		p.Printf("    %s\n", d.Name)
	}
}

func writeInvariantListGroup(p *out.Printer, th Theme, header string, invs []vocab.Invariant) {
	if len(invs) == 0 {
		return
	}
	p.Println()
	p.Println(th.Bold.Render(header))
	sorted := append([]vocab.Invariant(nil), invs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Statement < sorted[j].Statement })
	for _, inv := range sorted {
		if inv.ID != "" {
			p.Printf("    %s (owner: %s, id: %s)\n", inv.Statement, inv.Owner, inv.ID)
			continue
		}
		p.Printf("    %s (owner: %s)\n", inv.Statement, inv.Owner)
	}
}

func writeAssertionListGroup(p *out.Printer, th Theme, header string, assertions []vocab.Assertion) {
	if len(assertions) == 0 {
		return
	}
	p.Println()
	p.Println(th.Bold.Render(header))
	sorted := append([]vocab.Assertion(nil), assertions...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Statement < sorted[j].Statement })
	for _, a := range sorted {
		p.Printf("    %s (owner: %s, id: %s, on: %s)\n", a.Statement, a.Owner, a.ID, a.On)
	}
}

func writeSpecificationListGroup(p *out.Printer, th Theme, header string, specs []vocab.Specification) {
	if len(specs) == 0 {
		return
	}
	p.Println()
	p.Println(th.Bold.Render(header))
	sorted := append([]vocab.Specification(nil), specs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	for _, s := range sorted {
		p.Printf("    %s\n", s.Name)
	}
}

func writeEntityListGroup(p *out.Printer, th Theme, header string, entities []vocab.Entity, markAggregate bool) {
	if len(entities) == 0 {
		return
	}
	p.Println()
	p.Println(th.Bold.Render(header))
	for _, e := range sortedEntities(entities) {
		if markAggregate && e.Aggregate {
			p.Printf("    %s %s\n", e.Name, th.Muted.Render("[aggregate]"))
			continue
		}
		p.Printf("    %s\n", e.Name)
	}
}

func writeDomainShow(p *out.Printer, th Theme, result application.DomainDefinitionView) {
	doc := result.Concept.Doc()
	p.Printf("%s: %s\n", th.Bold.Render(doc.Title), th.Bold.Render(result.Definition.Name))
	if result.Context != "" {
		p.Printf("%s %s\n", th.Bold.Render("Context:"), result.Context)
	}
	if isEntityShow(result.Concept) {
		if result.Aggregate {
			p.Println(th.Bold.Render("Aggregate:") + " yes")
		} else if result.Concept == vocab.ConceptEntity {
			p.Println(th.Bold.Render("Aggregate:") + " no")
		}
	}
	if isInvariantShow(result.Concept) {
		if result.Owner != "" {
			p.Printf("%s %s\n", th.Bold.Render("Owner:"), result.Owner)
		}
		if result.ID != "" {
			p.Printf("%s %s\n", th.Bold.Render("ID:"), result.ID)
		}
		if result.On != "" {
			p.Printf("%s %s\n", th.Bold.Render("On:"), result.On)
		}
		return
	}
	if result.Definition.Definition != "" {
		p.Printf("%s %s\n", th.Bold.Render("Definition:"), result.Definition.Definition)
	}
	if len(result.Definition.Aliases) > 0 {
		p.Println(th.Bold.Render("Aliases:"))
		for _, alias := range result.Definition.Aliases {
			p.Printf("  %s\n", alias)
		}
	}
}

func isEntityShow(c vocab.Concept) bool {
	return c == vocab.ConceptEntity || c == vocab.ConceptAggregate || c == vocab.ConceptAggregateRoot
}

func isInvariantShow(c vocab.Concept) bool {
	return c == vocab.ConceptInvariant || c == vocab.ConceptAssertion || c == vocab.ConceptBusinessRule
}

func writeDomainExplain(p *out.Printer, th Theme, docs []vocab.ConceptDoc) {
	for i, doc := range docs {
		if i > 0 {
			p.Println()
		}
		p.Println(th.Bold.Render(doc.Title))
		p.Println()
		p.Println(doc.Meaning)
		p.Println()
		p.Println(th.Bold.Render("Ask:"))
		p.Println()
		for _, q := range doc.Questions {
			p.Printf("  %s\n", q)
		}
		p.Println()
		p.Println(doc.Supplies)
		p.Printf("ArcLint supplies the meaning of %s.\n", doc.Title)
	}
}

func writeDomainDefine(p *out.Printer, th Theme, rep cli.DomainDefineReport) {
	result := rep.Result
	typeSpelling := string(result.Concept)
	switch result.Outcome {
	case vocab.OutcomeCreated:
		p.Printf("%s %s %s.\n", th.OK.Render("Defined"), typeSpelling, th.Bold.Render(result.Name))
	case vocab.OutcomeUpdated:
		p.Printf("%s %s %s.\n", th.OK.Render("Updated"), typeSpelling, th.Bold.Render(result.Name))
		for _, field := range result.Changed {
			switch field {
			case "definition":
				if rep.Definition != nil && *rep.Definition == "" {
					p.Println("  definition: cleared")
				} else {
					p.Println("  definition: changed")
				}
			case "aliases":
				if rep.ClearAliases || len(result.Aliases) == 0 {
					p.Println("  aliases: cleared")
				} else {
					p.Printf("  aliases: %s\n", strings.Join(result.Aliases, ", "))
				}
			case "aggregate":
				p.Println("  aggregate: designated")
			case "owner":
				p.Println("  owner: changed")
			}
		}
	default:
		p.Printf("%s %s %s.\n", th.Muted.Render("Unchanged"), typeSpelling, result.Name)
	}
}

func writeDomainRemove(p *out.Printer, th Theme, result application.DomainRemoveResult) {
	if result.Concept == vocab.ConceptAggregate || result.Concept == vocab.ConceptAggregateRoot || result.EntityPreserved {
		p.Printf("Removed the Aggregate designation from %s.\n", th.Bold.Render(result.Name))
		p.Printf("The %s Entity remains defined.\n", result.Name)
		return
	}
	p.Printf("Removed %s %s from the project domain model.\n", result.Concept, th.Bold.Render(result.Name))
	p.Println(th.Muted.Render("Source files were not changed."))
}

func writeTwoLineEntities(p *out.Printer, entities []vocab.Entity, indent string) {
	for i, e := range entities {
		if i > 0 {
			p.Println()
		}
		p.Printf("%s  %s\n", indent, e.Name)
		if e.Definition.Definition != "" {
			p.Printf("%s  %s\n", indent, e.Definition.Definition)
		}
	}
}

func writePaddedOneLiners(p *out.Printer, defs []vocab.Definition, indent string) {
	width := 0
	for _, d := range defs {
		if d.Definition != "" && len(d.Name) > width {
			width = len(d.Name)
		}
	}
	for _, d := range defs {
		if d.Definition == "" {
			p.Printf("%s  %s\n", indent, d.Name)
			continue
		}
		p.Printf("%s  %-*s — %s\n", indent, width, d.Name, d.Definition)
	}
}

func writePaddedEntityOneLiners(p *out.Printer, entities []vocab.Entity, indent string) {
	width := 0
	for _, e := range entities {
		if e.Definition.Definition != "" && len(e.Name) > width {
			width = len(e.Name)
		}
	}
	for _, e := range entities {
		if e.Definition.Definition == "" {
			p.Printf("%s  %s\n", indent, e.Name)
			continue
		}
		p.Printf("%s  %-*s — %s\n", indent, width, e.Name, e.Definition.Definition)
	}
}

func countPhrase(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return itoa(n) + " " + plural
}

func sortedDefs(defs []vocab.Definition) []vocab.Definition {
	out := append([]vocab.Definition(nil), defs...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func sortedEntities(entities []vocab.Entity) []vocab.Entity {
	out := append([]vocab.Entity(nil), entities...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func listGroupHeader(c vocab.Concept) string {
	switch c {
	case vocab.ConceptEntity:
		return "Entities"
	case vocab.ConceptAggregate:
		return "Aggregates"
	case vocab.ConceptAggregateRoot:
		return "Aggregate roots"
	case vocab.ConceptValueObject:
		return "Value objects"
	case vocab.ConceptInvariant:
		return "Invariants"
	case vocab.ConceptAssertion:
		return "Assertions"
	case vocab.ConceptSpecification:
		return "Specifications"
	case vocab.ConceptBusinessRule:
		return "Business rules"
	case vocab.ConceptDomainEvent:
		return "Domain events"
	case vocab.ConceptBoundedContext:
		return "Bounded contexts"
	default:
		return vocab.Listing(c)
	}
}
