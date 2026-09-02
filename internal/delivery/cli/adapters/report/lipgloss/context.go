package lipgloss

import (
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
)

const contractSourceMissing = "missing"

func writeContext(p *out.Printer, th Theme, c application.ArchitecturalContext) {
	p.Printf("%s %s\n", th.Bold.Render("scope:"), c.Scope)
	for _, b := range c.Paths {
		owned := strings.Join(b.Modules, ", ")
		if owned == "" {
			owned = "no declared module"
		}
		p.Printf("  %s → %s\n", th.Path.Render(b.Path), owned)
	}
	if len(c.Languages) > 0 {
		p.Printf("%s %s\n", th.Bold.Render("languages:"), strings.Join(c.Languages, ", "))
	}
	p.Printf("%s %d\n", th.Bold.Render("configured rules:"), c.RuleCount)
	if c.UnknownImports != "" {
		p.Printf("%s %s\n", th.Bold.Render("unknown imports:"), c.UnknownImports)
	}
	if len(c.Modules) == 0 && c.Scope != "repository" {
		p.Println(th.Muted.Render("modules: none — the scope binds no declared module"))
	}
	for _, m := range c.Modules {
		p.Printf("\n%s %s", th.Bold.Render("module"), th.Bold.Render(m.Name))
		if m.Description != "" {
			p.Printf(" — %s", m.Description)
		}
		p.Printf("\n  paths: %s\n", strings.Join(m.Paths, ", "))
		if m.InternalRestricted {
			policy := strings.Join(m.Internal, ", ")
			if policy == "" {
				policy = "none (may import no other declared module)"
			}
			p.Printf("  internal imports: %s\n", policy)
		}
		if m.External != "allow" {
			p.Printf("  external imports: %s\n", m.External)
		}
		if m.Stdlib != "allow" {
			p.Printf("  stdlib imports: %s\n", m.Stdlib)
		}
	}
	if c.Domain != nil {
		writeDomainKnowledge(p, th, c.Domain)
	}
	if len(c.Kinds) > 0 {
		p.Println("\n" + th.Bold.Render("rule types in use:"))
		for _, k := range c.Kinds {
			p.Printf("  %s — %s\n", th.Bold.Render(k.Kind), k.Meaning)
		}
	}
	if len(c.Rules) > 0 {
		p.Println("\n" + th.Bold.Render("applicable rules:"))
		for _, r := range c.Rules {
			via := ""
			if len(r.Via) > 0 {
				via = " (via " + strings.Join(r.Via, ", ") + ")"
			}
			sev := th.severity(r.Summary.Severity).Render(r.Summary.Severity)
			meta := "[" + r.Summary.Type + "/" + sev + "]"
			p.Printf("  %s %s — %s%s\n", th.Bold.Render(r.Summary.ID), meta, r.Reason, via)
		}
	}
}

func writeDomainKnowledge(p *out.Printer, th Theme, d *application.DomainKnowledge) {
	counts := d.Counts
	p.Printf("\n%s (%s): %s", th.Bold.Render("project domain"), d.Source, domainCountPhrase(counts.Contexts, "context", "contexts"))
	p.Printf(", %s", domainCountPhrase(counts.Entities, "entity", "entities"))
	if counts.Aggregates > 0 {
		p.Printf(" (%s)", domainCountPhrase(counts.Aggregates, "aggregate", "aggregates"))
	}
	p.Printf(", %s, %s, %s, %s, %s\n",
		domainCountPhrase(counts.ValueObjects, "value object", "value objects"),
		domainCountPhrase(counts.Invariants, "invariant", "invariants"),
		domainCountPhrase(counts.Assertions, "assertion", "assertions"),
		domainCountPhrase(counts.Specifications, "specification", "specifications"),
		domainCountPhrase(counts.Events, "event", "events"),
	)
	for _, ctx := range d.Contexts {
		p.Printf("  %s %s:\n", th.Bold.Render("context"), th.Bold.Render(ctx.Name))
		if len(ctx.Entities) > 0 {
			names := make([]string, len(ctx.Entities))
			for i, e := range ctx.Entities {
				if e.Aggregate {
					names[i] = e.Name + " [aggregate]"
				} else {
					names[i] = e.Name
				}
			}
			p.Printf("    entities: %s\n", strings.Join(names, ", "))
		}
		if len(ctx.ValueObjects) > 0 {
			p.Printf("    value objects: %s\n", strings.Join(ctx.ValueObjects, ", "))
		}
		if len(ctx.Invariants) > 0 {
			p.Println("    invariants:")
			for _, inv := range ctx.Invariants {
				src := inv.Source
				if src == "" {
					src = contractSourceMissing
				}
				if inv.ID != "" {
					p.Printf("      %s (owner: %s, id: %s) %s\n", inv.Statement, inv.Owner, inv.ID, src)
					continue
				}
				p.Printf("      %s (owner: %s) %s\n", inv.Statement, inv.Owner, src)
			}
		}
		if len(ctx.Assertions) > 0 {
			p.Println("    assertions:")
			for _, a := range ctx.Assertions {
				src := a.Source
				if src == "" {
					src = contractSourceMissing
				}
				p.Printf("      %s (owner: %s, id: %s, on: %s) %s\n", a.Statement, a.Owner, a.ID, a.On, src)
			}
		}
		if len(ctx.Specifications) > 0 {
			p.Println("    specifications:")
			for _, s := range ctx.Specifications {
				src := s.Source
				if src == "" {
					src = contractSourceMissing
				}
				p.Printf("      %s %s\n", s.Name, src)
			}
		}
		if len(ctx.Events) > 0 {
			p.Printf("    events: %s\n", strings.Join(ctx.Events, ", "))
		}
	}
	for _, rel := range d.Relations {
		p.Printf("  relation: %s -[%s]-> %s\n", rel.From, rel.Kind, rel.To)
	}
}

func domainCountPhrase(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return itoa(n) + " " + plural
}
