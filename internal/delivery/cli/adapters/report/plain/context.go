package plain

import (
	"fmt"
	"io"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
)

func writeContext(w io.Writer, c application.ArchitecturalContext) error {
	p := &out.Printer{W: w}
	p.Printf("scope: %s\n", c.Scope)
	for _, b := range c.Paths {
		owned := strings.Join(b.Modules, ", ")
		if owned == "" {
			owned = "no declared module"
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
	if len(c.Modules) == 0 && c.Scope != "repository" {
		p.Println("modules: none — the scope binds no declared module")
	}
	for _, m := range c.Modules {
		p.Printf("\nmodule %s", m.Name)
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
		writeDomainKnowledge(p, c.Domain)
	}
	if len(c.Kinds) > 0 {
		p.Println("\nrule types in use:")
		for _, k := range c.Kinds {
			p.Printf("  %s — %s\n", k.Kind, k.Meaning)
		}
	}
	if len(c.Rules) > 0 {
		p.Println("\napplicable rules:")
		for _, r := range c.Rules {
			via := ""
			if len(r.Via) > 0 {
				via = " (via " + strings.Join(r.Via, ", ") + ")"
			}
			p.Printf("  %s [%s/%s] — %s%s\n", r.Summary.ID, r.Summary.Type, r.Summary.Severity, r.Reason, via)
		}
	}
	return p.Err
}

// writeDomainKnowledge prints the project domain summary after the
// modules block: counts line, then per-context term groups and
// relations. Designated entities carry " [aggregate]" only in this
// text form.
func writeDomainKnowledge(p *out.Printer, d *application.DomainKnowledge) {
	counts := d.Counts
	p.Printf("\nproject domain (%s): %s", d.Source, domainCountPhrase(counts.Contexts, "context", "contexts"))
	p.Printf(", %s", domainCountPhrase(counts.Entities, "entity", "entities"))
	if counts.Aggregates > 0 {
		p.Printf(" (%s)", domainCountPhrase(counts.Aggregates, "aggregate", "aggregates"))
	}
	p.Printf(", %s, %s, %s\n",
		domainCountPhrase(counts.ValueObjects, "value object", "value objects"),
		domainCountPhrase(counts.Invariants, "invariant", "invariants"),
		domainCountPhrase(counts.Events, "event", "events"),
	)
	for _, ctx := range d.Contexts {
		p.Printf("  context %s:\n", ctx.Name)
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
			parts := make([]string, len(ctx.Invariants))
			for i, inv := range ctx.Invariants {
				parts[i] = fmt.Sprintf("%s (owner: %s)", inv.Statement, inv.Owner)
			}
			p.Printf("    invariants: %s\n", strings.Join(parts, "; "))
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
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}
