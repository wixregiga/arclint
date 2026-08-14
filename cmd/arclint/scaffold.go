package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var scaffoldNameRE = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// newRulesScaffoldCmd writes the three pieces a new rule needs: a stub
// extension, a failing test case, and the rules.yaml snippet to arm it.
// Both field sessions asked for this independently.
func newRulesScaffoldCmd() *cobra.Command {
	var rulesFlag string
	cmd := &cobra.Command{
		Use:   "scaffold <rule-type>",
		Short: "stub extension + failing test case + rules.yaml snippet for a new rule",
		Long: "Writes .arclint/extensions/<type>.ts (a defineRule stub) and\n" +
			".arclint/tests/<type>.yaml (a case that FAILS until the rule reports),\n" +
			"then prints the rules.yaml snippet that arms the rule. Red first:\n" +
			"implement check() until `arclint rules test` passes.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !scaffoldNameRE.MatchString(name) {
				return &exitError{2, fmt.Sprintf("rule type %q must be kebab-case: lowercase letters, digits, dashes", name)}
			}
			path, err := resolveRules(rulesFlag, ".")
			if err != nil {
				return &exitError{2, err.Error() + " (run `arclint init` first: the rules.yaml directory is the extension root)"}
			}
			root := filepath.Dir(path)
			extPath := filepath.Join(root, ".arclint", "extensions", name+".ts")
			testPath := filepath.Join(root, ".arclint", "tests", name+".yaml")
			for _, p := range []string{extPath, testPath} {
				if _, err := os.Stat(p); err == nil {
					return &exitError{2, fmt.Sprintf("refusing to overwrite %s", p)}
				}
			}
			for _, p := range []string{extPath, testPath} {
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					return &exitError{2, err.Error()}
				}
			}
			if err := os.WriteFile(extPath, []byte(scaffoldExtension(name)), 0o644); err != nil {
				return &exitError{2, err.Error()}
			}
			if err := os.WriteFile(testPath, []byte(scaffoldCase(name)), 0o644); err != nil {
				return &exitError{2, err.Error()}
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "wrote %s\n", extPath)
			fmt.Fprintf(out, "wrote %s\n", testPath)
			fmt.Fprintf(out, "\narm the rule in %s:\n\n", path)
			fmt.Fprintf(out, "  rules:\n    - type: %s\n      id: %s\n\n", name, name)
			fmt.Fprintln(out, "then iterate: `arclint rules test` runs the scaffolded case, which")
			fmt.Fprintln(out, "fails until check() reports the expected finding.")
			return nil
		},
	}
	cmd.Flags().StringVar(&rulesFlag, "rules", "", "path to rules.yaml (default: discovered upward from .)")
	return cmd
}

func scaffoldExtension(name string) string {
	return strings.ReplaceAll(`import { defineRule, s } from "arclint";

export default defineRule({
  type: "__NAME__",
  description: "TODO: one line: what this rule enforces.",
  // The host validates params against this schema before check runs.
  params: s.object({
    // paths: s.array(s.string()).describe("globs this rule inspects"),
  }),
  check(ctx, params) {
    // ctx surface: files(glob?), read(path), imports(path), modules(),
    // facts(path) (decls with signatures), moduleOf(path), report(v).
    // TODO: find violations and report them, for example:
    // for (const f of ctx.files("src/**")) {
    //   ctx.report({ path: f.path, message: "TODO: why this violates the rule" });
    // }
  },
});
`, "__NAME__", name)
}

func scaffoldCase(name string) string {
	return strings.ReplaceAll(`# Scaffolded by arclint rules scaffold. This case FAILS until the rule
# reports the expected finding: implement check() in
# .arclint/extensions/__NAME__.ts, shape the files below into a minimal
# violating tree, then run arclint rules test.
case: __NAME__-reports
files:
  example/violating.txt: |
    TODO: replace with the smallest tree that violates the rule
expect:
  - rule: "__NAME__"
    path: example/violating.txt
`, "__NAME__", name)
}
