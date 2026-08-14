package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/engine"
	"github.com/wixregiga/arclint/internal/lang"
)

// newFactsCmd dumps declaration facts for one file: the same view rules
// get from ctx.facts(path), without writing a probe rule for it.
func newFactsCmd() *cobra.Command {
	var rulesFlag, format string
	cmd := &cobra.Command{
		Use:   "facts <file>",
		Short: "declaration facts for one file: kinds, names, owners, signatures",
		Long: "Prints the declaration facts a rule sees from ctx.facts(path): kinds,\n" +
			"names, owners, visibility, line spans, and syntactic signatures on\n" +
			"func/method decls. Use --format json for the exact wire shape.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "human" && format != "json" {
				return &exitError{2, fmt.Sprintf("unknown format %q (human, json)", format)}
			}
			path, err := resolveRules(rulesFlag, args[0])
			if err != nil {
				return &exitError{2, err.Error()}
			}
			rs, _, err := config.LoadCached(path, version)
			if err != nil {
				return &exitError{2, err.Error()}
			}
			abs, err := filepath.Abs(args[0])
			if err != nil {
				return &exitError{2, err.Error()}
			}
			rel, err := filepath.Rel(rs.Root, abs)
			if err != nil || strings.HasPrefix(rel, "..") {
				return &exitError{2, fmt.Sprintf("%s is outside the repo root %s", args[0], rs.Root)}
			}
			facts, err := engine.FileFactsFor(rs, filepath.ToSlash(rel))
			if err != nil {
				return &exitError{2, err.Error()}
			}

			out := cmd.OutOrStdout()
			if format == "json" {
				data, err := json.MarshalIndent(facts, "", "  ")
				if err != nil {
					return &exitError{2, err.Error()}
				}
				fmt.Fprintln(out, string(data))
				return nil
			}
			fmt.Fprintf(out, "%s\n", facts.Path)
			if facts.Package != "" {
				fmt.Fprintf(out, "package: %s\n", facts.Package)
			}
			if facts.ParseError != "" {
				fmt.Fprintf(out, "parse error: %s\n", facts.ParseError)
				return nil
			}
			if len(facts.Decls) == 0 {
				fmt.Fprintln(out, "no declarations")
				return nil
			}
			w := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
			fmt.Fprintln(w, "KIND\tNAME\tOWNER\tEXPORTED\tLINES\tSIGNATURE")
			for _, d := range facts.Decls {
				owner := d.Owner
				if owner == "" {
					owner = "-"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%v\t%d-%d\t%s\n",
					d.Kind, d.Name, owner, d.Exported, d.StartLine, d.EndLine, renderSignature(d))
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&rulesFlag, "rules", "", "path to rules.yaml (default: discovered upward from <file>)")
	cmd.Flags().StringVar(&format, "format", "human", "output format: human or json")
	return cmd
}

// renderSignature prints one func/method signature in a neutral human
// form: (name type, opt? type, ...rest type) -> (A, B). The JSON output
// is the precise shape; this is for eyes.
func renderSignature(d lang.Decl) string {
	if d.Kind != "func" && d.Kind != "method" {
		return ""
	}
	parts := make([]string, 0, len(d.Params))
	for _, p := range d.Params {
		s := p.Name
		if p.Variadic && !strings.HasPrefix(s, "*") {
			s = "..." + s
		}
		if p.Optional {
			s += "?"
		}
		if p.Type != "" {
			if s != "" {
				s += " "
			}
			s += p.Type
		}
		parts = append(parts, s)
	}
	sig := "(" + strings.Join(parts, ", ") + ")"
	switch len(d.Results) {
	case 0:
	case 1:
		sig += " -> " + d.Results[0]
	default:
		sig += " -> (" + strings.Join(d.Results, ", ") + ")"
	}
	return sig
}
