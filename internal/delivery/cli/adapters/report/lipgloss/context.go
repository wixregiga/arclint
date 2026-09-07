package lipgloss

import (
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
)

func writeContext(p *out.Printer, th Theme, c application.ArchitecturalContext) {
	p.Printf("%s %s\n", th.Bold.Render("scope:"), c.Scope)
	for _, b := range c.Paths {
		owned := strings.Join(b.Zones, ", ")
		if owned == "" {
			owned = "no declared zone"
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
	if len(c.Zones) == 0 && c.Scope != "repository" {
		p.Println(th.Muted.Render("zones: none (the scope binds no declared zone)"))
	}
	for _, m := range c.Zones {
		p.Printf("\n%s %s", th.Bold.Render("zone"), th.Bold.Render(m.Name))
		if m.Description != "" {
			p.Printf(": %s", m.Description)
		}
		p.Printf("\n  paths: %s\n", strings.Join(m.Paths, ", "))
		if m.InternalRestricted {
			policy := strings.Join(m.Internal, ", ")
			if policy == "" {
				policy = "none (may import no other declared zone)"
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
			p.Printf("  %s: %s\n", th.Bold.Render(k.Kind), k.Meaning)
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
			p.Printf("  %s %s: %s%s\n", th.Bold.Render(r.Summary.ID), meta, r.Reason, via)
		}
	}
}
