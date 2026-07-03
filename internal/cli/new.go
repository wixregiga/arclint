package cli

import (
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jofyi/arclint/internal/answers"
	"github.com/jofyi/arclint/internal/config"
	"github.com/jofyi/arclint/internal/prompt"
	"github.com/jofyi/arclint/internal/template"
)

func init() { Register(newNewCmd()) }

func newNewCmd() *cobra.Command {
	var varFlags []string
	var outDir string
	var dryRun, list bool

	cmd := &cobra.Command{
		Use:   "new <thing> [name]",
		Short: "Generate a new unit (service, package, docs page — any thing) from a template",
		Long: "Generate a new unit from a template in .arclint/templates/.\n" +
			"Values resolve as: --var flags, then manifest defaults; a prompt fires only\n" +
			"for a required variable (no default) left unresolved. Regenerating an\n" +
			"existing unit is `arclint make`'s job — new has no --force.",
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNew(cmd, args, varFlags, outDir, dryRun, list)
		},
	}
	f := cmd.Flags()
	f.StringArrayVar(&varFlags, "var", nil, "set a template variable name=value (repeatable)")
	f.StringVar(&outDir, "out", "", "override the manifest's destination (relative to the repo root)")
	f.BoolVar(&dryRun, "dry-run", false, "render and show the file list, write nothing")
	f.BoolVar(&list, "list", false, "list available things with their descriptions, then exit")
	return cmd
}

