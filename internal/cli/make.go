package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

	"github.com/wixregiga/arclint/internal/answers"
	"github.com/wixregiga/arclint/internal/template"
)

func init() { Register(newMakeCmd()) }

func newMakeCmd() *cobra.Command {
	var apply, showDiff, failOnDrift, takeTemplate bool
	var varFlags []string

	cmd := &cobra.Command{
		Use:   "make [paths...]",
		Short: "Re-render generated units against saved answers and report or apply drift",
		Long: "Re-render every recorded unit (or the units under the given paths) to\n" +
			"memory and compare against disk. The default is a dry-run report and always\n" +
			"exits 0; --fail-on-drift makes drift exit 1 for CI. --apply writes clean\n" +
			"updates; a file you edited that the template also changed is a conflict and\n" +
			"is skipped unless --apply --take-template overwrites your version.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMake(cmd, args, varFlags, apply, showDiff, failOnDrift, takeTemplate)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&apply, "apply", false, "write the re-rendered output (default is a dry-run diff summary)")
	f.BoolVar(&showDiff, "diff", false, "show a unified diff per drifted file")
	f.StringArrayVar(&varFlags, "var", nil, "override a saved answer name=value for this run (persisted only with --apply)")
	f.BoolVar(&failOnDrift, "fail-on-drift", false, "exit 1 when drift is found (for CI)")
	f.BoolVar(&takeTemplate, "take-template", false, "with --apply, overwrite conflicted files with the template version (destructive)")
	return cmd
}

// fileReport is one file's drift record inside a unit.
type fileReport struct {
	rel        string // path under the unit destination, slash-separated
	status     string // added | changed | conflict
	userEdited bool   // disk hash no longer matches the recorded baseline (changed/conflict only)
	disk       []byte // nil when added
	rendered   []byte
}

// unitReport is one unit's drift record.
type unitReport struct {
	unit     *answers.Unit
	tpl      *template.Template // nil for orphans
	status   string             // clean | drift | conflict | orphan
	outdated bool
	resolved map[string]string // post-resolution answers (for shard update)
	rendered map[string][]byte // full render (for hash refresh on apply)
	files    []fileReport
}

func runMake(cmd *cobra.Command, args []string, varFlags []string, apply, showDiff, failOnDrift, takeTemplate bool) error {
	stdout := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	if takeTemplate && !apply {
		return UsageErrorf("--take-template only makes sense with --apply — add --apply, or drop --take-template")
	}

	root, err := repoRootFromGlobals()
	if err != nil {
		return err
	}
	allUnits, err := answers.List(root)
	if err != nil {
		return UsageErrorf("%s", err)
	}

	units, err := selectUnits(allUnits, args)
	if err != nil {
		return err
	}
	if len(units) == 0 {
		if Globals().Format == FormatJSON {
			fmt.Fprintln(stdout, `{"units":[]}`)
		} else if !Globals().Quiet {
			fmt.Fprintln(stdout, "no generated units recorded in .arclint/answers/ — create one with `arclint new`")
		}
		return nil
	}

	rawOverrides, err := parseRawVars(varFlags)
	if err != nil {
		return err
	}
	overrideUsed := map[string]bool{}

	reports := make([]*unitReport, 0, len(units))
	for _, u := range units {
		rep, err := evaluateUnit(root, u, rawOverrides, overrideUsed, stderr)
		if err != nil {
			return err
		}
		reports = append(reports, rep)
	}
	for k := range rawOverrides {
		if !overrideUsed[k] {
			return UsageErrorf("unknown variable %q — no targeted unit's template declares it", k)
		}
	}

	if Globals().Format == FormatJSON {
		if err := printMakeJSON(stdout, reports); err != nil {
			return err
		}
	} else {
		printMakeText(stdout, reports, showDiff, apply)
	}

	if apply {
		if err := applyReports(root, reports, takeTemplate, rawOverrides, stdout); err != nil {
			return err
		}
	}

	if failOnDrift {
		for _, r := range reports {
			if r.status != "clean" {
				return Findings()
			}
		}
	}
	return nil
}

// selectUnits filters recorded units by the given paths; a parent path
// selects all units beneath it. A path matching nothing is an exit-2 error.
func selectUnits(all []*answers.Unit, args []string) ([]*answers.Unit, error) {
	if len(args) == 0 {
		return all, nil
	}
	seen := map[string]bool{}
	var out []*answers.Unit
	for _, arg := range args {
		p := strings.Trim(path.Clean(filepath.ToSlash(arg)), "/")
		matched := false
		for _, u := range all {
			if u.Destination == p || strings.HasPrefix(u.Destination, p+"/") {
				matched = true
				if !seen[u.Destination] {
					seen[u.Destination] = true
					out = append(out, u)
				}
			}
		}
		if !matched {
			return nil, UsageErrorf("path %q has no recorded unit in .arclint/answers/ — generate it first with `arclint new`", arg)
		}
	}
	slices.SortFunc(out, func(a, b *answers.Unit) int { return strings.Compare(a.Destination, b.Destination) })
	return out, nil
}

// parseRawVars parses --var overrides without a manifest (make spans units
// with different templates; per-unit filtering happens in evaluateUnit).
func parseRawVars(varFlags []string) (map[string]string, error) {
	out := map[string]string{}
	for _, kv := range varFlags {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return nil, UsageErrorf("invalid --var %q — the form is --var name=value", kv)
		}
		out[k] = v
	}
	return out, nil
}

