package lipgloss

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

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
		p.Println(th.Muted.Render("modules: none (the scope binds no declared module)"))
	}
	for _, m := range c.Modules {
		p.Printf("\n%s %s", th.Bold.Render("module"), th.Bold.Render(m.Name))
		if m.Description != "" {
			p.Printf(": %s", m.Description)
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

// writeDomainKnowledge mirrors the plain renderer's domain block with
// the theme applied: headline, per-context terms with each contract's
// anchor, relations, then the unanchored contracts with their reasons.
func writeDomainKnowledge(p *out.Printer, th Theme, d *application.DomainKnowledge) {
	p.Printf("\n%s (%s): %s\n", th.Bold.Render("project domain"), d.Source, domainHeadline(d))
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
				if inv.ID != "" {
					p.Printf("      %s (owner: %s, id: %s)%s\n", inv.Statement, inv.Owner, inv.ID, th.anchorSuffix(inv.Source, inv.Anchor))
					continue
				}
				p.Printf("      %s (owner: %s)%s\n", inv.Statement, inv.Owner, th.anchorSuffix(inv.Source, inv.Anchor))
			}
		}
		if len(ctx.Assertions) > 0 {
			p.Println("    assertions:")
			for _, a := range ctx.Assertions {
				p.Printf("      %s (owner: %s, id: %s, on: %s)%s\n", a.Statement, a.Owner, a.ID, a.On, th.anchorSuffix(a.Source, a.Anchor))
			}
		}
		if len(ctx.Specifications) > 0 {
			p.Println("    specifications:")
			for _, s := range ctx.Specifications {
				p.Printf("      %s%s\n", s.Name, th.anchorSuffix(s.Source, s.Anchor))
			}
		}
		if len(ctx.Events) > 0 {
			p.Printf("    events: %s\n", strings.Join(ctx.Events, ", "))
		}
	}
	for _, rel := range d.Relations {
		p.Printf("  relation: %s -[%s]-> %s\n", rel.From, rel.Kind, rel.To)
	}
	writeUnanchored(p, th, d)
}

func domainHeadline(d *application.DomainKnowledge) string {
	if !d.Scoped {
		return countsPhrase(d.Counts)
	}
	if d.Shown.Contexts == 0 {
		return fmt.Sprintf("nothing recorded anchors into this scope; --full shows the whole model (%s)", countsPhrase(d.Counts))
	}
	return fmt.Sprintf("%s anchor into this scope; --full shows the whole model", shownPhrase(d))
}

func countsPhrase(c vocab.Counts) string {
	var b strings.Builder
	b.WriteString(domainCountPhrase(c.Contexts, "context", "contexts"))
	b.WriteString(", " + domainCountPhrase(c.Entities, "entity", "entities"))
	if c.Aggregates > 0 {
		fmt.Fprintf(&b, " (%s)", domainCountPhrase(c.Aggregates, "aggregate", "aggregates"))
	}
	fmt.Fprintf(&b, ", %s, %s, %s, %s, %s",
		domainCountPhrase(c.ValueObjects, "value object", "value objects"),
		domainCountPhrase(c.Invariants, "invariant", "invariants"),
		domainCountPhrase(c.Assertions, "assertion", "assertions"),
		domainCountPhrase(c.Specifications, "specification", "specifications"),
		domainCountPhrase(c.Events, "event", "events"))
	return b.String()
}

func shownPhrase(d *application.DomainKnowledge) string {
	s, c := d.Shown, d.Counts
	parts := []string{
		fmt.Sprintf("%d of %s", s.Contexts, domainCountPhrase(c.Contexts, "context", "contexts")),
		fmt.Sprintf("%d of %s", s.Entities, domainCountPhrase(c.Entities, "entity", "entities")),
		fmt.Sprintf("%d of %s", s.ValueObjects, domainCountPhrase(c.ValueObjects, "value object", "value objects")),
		fmt.Sprintf("%d of %s", s.Invariants, domainCountPhrase(c.Invariants, "invariant", "invariants")),
	}
	if c.Assertions > 0 {
		parts = append(parts, fmt.Sprintf("%d of %s", s.Assertions, domainCountPhrase(c.Assertions, "assertion", "assertions")))
	}
	if c.Specifications > 0 {
		parts = append(parts, fmt.Sprintf("%d of %s", s.Specifications, domainCountPhrase(c.Specifications, "specification", "specifications")))
	}
	if c.Events > 0 {
		parts = append(parts, fmt.Sprintf("%d of %s", s.Events, domainCountPhrase(c.Events, "event", "events")))
	}
	return strings.Join(parts, ", ")
}

