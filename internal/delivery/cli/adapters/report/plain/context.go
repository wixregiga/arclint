package plain

import (
	"io"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/domaintext"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
)

func writeContext(w io.Writer, c application.ArchitecturalContext) error {
	p := &out.Printer{W: w}
	p.Printf("scope: %s\n", c.Scope)
	for _, b := range c.Paths {
		owned := strings.Join(b.Zones, ", ")
		if owned == "" {
			owned = "no declared zone"
		}
		p.Printf("  %s → %s\n", b.Path, owned)
	}
	if len(c.Languages) > 0 {
		p.Printf("languages: %s\n", strings.Join(c.Languages, ", "))
	}
	p.Printf("configured rules: %d\n", c.RuleCount)
	if c.UnknownImports != "" {
		p.Printf("unknown imports: %s\n", c.UnknownImports)
	}
	if len(c.Zones) == 0 && c.Scope != "repository" {
		p.Println("zones: none (the scope binds no declared zone)")
	}
	for _, m := range c.Zones {
		p.Printf("\nzone %s", m.Name)
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
		writeDomainKnowledge(p, c.Domain)
	}
	if len(c.Kinds) > 0 {
		p.Println("\nrule types in use:")
		for _, k := range c.Kinds {
			p.Printf("  %s: %s\n", k.Kind, k.Meaning)
		}
	}
	if len(c.Rules) > 0 {
		p.Println("\napplicable rules:")
		for _, r := range c.Rules {
			via := ""
			if len(r.Via) > 0 {
				via = " (via " + strings.Join(r.Via, ", ") + ")"
			}
			p.Printf("  %s [%s/%s]: %s%s\n", r.Summary.ID, r.Summary.Type, r.Summary.Severity, r.Reason, via)
		}
	}
	return p.Err
}

// writeDomainKnowledge prints the project domain block after the
// zones block, in the wording shared with the Lipgloss renderer.
func writeDomainKnowledge(p *out.Printer, d *application.DomainKnowledge) {
	domaintext.Knowledge(p, domaintext.Plain(), d)
}