// evaluateUnit re-renders one unit to memory and classifies every file.
// The recorded per-file hashes are the baseline: disk differing from the
// baseline means the user edited; the render differing from the baseline
// means the template changed; both at once is a conflict
// (docs/design/templating.md §4, conflict policy).
func evaluateUnit(root string, u *answers.Unit, overrides map[string]string, overrideUsed map[string]bool, stderr io.Writer) (*unitReport, error) {
	rep := &unitReport{unit: u, status: "clean"}

	destAbs := filepath.Join(root, filepath.FromSlash(u.Destination))
	tpl, err := template.Load(root, u.Template)
	if err != nil {
		if _, statErr := os.Stat(filepath.Join(template.TemplatesDir(root), u.Template, "template.yaml")); os.IsNotExist(statErr) {
			rep.status = "orphan"
			return rep, nil
		}
		return nil, UsageErrorf("%s", err) // template exists but is invalid
	}
	rep.tpl = tpl
	if info, err := os.Stat(destAbs); err != nil || !info.IsDir() {
		rep.status = "orphan"
		return rep, nil
	}

	flagVals := map[string]string{}
	for i := range tpl.Manifest.Variables {
		name := tpl.Manifest.Variables[i].Name
		if v, ok := overrides[name]; ok {
			flagVals[name] = v
			overrideUsed[name] = true
		}
	}
	res, _, err := resolveWithPrompts(tpl.Manifest, flagVals, u.Answers, template.Builtins(root), stderr)
	if err != nil {
		return nil, err
	}
	rep.resolved = res.Values

	rendered, err := tpl.RenderUnit(res.RenderVars(template.Builtins(root)))
	if err != nil {
		return nil, UsageErrorf("%s", err)
	}
	rep.rendered = rendered
	rep.outdated = u.TemplateVersion != tpl.Manifest.Version

	for _, p := range slices.Sorted(maps.Keys(rendered)) {
		want := rendered[p]
		disk, err := os.ReadFile(filepath.Join(destAbs, filepath.FromSlash(p)))
		if err != nil {
			if os.IsNotExist(err) {
				rep.files = append(rep.files, fileReport{rel: p, status: "added", rendered: want})
				continue
			}
			return nil, UsageErrorf("cannot read %s — %v", p, err)
		}
		// Compare and hash on CRLF-normalized bytes only, so a file checked
		// out with CRLF line endings does not read as drift against LF-rendered
		// output (and vice versa). The bytes written on --apply are untouched
		// (applyReports writes f.rendered verbatim) — normalization is a
		// comparison concern, never a mutation.
		if string(normalizeEOL(disk)) == string(normalizeEOL(want)) {
			continue
		}
		status := "changed"
		edited := false
		if baseline, ok := u.Files[p]; ok {
			edited = hashNormalized(disk) != baseline
			templateChanged := hashNormalized(want) != baseline
			if edited && templateChanged {
				status = "conflict"
			}
		}
		rep.files = append(rep.files, fileReport{rel: p, status: status, userEdited: edited, disk: disk, rendered: want})
	}

	for _, f := range rep.files {
		if f.status == "conflict" {
			rep.status = "conflict"
			break
		}
		rep.status = "drift"
	}
	return rep, nil
}

// plural picks "word" or "word+s" for a count, so output reads "1 file" not
// "1 files".
func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func printMakeText(w io.Writer, reports []*unitReport, showDiff, apply bool) {
	quiet := Globals().Quiet
	statusWord := map[string]string{"added": "created", "changed": "modified", "conflict": "conflict"}
	dirty := 0
	filesAffected := 0
	for _, r := range reports {
		if r.status == "clean" {
			continue
		}
		dirty++
		if r.status == "orphan" {
			fmt.Fprintf(w, "orphan: %s (template: %s — template or destination missing)\n", r.unit.Destination, r.unit.Template)
			continue
		}
		note := ""
		if r.outdated {
			note = fmt.Sprintf(", outdated: unit has v%d, template is v%d", r.unit.TemplateVersion, r.tpl.Manifest.Version)
		}
		fmt.Fprintf(w, "%s: %s (template: %s, %s%s)\n", r.status, r.unit.Destination, r.unit.Template, plural(len(r.files), "file"), note)
		for _, f := range r.files {
			full := r.unit.Destination + "/" + f.rel
			fmt.Fprintf(w, "  %-9s %s\n", statusWord[f.status], full)
			filesAffected++
		}
		if showDiff {
			for _, f := range r.files {
				if d := template.UnifiedDiff(r.unit.Destination+"/"+f.rel, f.disk, f.rendered); d != "" {
					fmt.Fprint(w, d)
				}
			}
		}
	}
	if quiet {
		return
	}
	if dirty == 0 {
		fmt.Fprintf(w, "all %s clean\n", plural(len(reports), "unit"))
		return
	}
	summary := fmt.Sprintf("%d of %s drifted, %s affected.", dirty, plural(len(reports), "unit"), plural(filesAffected, "file"))
	if !apply {
		summary += " run with --apply to write."
	}
	fmt.Fprintln(w, summary)
}

