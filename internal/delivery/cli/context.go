package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
)

// NewContextCommand adapts the architectural-context use case: the
// modules, rules, and reasons binding a location, for humans and
// agents alike.
func NewContextCommand(context application.GetArchitecturalContext) Command {
	return Command{
		Name:    "context",
		Short:   "modules, rules, and reasons binding a location",
		MaxArgs: 1,
		Flags: []Flag{
			{Name: "format", Default: formatHuman, Doc: "output format: human, json"},
		},
		Run: func(ctx Context) error {
			format := ctx.String("format")
			if format != formatHuman && format != formatJSON {
				return &ExitError{
					Code:    ExitConfigError,
					Message: fmt.Sprintf("unknown format %q (human, json)", format),
				}
			}
			scope := ""
			if len(ctx.Args) == 1 {
				scope = strings.TrimPrefix(ctx.Args[0], "./")
			}
			result, err := context.Execute(scope)
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

func writeContext(w io.Writer, c application.ArchitecturalContext) error {
	p := &printer{w: w}
	p.printf("scope: %s\n", c.Scope)
	if len(c.Languages) > 0 {
		p.printf("languages: %s\n", strings.Join(c.Languages, ", "))
	}
	p.printf("configured rules: %d\n", c.RuleCount)
	if len(c.Modules) == 0 && c.Scope != "repository" {
		p.println("modules: none — the path belongs to no declared module")
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
	if len(c.Rules) > 0 {
		p.println("\napplicable rules:")
		for _, r := range c.Rules {
			p.printf("  %s [%s/%s] — %s\n", r.Summary.ID, r.Summary.Type, r.Summary.Severity, r.Reason)
		}
	}
	return p.err
}