// anchorSuffix renders a contract's anchor after its line: the source
// as a path when found, the anchor word in the severity color that
// matches its weight otherwise, nothing when the contracts were never
// located.
func (th Theme) anchorSuffix(source string, anchor application.ContractAnchor) string {
	switch anchor {
	case application.AnchorFound:
		return " " + th.Path.Render(source)
	case application.AnchorMissing, application.AnchorUnanchorable:
		return " " + th.anchorWord(anchor)
	}
	return ""
}

// writeUnanchored mirrors the plain renderer's grouped block with the
// anchor words colored by weight.
func writeUnanchored(p *out.Printer, th Theme, d *application.DomainKnowledge) {
	if len(d.Unanchored) == 0 {
		return
	}
	var unanchorable, missing int
	for _, u := range d.Unanchored {
		if u.Anchor == application.AnchorUnanchorable {
			unanchorable++
		} else {
			missing++
		}
	}
	p.Printf("  %s %s\n", th.Bold.Render("unanchored contracts:"), unanchoredPhrase(unanchorable, missing))
	for _, g := range groupUnanchored(d.Unanchored) {
		p.Printf("    %s: %s\n", th.anchorWord(g.first.Anchor), g.label())
		p.Printf("      %s\n", g.cause)
	}
	if unanchorable > 0 {
		p.Println(th.Muted.Render("    an unanchorable contract needs its recording changed before any source can carry it"))
	}
	if missing > 0 {
		p.Println(th.Muted.Render("    an invariants Rule on the owning Module reports each missing contract as a Violation"))
	}
}

func (th Theme) anchorWord(anchor application.ContractAnchor) string {
	if anchor == application.AnchorUnanchorable {
		return th.severity("error").Render(string(anchor))
	}
	return th.severity("warning").Render(string(anchor))
}

func unanchoredPhrase(unanchorable, missing int) string {
	var parts []string
	if unanchorable > 0 {
		parts = append(parts, fmt.Sprintf("%d unanchorable", unanchorable))
	}
	if missing > 0 {
		parts = append(parts, fmt.Sprintf("%d missing", missing))
	}
	return strings.Join(parts, ", ")
}

// unanchoredGroup is one line of the unanchored block: every contract
// sharing an anchor, a kind, a context, an owner, and a cause.
type unanchoredGroup struct {
	first application.UnanchoredContract
	cause string
	count int
}

func (g unanchoredGroup) label() string {
	u := g.first
	switch u.Kind {
	case application.ContractSpecification:
		return fmt.Sprintf("specification %s (context %s)", u.Name, u.Context)
	case application.ContractAssertion:
		return fmt.Sprintf("assertion %s on %s (context %s)", u.ID, u.Owner, u.Context)
	case application.ContractInvariant:
	}
	if u.ID != "" {
		return fmt.Sprintf("invariant %s on %s (context %s)", u.ID, u.Owner, u.Context)
	}
	return fmt.Sprintf("%s owned by %s (context %s)", domainCountPhrase(g.count, "invariant", "invariants"), u.Owner, u.Context)
}

func groupUnanchored(list []application.UnanchoredContract) []unanchoredGroup {
	var groups []unanchoredGroup
	index := map[string]int{}
	for _, u := range list {
		cause := u.Reason
		if u.Anchor == application.AnchorMissing {
			cause = missingExpectation(u)
		}
		key := strings.Join([]string{string(u.Anchor), string(u.Kind), u.Context, u.Owner, u.ID, u.Name, cause}, "\x00")
		if i, ok := index[key]; ok {
			groups[i].count++
			continue
		}
		index[key] = len(groups)
		groups = append(groups, unanchoredGroup{first: u, cause: cause, count: 1})
	}
	return groups
}

func missingExpectation(u application.UnanchoredContract) string {
	switch u.Kind {
	case application.ContractSpecification:
		return fmt.Sprintf("no SatisfiedBy method declared on %s", u.Name)
	case application.ContractAssertion:
		return fmt.Sprintf("no method %s declared on %s", u.ID, u.Owner)
	case application.ContractInvariant:
	}
	if u.ID != "" {
		return fmt.Sprintf("no method %s declared on %s", u.ID, u.Owner)
	}
	return fmt.Sprintf("no constructor declared for %s", u.Owner)
}

func domainCountPhrase(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return itoa(n) + " " + plural
}
