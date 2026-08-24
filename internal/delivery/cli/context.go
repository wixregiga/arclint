package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
)

// NewContextCommand adapts the architectural-context use case: with no
// scope it explains the repository — every Module, the Rule kinds in
// use, the enforcement posture; with paths or named Modules it is the
// worksite call, answering in one payload what governs the given set.
func NewContextCommand(context application.GetArchitecturalContext) Command {
	return Command{
		Name:    "context",
		Short:   "explain the architecture: the repository, or everything binding the given paths",
		MaxArgs: -1,
		Flags: []Flag{
			{
				Name:    flagFormat,
				Default: formatHuman,
				Doc:     "output format: human, json",
				Options: []string{formatHuman, formatJSON},
			},
			{
				Name:     "module",
				Doc:      "declared modules to include in the scope (comma or space separated)",
				Complete: completeModuleNames(context),
			},
		},
		Run: func(ctx Context) error {
			format := ctx.String(flagFormat)
			if format != formatHuman && format != formatJSON {
				return &ExitError{
					Code:    ExitConfigError,
					Message: fmt.Sprintf("unknown format %q (human, json)", format),
				}
			}
			paths := make([]string, 0, len(ctx.Args))
			for _, a := range ctx.Args {
				paths = append(paths, strings.TrimPrefix(a, "./"))
			}
			result, err := context.Execute(application.ContextRequest{
				Paths:   paths,
				Modules: splitSelectors(ctx.String("module")),
			})
			if err != nil {
				return ConfigError(err)
			}
			if format == formatJSON {
				data, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return ConfigError(err)
				}
				if _, err := fmt.Fprintln(ctx.Stdout, string(data)); err != nil {
					return fmt.Errorf("write output: %w", err)
				}
				return nil
			}
			if err := writeContext(ctx.Stdout, result); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}
}

// completeModuleNames completes declared Module names for the context
// --module flag, keeping typed comma segments as the inserted prefix.
// Per the completion contract a failing ruleset yields no candidates.
func completeModuleNames(context application.GetArchitecturalContext) func(toComplete string) []AutoCompleteCandidate {
	return func(toComplete string) []AutoCompleteCandidate {
		result, err := context.Execute(application.ContextRequest{})
		if err != nil {
			return nil
		}
		candidates := make([]AutoCompleteCandidate, 0, len(result.Modules))
		for _, m := range result.Modules {
			candidates = append(candidates, AutoCompleteCandidate{Value: m.Name, Doc: m.Description})
		}
		return withListPrefix(toComplete, candidates)
	}
}

func writeContext(w io.Writer, c application.ArchitecturalContext) error {
	p := &printer{w: w}
	p.printf("scope: %s\n", c.Scope)
	for _, b := range c.Paths {
		owned := strings.Join(b.Modules, ", ")
		if owned == "" {
			owned = "no declared module"
		}
		p.printf("  %s → %s\n", b.Path, owned)
	}
	if len(c.Languages) > 0 {
		p.printf("languages: %s\n", strings.Join(c.Languages, ", "))
	}
	p.printf("configured rules: %d\n", c.RuleCount)
	if c.UnknownImports != "" {
		p.printf("unknown imports: %s\n", c.UnknownImports)
	}
	if len(c.Modules) == 0 && c.Scope != "repository" {
		p.println("modules: none — the scope binds no declared module")
	}
	for _, m := range c.Modules {
		p.printf("\nmodule %s", m.Name)
		if m.Description != "" {
			p.printf(" — %s", m.Description)
		}
		p.printf("\n  paths: %s\n", strings.Join(m.Paths, ", "))
		if m.InternalRestricted {
			policy := strings.Join(m.Internal, ", ")
			if policy == "" {
				policy = "none (may import no other declared module)"
			}
			p.printf("  internal imports: %s\n", policy)
		}
		if m.External != "allow" {
			p.printf("  external imports: %s\n", m.External)
		}
		if m.Stdlib != "allow" {
			p.printf("  stdlib imports: %s\n", m.Stdlib)
		}
	}
	if c.Domain != nil {
		writeDomainKnowledge(p, c.Domain)
	}
	if len(c.Kinds) > 0 {
		p.println("\nrule types in use:")
		for _, k := range c.Kinds {
			p.printf("  %s — %s\n", k.Kind, k.Meaning)
		}
	}
	if len(c.Rules) > 0 {
		p.println("\napplicable rules:")
		for _, r := range c.Rules {
			via := ""
			if len(r.Via) > 0 {
				via = " (via " + strings.Join(r.Via, ", ") + ")"
			}
			p.printf("  %s [%s/%s] — %s%s\n", r.Summary.ID, r.Summary.Type, r.Summary.Severity, r.Reason, via)
		}
	}
	return p.err
}

// writeDomainKnowledge prints the project domain summary after the
// modules block: counts line, then per-context term groups and
// relations. Designated entities carry " [aggregate]" only in this
// text form.
func writeDomainKnowledge(p *printer, d *application.DomainKnowledge) {
	counts := d.Counts
	p.printf("\nproject domain (%s): %s", d.Source, domainCountPhrase(counts.Contexts, "context", "contexts"))
	p.printf(", %s", domainCountPhrase(counts.Entities, "entity", "entities"))
	if counts.Aggregates > 0 {
		p.printf(" (%s)", domainCountPhrase(counts.Aggregates, "aggregate", "aggregates"))
	}
	p.printf(", %s, %s, %s\n",
		domainCountPhrase(counts.ValueObjects, "value object", "value objects"),
		domainCountPhrase(counts.Invariants, "invariant", "invariants"),
		domainCountPhrase(counts.Events, "event", "events"),
	)
	for _, ctx := range d.Contexts {
		p.printf("  context %s:\n", ctx.Name)
		if len(ctx.Entities) > 0 {
			names := make([]string, len(ctx.Entities))
			for i, e := range ctx.Entities {
				if e.Aggregate {
					names[i] = e.Name + " [aggregate]"
				} else {
					names[i] = e.Name
				}
			}
			p.printf("    entities: %s\n", strings.Join(names, ", "))
		}
		if len(ctx.ValueObjects) > 0 {
			p.printf("    value objects: %s\n", strings.Join(ctx.ValueObjects, ", "))
		}
		if len(ctx.Invariants) > 0 {
			parts := make([]string, len(ctx.Invariants))
			for i, inv := range ctx.Invariants {
				parts[i] = fmt.Sprintf("%s (owner: %s)", inv.Statement, inv.Owner)
			}
			p.printf("    invariants: %s\n", strings.Join(parts, "; "))
		}
		if len(ctx.Events) > 0 {
			p.printf("    events: %s\n", strings.Join(ctx.Events, ", "))
		}
	}
	for _, rel := range d.Relations {
		p.printf("  relation: %s -[%s]-> %s\n", rel.From, rel.Kind, rel.To)
	}
}

func domainCountPhrase(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}
