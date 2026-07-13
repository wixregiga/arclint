package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/wixregiga/arclint/internal/rules"
)

const checkTestRules = `version: 1
rules:
  no-utils-dir:
    type: structure
    severity: error
    description: No grab-bag utility directories.
    params:
      forbid: ["**/utils/**"]
    fixHint: Move helpers next to the code that uses them.
  docs-required:
    type: structure
    severity: warn
    description: A docs directory must exist.
    params:
      require: ["docs/**"]
`

// checkRepo builds a temp repo with the given rules.yaml and files.
func checkRepo(t *testing.T, rulesYAML string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	all := map[string]string{".arclint/rules.yaml": rulesYAML}
	for p, c := range files {
		all[p] = c
	}
	for p, content := range all {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// resetCheckFlags puts the shared cobra command tree back to flag defaults
// so runs do not leak state into each other.
func resetCheckFlags(t *testing.T) {
	t.Helper()
	reset := func(f *pflag.Flag) {
		if err := f.Value.Set(f.DefValue); err != nil {
			t.Fatalf("reset flag %s: %v", f.Name, err)
		}
		f.Changed = false
	}
	rootCmd.PersistentFlags().VisitAll(reset)
	for _, c := range rootCmd.Commands() {
		if c.Name() == "check" {
			c.Flags().VisitAll(reset)
		}
	}
}

// runCheckCLI executes `arclint <args...>` in-process and returns stdout
// plus the exit code.
func runCheckCLI(t *testing.T, args ...string) (string, int) {
	t.Helper()
	resetCheckFlags(t)
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs(args)
	code := Execute()
	return buf.String(), code
}

func TestCheckViolationsExitOne(t *testing.T) {
	root := checkRepo(t, checkTestRules, map[string]string{
		"pkg/utils/x.go": "package utils\n",
		"docs/a.md":      "docs\n",
	})
	out, code := runCheckCLI(t, "check", "--config", root)
	if code != ExitFindings {
		t.Fatalf("want exit %d, got %d\n%s", ExitFindings, code, out)
	}
	for _, want := range []string{"structure (1)", "no-utils-dir", "pkg/utils/x.go", "fix: Move helpers"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

func TestCheckCleanExitZero(t *testing.T) {
	root := checkRepo(t, checkTestRules, map[string]string{
		"pkg/core/x.go": "package core\n",
		"docs/a.md":     "docs\n",
	})
	out, code := runCheckCLI(t, "check", "--config", root)
	if code != ExitOK {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "0 violations") {
		t.Errorf("clean run must print the summary line\n%s", out)
	}
}

func TestCheckWarnOnlyExitZero(t *testing.T) {
	// docs/** missing → only the warn rule fires; warn never exits 1.
	root := checkRepo(t, checkTestRules, map[string]string{
		"pkg/core/x.go": "package core\n",
	})
	out, code := runCheckCLI(t, "check", "--config", root)
	if code != ExitOK {
		t.Fatalf("warn-only run must exit 0, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "docs-required") {
		t.Errorf("warn violation must still print\n%s", out)
	}
}

func TestCheckJSONOutput(t *testing.T) {
	root := checkRepo(t, checkTestRules, map[string]string{
		"pkg/utils/x.go": "package utils\n",
		"docs/a.md":      "docs\n",
	})
	out, code := runCheckCLI(t, "check", "--config", root, "--format", "json")
	if code != ExitFindings {
		t.Fatalf("want exit 1, got %d\n%s", code, out)
	}
	var report struct {
		Violations []struct {
			RuleID   string `json:"ruleId"`
			Category string `json:"category"`
			Severity string `json:"severity"`
			Path     string `json:"path"`
			FixHint  string `json:"fixHint"`
		} `json:"violations"`
		Summary struct {
			Total        int `json:"total"`
			FilesScanned int `json:"filesScanned"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("stdout is not the JSON report: %v\n%s", err, out)
	}
	if report.Summary.Total != 1 || len(report.Violations) != 1 {
		t.Fatalf("want exactly the error violation, got %+v", report)
	}
	v := report.Violations[0]
	if v.RuleID != "no-utils-dir" || v.Category != "structure" || v.Severity != "error" || v.Path != "pkg/utils/x.go" {
		t.Errorf("unexpected violation: %+v", v)
	}
	if report.Summary.FilesScanned < 2 {
		t.Errorf("filesScanned looks wrong: %+v", report.Summary)
	}
}

func TestCheckRulesFlagUnknownID(t *testing.T) {
	root := checkRepo(t, checkTestRules, map[string]string{"docs/a.md": "d\n"})
	_, code := runCheckCLI(t, "check", "--config", root, "--rules", "no-such-rule")
	if code != ExitUsage {
		t.Fatalf("unknown --rules id must exit 2, got %d", code)
	}
}

func TestCheckSkipFlag(t *testing.T) {
	root := checkRepo(t, checkTestRules, map[string]string{
		"pkg/utils/x.go": "package utils\n",
		"docs/a.md":      "docs\n",
	})
	out, code := runCheckCLI(t, "check", "--config", root, "--skip", "no-utils-dir")
	if code != ExitOK {
		t.Fatalf("skipping the only failing rule must exit 0, got %d\n%s", code, out)
	}
}

func TestCheckNonexistentPathArg(t *testing.T) {
	root := checkRepo(t, checkTestRules, map[string]string{"docs/a.md": "d\n"})
	_, code := runCheckCLI(t, "check", "--config", root, filepath.Join(root, "ghost"))
	if code != ExitUsage {
		t.Fatalf("nonexistent path must exit 2, got %d", code)
	}
}

func TestCheckPathScoping(t *testing.T) {
	root := checkRepo(t, checkTestRules, map[string]string{
		"pkg/utils/x.go": "package utils\n",
		"clean/y.go":     "package clean\n",
		"docs/a.md":      "docs\n",
	})
	// Scoped to the clean subtree, the forbid rule has nothing to flag.
	out, code := runCheckCLI(t, "check", "--config", root, filepath.Join(root, "clean"))
	if code != ExitOK {
		t.Fatalf("scoped clean check must exit 0, got %d\n%s", code, out)
	}
}

// TestCheckJobsDefaultsToAutoSentinel covers MEDIUM 4: --jobs must
// default to 0 (the "auto, all CPUs" sentinel that rules.jobCount()
// understands), not runtime.NumCPU(). Defaulting to NumCPU() bakes in a
// fixed number at flag-registration time and masks the sentinel, so a
// caller can no longer tell "user explicitly requested N" apart from
// "user didn't pass --jobs at all".
func TestCheckJobsDefaultsToAutoSentinel(t *testing.T) {
	resetCheckFlags(t)
	for _, c := range rootCmd.Commands() {
		if c.Name() != "check" {
			continue
		}
		f := c.Flags().Lookup("jobs")
		if f == nil {
			t.Fatal("check command has no --jobs flag")
		}
		if f.DefValue != "0" {
			t.Fatalf("want --jobs default \"0\" (auto), got %q", f.DefValue)
		}
	}
}

// TestCheckJobsFlagPropagates confirms an explicit --jobs value still
// reaches rules.Jobs and a run completes normally with it set.
func TestCheckJobsFlagPropagates(t *testing.T) {
	root := checkRepo(t, checkTestRules, map[string]string{
		"pkg/core/x.go": "package core\n",
		"docs/a.md":     "docs\n",
	})
	out, code := runCheckCLI(t, "check", "--config", root, "--jobs", "2")
	if code != ExitOK {
		t.Fatalf("want exit 0 with --jobs 2, got %d\n%s", code, out)
	}
	if rules.Jobs != 2 {
		t.Fatalf("want rules.Jobs == 2 after --jobs 2, got %d", rules.Jobs)
	}
}