func runNew(cmd *cobra.Command, args []string, varFlags []string, outDir string, dryRun, list bool) error {
	stdout := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	root, err := repoRootFromGlobals()
	if err != nil {
		return err
	}
	things, err := template.Discover(root)
	if err != nil {
		return UsageErrorf("%s", err)
	}

	if list {
		if len(things) == 0 {
			fmt.Fprintln(stdout, "no templates found — drop a directory with a template.yaml into .arclint/templates/")
			return nil
		}
		for _, name := range things {
			tpl, err := template.Load(root, name)
			if err != nil {
				fmt.Fprintf(stdout, "%s — (invalid template.yaml: %s)\n", name, err)
				continue
			}
			desc := tpl.Manifest.Description
			if desc == "" {
				desc = "(no description)"
			}
			fmt.Fprintf(stdout, "%s — %s\n", name, desc)
		}
		return nil
	}

	if len(args) == 0 {
		return UsageErrorf("missing <thing> — available: %s", strings.Join(things, ", "))
	}
	thing := args[0]
	if !slices.Contains(things, thing) {
		msg := fmt.Sprintf("unknown template %q — available: %s", thing, strings.Join(things, ", "))
		if best := closest(thing, things); best != "" {
			msg += fmt.Sprintf(" (did you mean %q?)", best)
		}
		return UsageErrorf("%s", msg)
	}
	tpl, err := template.Load(root, thing)
	if err != nil {
		return UsageErrorf("%s", err)
	}

	flagVals, err := parseVarFlags(varFlags, tpl.Manifest)
	if err != nil {
		return err
	}
	if len(args) == 2 {
		if !manifestHasVar(tpl.Manifest, "name") {
			return UsageErrorf("template %q declares no \"name\" variable — drop the positional name argument", thing)
		}
		if prev, ok := flagVals["name"]; ok && prev != args[1] {
			return UsageErrorf("positional name %q conflicts with --var name=%s — pass one of them", args[1], prev)
		}
		flagVals["name"] = args[1]
	}

	builtins := template.Builtins(root)
	res, prompted, err := resolveWithPrompts(tpl.Manifest, flagVals, nil, builtins, stderr)
	if err != nil {
		return err
	}

	if len(prompted) > 0 && !Globals().Quiet {
		prompt.Tip(stderr, equivalentCommand(tpl, res))
	}

	renderVars := res.RenderVars(builtins)
	dest := outDir
	if dest == "" {
		dest, err = tpl.Destination(renderVars)
		if err != nil {
			return UsageErrorf("%s", err)
		}
	} else {
		dest = path.Clean(filepath.ToSlash(dest))
		// Segment-based validation (reuse the template guard) so a legit dir
		// like "..cache" is accepted while a real ".." escape is rejected.
		if err := template.ValidateRelPath(dest); err != nil {
			return UsageErrorf("--out %q must be a relative path inside the repo — pass a path like services/my-thing", outDir)
		}
	}

	// One destination, one unit, one template: refuse a destination claimed
	// by a different template's recorded unit (docs/design/templating.md §3).
	units, err := answers.List(root)
	if err != nil {
		return UsageErrorf("%s", err)
	}
	for _, u := range units {
		if u.Destination == dest && u.Template != tpl.Name {
			return UsageErrorf("destination %q is already claimed by template %q — template %q cannot generate there; pick a different destination or delete the existing unit first", dest, u.Template, tpl.Name)
		}
	}

	destAbs := filepath.Join(root, filepath.FromSlash(dest))
	if _, err := os.Stat(destAbs); err == nil {
		return UsageErrorf("destination %s already exists — regenerate an existing unit with `arclint make %s`", dest, dest)
	}

	files, err := tpl.RenderUnit(renderVars)
	if err != nil {
		return UsageErrorf("%s", err)
	}
	paths := slices.Sorted(maps.Keys(files))

	if dryRun {
		for _, p := range paths {
			fmt.Fprintf(stdout, "would create %s\n", dest+"/"+p)
		}
		if !Globals().Quiet {
			fmt.Fprintf(stdout, "dry-run: %d files, nothing written\n", len(paths))
		}
		return nil
	}

	// All rendering succeeded; write atomically. Render the whole tree into a
	// temp dir beside the final destination, then os.Rename it into place. A
	// failure partway through leaves only the temp dir (removed here), never a
	// half-written unit that would block a re-run. The refusal checks above
	// already ran, so nothing claims dest yet.
	destParent := filepath.Dir(destAbs)
	if err := os.MkdirAll(destParent, 0o755); err != nil {
		return UsageErrorf("cannot create %s — %v", destParent, err)
	}
	tmpDir, err := os.MkdirTemp(destParent, ".arclint-new-*")
	if err != nil {
		return UsageErrorf("cannot create a temp dir under %s — %v", destParent, err)
	}
	writeErr := func() error {
		for _, p := range paths {
			abs := filepath.Join(tmpDir, filepath.FromSlash(p))
			if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
				return fmt.Errorf("cannot create %s — %v", filepath.Dir(abs), err)
			}
			if err := os.WriteFile(abs, files[p], 0o644); err != nil {
				return fmt.Errorf("cannot write %s — %v", abs, err)
			}
		}
		return nil
	}()
	if writeErr != nil {
		os.RemoveAll(tmpDir)
		return UsageErrorf("%s", writeErr)
	}
	if err := os.Rename(tmpDir, destAbs); err != nil {
		os.RemoveAll(tmpDir)
		return UsageErrorf("cannot move rendered unit into %s — %v", dest, err)
	}

	// Generation is itself the apply step: record answers immediately.
	unit := &answers.Unit{
		Version:         answers.CurrentVersion,
		Template:        tpl.Name,
		TemplateVersion: tpl.Manifest.Version,
		Destination:     dest,
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
		Answers:         res.Values,
		Files:           hashRendered(files),
	}
	if err := answers.Save(root, unit); err != nil {
		return UsageErrorf("%s", err)
	}

	if !Globals().Quiet {
		for _, p := range paths {
			fmt.Fprintf(stdout, "created %s\n", dest+"/"+p)
		}
		fmt.Fprintf(stdout, "recorded %s\n", ".arclint/answers/"+answers.Slug(dest)+".yaml")
	}
	return nil
}