func printMakeJSON(w io.Writer, reports []*unitReport) error {
	type jsonFile struct {
		Path   string `json:"path"`
		Status string `json:"status"`
	}
	type jsonUnit struct {
		Unit     string     `json:"unit"`
		Template string     `json:"template"`
		Status   string     `json:"status"`
		Files    []jsonFile `json:"files"`
	}
	out := struct {
		Units []jsonUnit `json:"units"`
	}{Units: []jsonUnit{}}
	for _, r := range reports {
		ju := jsonUnit{
			Unit:     r.unit.Destination,
			Template: r.unit.Template,
			Status:   r.status,
			Files:    []jsonFile{},
		}
		for _, f := range r.files {
			ju.Files = append(ju.Files, jsonFile{Path: r.unit.Destination + "/" + f.rel, Status: f.status})
		}
		out.Units = append(out.Units, ju)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// applyReports writes the re-rendered output: clean updates and additions are
// written; conflicted files are skipped unless takeTemplate. The shard is
// updated (template_version, answers, hashes) for every touched or outdated
// unit; a skipped conflict keeps its old baseline hash so it stays a conflict.
func applyReports(root string, reports []*unitReport, takeTemplate bool, overrides map[string]string, stdout io.Writer) error {
	quiet := Globals().Quiet
	jsonOut := Globals().Format == FormatJSON
	for _, r := range reports {
		if r.status == "orphan" {
			if !quiet && !jsonOut {
				fmt.Fprintf(stdout, "skipped orphan %s — restore the template and destination, or delete %s\n",
					r.unit.Destination, ".arclint/answers/"+answers.Slug(r.unit.Destination)+".yaml")
			}
			continue
		}
		wrote := false
		skipped := []string{}
		destAbs := filepath.Join(root, filepath.FromSlash(r.unit.Destination))
		for _, f := range r.files {
			if f.status == "conflict" && !takeTemplate {
				skipped = append(skipped, f.rel)
				continue
			}
			abs := filepath.Join(destAbs, filepath.FromSlash(f.rel))
			if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
				return UsageErrorf("cannot create %s — %v", filepath.Dir(abs), err)
			}
			if err := os.WriteFile(abs, f.rendered, 0o644); err != nil {
				return UsageErrorf("cannot write %s — %v", abs, err)
			}
			wrote = true
			if !quiet && !jsonOut {
				full := r.unit.Destination + "/" + f.rel
				if f.status == "changed" && f.userEdited {
					fmt.Fprintf(stdout, "restoring %s (your edits replaced by template)\n", full)
				} else {
					fmt.Fprintf(stdout, "wrote %s\n", full)
				}
			}
		}
		for _, rel := range skipped {
			if !quiet && !jsonOut {
				fmt.Fprintf(stdout, "skipped conflict %s — reconcile by hand or rerun with --apply --take-template to overwrite your version\n",
					r.unit.Destination+"/"+rel)
			}
		}
		if !wrote && !r.outdated && len(overrides) == 0 {
			continue // nothing changed for this unit; leave the shard alone
		}
		hashes := make(map[string]string, len(r.rendered))
		for p, data := range r.rendered {
			hashes[p] = hashNormalized(data)
		}
		// A skipped conflict keeps its previous baseline so the next make
		// still reports it as a conflict, not a clean template update.
		for _, rel := range skipped {
			if old, ok := r.unit.Files[rel]; ok {
				hashes[rel] = old
			}
		}
		unit := &answers.Unit{
			Version:         answers.CurrentVersion,
			Template:        r.unit.Template,
			TemplateVersion: r.tpl.Manifest.Version,
			Destination:     r.unit.Destination,
			GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
			Answers:         r.resolved,
			Files:           hashes,
		}
		if err := answers.Save(root, unit); err != nil {
			return UsageErrorf("%s", err)
		}
	}
	return nil
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// normalizeEOL collapses CRLF to LF so line-ending differences do not read as
// content drift. Used for comparison and baseline hashing only — never for the
// bytes written to disk.
func normalizeEOL(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

// hashNormalized is hashBytes over CRLF-normalized content, so the recorded
// baseline is EOL-agnostic and matches the comparison in evaluateUnit.
func hashNormalized(data []byte) string {
	return hashBytes(normalizeEOL(data))
}
