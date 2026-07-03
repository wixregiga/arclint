package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jofyi/arclint/internal/config"
	"github.com/jofyi/arclint/internal/output"
	"github.com/jofyi/arclint/internal/rules"
	"github.com/jofyi/arclint/internal/walk"
)

func init() { Register(newCheckCmd()) }

func newCheckCmd() *cobra.Command {
	var (
		rulesFlag string
		skipFlag  string
		jobs      int
	)
	cmd := &cobra.Command{
		Use:   "check [paths...]",
		Short: "Lint the architecture against .arclint/rules.yaml",
		Long: "Run every rule in .arclint/rules.yaml against the tree.\n" +
			"Exit 0 when clean (warn-severity findings never fail the run),\n" +
			"1 on error-severity violations, 2 on config or usage errors.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(cmd, args, rulesFlag, skipFlag, jobs)
		},
	}
	cmd.Flags().StringVar(&rulesFlag, "rules", "", "comma-separated rule ids to run exclusively")
	cmd.Flags().StringVar(&skipFlag, "skip", "", "comma-separated rule ids to skip")
	cmd.Flags().IntVar(&jobs, "jobs", 0, "parallelism for the file walk (0 = auto, all CPUs)")
	return cmd
}

func runCheck(cmd *cobra.Command, args []string, rulesFlag, skipFlag string, jobs int) error {
	start := time.Now()
	g := Globals()

	// Resolve the repo root: --config wins, else discover .arclint/ upward.
	root := g.Config
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return UsageErrorf("cannot determine working directory — %v", err)
		}
		root, err = config.FindConfigRoot(cwd)
		if err != nil {
			return UsageErrorf("%s", err)
		}
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return UsageErrorf("cannot resolve config root %q — %v", g.Config, err)
	}

	cfg, err := config.Load(config.RulesPath(root))
	if err != nil {
		return UsageErrorf("%s", err)
	}

	// Merge extends now so --rules/--skip validate against the full
	// effective registry, presets included.
	merged, err := rules.MergedRules(cfg)
	if err != nil {
		return UsageErrorf("%s", err)
	}
	effective, err := selectRules(merged, rulesFlag, skipFlag)
	if err != nil {
		return err
	}

	// Walk from the repo root so global exclude globs stay root-relative,
	// then narrow to the requested paths.
	files, err := walkRepo(root, cfg.Exclude)
	if err != nil {
		return UsageErrorf("cannot walk %s — %v", root, err)
	}
	if len(args) > 0 {
		files, err = filterToPaths(files, root, args)
		if err != nil {
			return err
		}
	}

	runCfg := *cfg
	runCfg.Rules = effective
	runCfg.Extends = nil

	rules.Jobs = jobs
	violations, err := rules.Evaluate(&runCfg, root, files)
	if err != nil {
		return UsageErrorf("%s", err)
	}

	sum := output.Summary{
		Total:        len(violations),
		FilesScanned: len(files),
		DurationMs:   time.Since(start).Milliseconds(),
	}
	stdout := cmd.OutOrStdout()
	if g.Format == FormatJSON {
		if err := output.JSON(stdout, violations, sum); err != nil {
			return UsageErrorf("cannot encode JSON output — %v", err)
		}
	} else {
		output.Text(stdout, violations, sum, g.Quiet)
	}

	for _, v := range violations {
		if v.Severity == config.SeverityError {
			return Findings()
		}
	}
	return nil
}

// selectRules applies --rules (exclusive allowlist) then --skip. Unknown
// ids are usage errors with a closest-match suggestion.
func selectRules(merged map[string]config.Rule, rulesFlag, skipFlag string) (map[string]config.Rule, error) {
	known := make([]string, 0, len(merged))
	for id := range merged {
		known = append(known, id)
	}
	sort.Strings(known)

	check := func(flag, id string) error {
		if _, ok := merged[id]; ok {
			return nil
		}
		msg := fmt.Sprintf("unknown rule id %q in --%s — available: %s", id, flag, strings.Join(known, ", "))
		if hint := closest(id, known); hint != "" {
			msg += fmt.Sprintf(" (did you mean %q?)", hint)
		}
		return UsageErrorf("%s", msg)
	}

	var out map[string]config.Rule
	if ids := splitIDs(rulesFlag); len(ids) > 0 {
		out = make(map[string]config.Rule, len(ids))
		for _, id := range ids {
			if err := check("rules", id); err != nil {
				return nil, err
			}
			out[id] = merged[id]
		}
	} else {
		out = make(map[string]config.Rule, len(merged))
		for id, r := range merged {
			out[id] = r
		}
	}
	for _, id := range splitIDs(skipFlag) {
		if err := check("skip", id); err != nil {
			return nil, err
		}
		delete(out, id)
	}
	return out, nil
}

func splitIDs(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// walkRepo returns every checkable file as a sorted, slash-separated,
// root-relative path.
func walkRepo(root string, excludes []string) ([]string, error) {
	abs, err := walk.WalkFiles([]string{root}, excludes)
	if err != nil {
		return nil, err
	}
	rels := make([]string, 0, len(abs))
	for _, a := range abs {
		rel, err := filepath.Rel(root, a)
		if err != nil {
			return nil, err
		}
		rels = append(rels, filepath.ToSlash(rel))
	}
	return rels, nil
}

// filterToPaths narrows the walked file list to the files or directories
// named on the command line. A nonexistent path is a usage error.
func filterToPaths(files []string, root string, args []string) ([]string, error) {
	var selectors []string
	for _, arg := range args {
		abs, err := filepath.Abs(arg)
		if err != nil {
			return nil, UsageErrorf("cannot resolve path %q — %v", arg, err)
		}
		if _, err := os.Stat(abs); err != nil {
			return nil, UsageErrorf("path %q does not exist — check the spelling or pass a path under the repo root", arg)
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil || strings.HasPrefix(filepath.ToSlash(rel), "../") {
			return nil, UsageErrorf("path %q is outside the repo root %s — pass a path under the root", arg, root)
		}
		selectors = append(selectors, filepath.ToSlash(rel))
	}

	var out []string
	for _, f := range files {
		for _, sel := range selectors {
			if sel == "." || f == sel || strings.HasPrefix(f, sel+"/") {
				out = append(out, f)
				break
			}
		}
	}
	return out, nil
}

// closest returns the candidate with the smallest Levenshtein distance to
// input, when that distance is small enough to be a plausible typo.
func closest(input string, candidates []string) string {
	best, bestDist := "", 4 // suggestions only for distance <= 3
	for _, c := range candidates {
		if d := levenshtein(input, c); d < bestDist {
			best, bestDist = c, d
		}
	}
	return best
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