// resolveWithPrompts runs the resolve loop shared by new and make: resolve,
// prompt for missing required variables when interactive, merge, re-resolve
// (a prompted value can activate a later when). Under --no-input or a
// non-TTY, unresolved required variables are exit-2 errors, one line each.
func resolveWithPrompts(m *template.Manifest, flagVals, saved, builtins map[string]string, stderr io.Writer) (*template.Resolution, map[string]string, error) {
	prompted := map[string]string{}
	for range m.Variables {
		res, err := m.Resolve(template.ResolveInput{Flags: flagVals, Saved: saved, Builtins: builtins})
		if err != nil {
			return nil, nil, UsageErrorf("%s", err)
		}
		if len(res.Missing) == 0 {
			return res, prompted, nil
		}
		if Globals().NoInput || !prompt.Interactive() {
			for _, v := range res.Missing {
				fmt.Fprintf(stderr, "error: missing required input %q — pass --var %s=<value> or run without --no-input\n", v.Name, v.Name)
			}
			return nil, nil, &ExitError{Code: ExitUsage}
		}
		got, err := prompt.Ask(res.Missing)
		if err != nil {
			return nil, nil, UsageErrorf("prompt aborted — %v; pass values with --var instead", err)
		}
		for k, v := range got {
			flagVals[k] = v
			prompted[k] = v
		}
	}
	res, err := m.Resolve(template.ResolveInput{Flags: flagVals, Saved: saved, Builtins: builtins})
	if err != nil {
		return nil, nil, UsageErrorf("%s", err)
	}
	if len(res.Missing) > 0 {
		return nil, nil, UsageErrorf("could not resolve all required variables — pass them with --var")
	}
	return res, prompted, nil
}

// equivalentCommand builds the full non-interactive command line for the tip:
// positional name (when the template declares one) plus --var for everything
// else, in manifest declaration order.
func equivalentCommand(tpl *template.Template, res *template.Resolution) string {
	var b strings.Builder
	b.WriteString("arclint new " + tpl.Name)
	if v, ok := res.Values["name"]; ok && manifestHasVar(tpl.Manifest, "name") {
		b.WriteString(" " + prompt.ShellQuote(v))
	}
	for i := range tpl.Manifest.Variables {
		name := tpl.Manifest.Variables[i].Name
		if name == "name" {
			continue
		}
		if v, ok := res.Values[name]; ok {
			b.WriteString(" --var " + name + "=" + prompt.ShellQuote(v))
		}
	}
	return b.String()
}

func manifestHasVar(m *template.Manifest, name string) bool {
	for i := range m.Variables {
		if m.Variables[i].Name == name {
			return true
		}
	}
	return false
}

// parseVarFlags parses repeated --var name=value flags and rejects names the
// manifest does not declare (with a closest-match suggestion).
func parseVarFlags(varFlags []string, m *template.Manifest) (map[string]string, error) {
	out := map[string]string{}
	known := make([]string, 0, len(m.Variables))
	for i := range m.Variables {
		known = append(known, m.Variables[i].Name)
	}
	for _, kv := range varFlags {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return nil, UsageErrorf("invalid --var %q — the form is --var name=value", kv)
		}
		if !slices.Contains(known, k) {
			msg := fmt.Sprintf("unknown variable %q — this template declares: %s", k, strings.Join(known, ", "))
			if best := closest(k, known); best != "" {
				msg += fmt.Sprintf(" (did you mean %q?)", best)
			}
			return nil, UsageErrorf("%s", msg)
		}
		out[k] = v
	}
	return out, nil
}

// repoRootFromGlobals resolves the repo root: --config when given (either the
// repo root or its .arclint directory), else upward discovery from cwd.
func repoRootFromGlobals() (string, error) {
	g := Globals()
	if g.Config != "" {
		abs, err := filepath.Abs(g.Config)
		if err != nil {
			return "", UsageErrorf("cannot resolve --config %q — %v", g.Config, err)
		}
		if filepath.Base(abs) == ".arclint" {
			abs = filepath.Dir(abs)
		}
		if info, err := os.Stat(filepath.Join(abs, ".arclint")); err != nil || !info.IsDir() {
			return "", UsageErrorf("--config %q has no .arclint/ directory — pass the repo root or the .arclint path", g.Config)
		}
		return abs, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", UsageErrorf("cannot determine working directory — %v", err)
	}
	root, err := config.FindConfigRoot(cwd)
	if err != nil {
		return "", &ExitError{Code: ExitUsage, Err: err}
	}
	return root, nil
}

// hashRendered records the per-file baseline. It hashes CRLF-normalized bytes
// so the baseline matches `arclint make`'s EOL-agnostic comparison — a file
// later checked out with CRLF endings does not spuriously read as drift.
func hashRendered(files map[string][]byte) map[string]string {
	out := make(map[string]string, len(files))
	for p, data := range files {
		out[p] = hashNormalized(data)
	}
	return out
}
