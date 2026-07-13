package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/answers"
)

// generateUnit runs `arclint new svc <name>` and fails the test on error.
func generateUnit(t *testing.T, root, name string) {
	t.Helper()
	if _, stderr, code := runArclint(t, root, "new", "svc", name); code != 0 {
		t.Fatalf("new svc %s: exit %d, stderr: %s", name, code, stderr)
	}
}

func TestMakeCleanExitsZero(t *testing.T) {
	root := newTestRepo(t)
	generateUnit(t, root, "alpha")
	stdout, stderr, code := runArclint(t, root, "make")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "all 1 unit clean") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestMakeDriftDetectApplyRestores(t *testing.T) {
	root := newTestRepo(t)
	generateUnit(t, root, "alpha")
	edited := filepath.Join(root, "services/alpha/main.go")
	original := mustReadFile(t, edited)
	if err := os.WriteFile(edited, []byte("package main // user hacked this\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Default dry-run reports drift and exits 0.
	stdout, stderr, code := runArclint(t, root, "make")
	if code != 0 {
		t.Fatalf("dry-run exit %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "drift: services/alpha") || !strings.Contains(stdout, "modified") {
		t.Errorf("drift report wrong: %q", stdout)
	}

	// --fail-on-drift flips the exit to 1.
	_, _, code = runArclint(t, root, "make", "--fail-on-drift")
	if code != ExitFindings {
		t.Fatalf("--fail-on-drift exit %d, want 1", code)
	}

	// --diff shows a unified diff.
	stdout, _, _ = runArclint(t, root, "make", "--diff")
	if !strings.Contains(stdout, "--- a/services/alpha/main.go") || !strings.Contains(stdout, "@@") {
		t.Errorf("--diff output lacks unified diff markers: %q", stdout)
	}

	// --apply restores the template render (template unchanged, user edited)
	// and must name the file as restored, not a plain write.
	stdout, stderr, code = runArclint(t, root, "make", "--apply")
	if code != 0 {
		t.Fatalf("--apply exit %d, stderr: %s", code, stderr)
	}
	if got := mustReadFile(t, edited); got != original {
		t.Errorf("apply must restore render, got %q", got)
	}
	if !strings.Contains(stdout, "restoring services/alpha/main.go (your edits replaced by template)") {
		t.Errorf("apply must name the restored file: %q", stdout)
	}

	// And a second make is clean again.
	stdout, _, code = runArclint(t, root, "make")
	if code != 0 || !strings.Contains(stdout, "all 1 unit clean") {
		t.Errorf("post-apply state not clean: %q", stdout)
	}
}

// TestMakeCRLFNoDrift pins the medium-4 fix: a file checked out with CRLF line
// endings must not read as drift against the LF-rendered template — only line
// endings differ, so the content is identical after normalization.
func TestMakeCRLFNoDrift(t *testing.T) {
	root := newTestRepo(t)
	generateUnit(t, root, "alpha")
	f := filepath.Join(root, "services/alpha/main.go")
	lf := mustReadFile(t, f)
	if !strings.Contains(lf, "\n") || strings.Contains(lf, "\r\n") {
		t.Fatalf("fixture expected LF content, got %q", lf)
	}
	crlf := strings.ReplaceAll(lf, "\n", "\r\n")
	if err := os.WriteFile(f, []byte(crlf), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runArclint(t, root, "make")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "all 1 unit clean") {
		t.Errorf("CRLF-only difference must not be drift: %q", stdout)
	}
	// --fail-on-drift must still pass (exit 0), confirming no drift in CI.
	if _, _, code = runArclint(t, root, "make", "--fail-on-drift"); code != 0 {
		t.Errorf("--fail-on-drift exit %d, want 0 for CRLF-only", code)
	}
	// The CRLF bytes on disk must be left untouched (normalization is
	// comparison-only, never a mutation).
	if got := mustReadFile(t, f); got != crlf {
		t.Errorf("disk bytes must be untouched, got %q", got)
	}
}

func TestMakeDeletedFileReportedAddedAndRestored(t *testing.T) {
	root := newTestRepo(t)
	generateUnit(t, root, "beta")
	if err := os.Remove(filepath.Join(root, "services/beta/service.yml")); err != nil {
		t.Fatal(err)
	}
	stdout, _, code := runArclint(t, root, "make")
	if code != 0 || !strings.Contains(stdout, "created   services/beta/service.yml") {
		t.Fatalf("missing-file drift wrong (exit %d): %q", code, stdout)
	}
	_, _, code = runArclint(t, root, "make", "--apply")
	if code != 0 {
		t.Fatal("apply failed")
	}
	if _, err := os.Stat(filepath.Join(root, "services/beta/service.yml")); err != nil {
		t.Error("apply must recreate the deleted file")
	}
}

// bumpTemplate changes the svc template content and version, so re-renders
// differ from the recorded baseline.
func bumpTemplate(t *testing.T, root string) {
	t.Helper()
	writeRepoFiles(t, root, map[string]string{
		".arclint/templates/svc/files/main.go": "package main // v2 {{ name | pascal }} {{ transport }}\n",
	})
	manifest := mustReadFile(t, filepath.Join(root, ".arclint/templates/svc/template.yaml"))
	manifest = strings.Replace(manifest, "version: 1", "version: 2", 1)
	writeRepoFiles(t, root, map[string]string{".arclint/templates/svc/template.yaml": manifest})
}

func TestMakeConflictPolicyHonored(t *testing.T) {
	root := newTestRepo(t)
	generateUnit(t, root, "gamma")
	edited := filepath.Join(root, "services/gamma/main.go")
	userVersion := "package main // user edit\n"
	if err := os.WriteFile(edited, []byte(userVersion), 0o644); err != nil {
		t.Fatal(err)
	}
	bumpTemplate(t, root) // user edited AND template changed -> conflict

	stdout, _, code := runArclint(t, root, "make")
	if code != 0 {
		t.Fatalf("dry-run exit %d", code)
	}
	if !strings.Contains(stdout, "conflict: services/gamma") || !strings.Contains(stdout, "conflict  services/gamma/main.go") {
		t.Fatalf("conflict not reported: %q", stdout)
	}
	if !strings.Contains(stdout, "outdated") {
		t.Errorf("version bump must flag outdated: %q", stdout)
	}

	// --apply skips the conflicted file but advances template_version.
	stdout, _, code = runArclint(t, root, "make", "--apply")
	if code != 0 {
		t.Fatalf("apply exit %d", code)
	}
	if !strings.Contains(stdout, "skipped conflict services/gamma/main.go") {
		t.Errorf("apply must report the skip: %q", stdout)
	}
	if got := mustReadFile(t, edited); got != userVersion {
		t.Errorf("apply must not touch a conflicted file, got %q", got)
	}
	u, err := answers.Load(answers.Path(root, "services/gamma"))
	if err != nil || u.TemplateVersion != 2 {
		t.Errorf("template_version must advance to 2, got %+v (%v)", u, err)
	}

	// Still a conflict on the next run (baseline for the file was kept).
	stdout, _, _ = runArclint(t, root, "make")
	if !strings.Contains(stdout, "conflict  services/gamma/main.go") {
		t.Errorf("conflict must persist after skipping apply: %q", stdout)
	}

	// --apply --take-template overwrites the user's version.
	_, _, code = runArclint(t, root, "make", "--apply", "--take-template")
	if code != 0 {
		t.Fatal("take-template apply failed")
	}
	if got := mustReadFile(t, edited); !strings.Contains(got, "v2 Gamma") {
		t.Errorf("take-template must write the render, got %q", got)
	}
	stdout, _, code = runArclint(t, root, "make")
	if code != 0 || !strings.Contains(stdout, "all 1 unit clean") {
		t.Errorf("post-take-template not clean: %q", stdout)
	}
}

func TestMakeTemplateUpdateWithoutUserEditIsDrift(t *testing.T) {
	root := newTestRepo(t)
	generateUnit(t, root, "delta")
	bumpTemplate(t, root) // template changed, user did not edit

	stdout, _, code := runArclint(t, root, "make")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "drift: services/delta") || strings.Contains(stdout, "conflict") {
		t.Fatalf("clean template update must be drift, not conflict: %q", stdout)
	}
	stdout, _, code = runArclint(t, root, "make", "--apply")
	if code != 0 {
		t.Fatal("apply failed")
	}
	if got := mustReadFile(t, filepath.Join(root, "services/delta/main.go")); !strings.Contains(got, "v2 Delta") {
		t.Errorf("apply must write the template update, got %q", got)
	}
	if !strings.Contains(stdout, "wrote services/delta/main.go") || strings.Contains(stdout, "restoring") {
		t.Errorf("untouched-since-generation file must use plain wrote, not restoring: %q", stdout)
	}
}

func TestMakeOrphanWhenTemplateGone(t *testing.T) {
	root := newTestRepo(t)
	generateUnit(t, root, "omega")
	if err := os.RemoveAll(filepath.Join(root, ".arclint/templates/svc")); err != nil {
		t.Fatal(err)
	}
	stdout, _, code := runArclint(t, root, "make")
	if code != 0 {
		t.Fatalf("orphan report must exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "orphan: services/omega") {
		t.Errorf("orphan not reported: %q", stdout)
	}
	_, _, code = runArclint(t, root, "make", "--fail-on-drift")
	if code != ExitFindings {
		t.Errorf("orphan under --fail-on-drift must exit 1, got %d", code)
	}
}

func TestMakeUnknownPathExits2(t *testing.T) {
	root := newTestRepo(t)
	generateUnit(t, root, "alpha")
	_, stderr, code := runArclint(t, root, "make", "services/ghosts")
	if code != ExitUsage || !strings.Contains(stderr, "has no recorded unit") {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
}

func TestMakePathScoping(t *testing.T) {
	root := newTestRepo(t)
	generateUnit(t, root, "alpha")
	generateUnit(t, root, "beta")
	if err := os.WriteFile(filepath.Join(root, "services/beta/main.go"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Scoped to alpha only: clean, beta's drift invisible.
	stdout, _, code := runArclint(t, root, "make", "services/alpha")
	if code != 0 || !strings.Contains(stdout, "all 1 unit clean") {
		t.Fatalf("scoped make wrong (exit %d): %q", code, stdout)
	}
	// Parent path selects both.
	stdout, _, _ = runArclint(t, root, "make", "services")
	if !strings.Contains(stdout, "drift: services/beta") {
		t.Errorf("parent path must select beta: %q", stdout)
	}
}

func TestMakeJSONFormat(t *testing.T) {
	root := newTestRepo(t)
	generateUnit(t, root, "alpha")
	if err := os.WriteFile(filepath.Join(root, "services/alpha/main.go"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runArclint(t, root, "make", "--format", "json")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	var report struct {
		Units []struct {
			Unit     string `json:"unit"`
			Template string `json:"template"`
			Status   string `json:"status"`
			Files    []struct {
				Path   string `json:"path"`
				Status string `json:"status"`
			} `json:"files"`
		} `json:"units"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if len(report.Units) != 1 {
		t.Fatalf("units = %+v", report.Units)
	}
	u := report.Units[0]
	if u.Unit != "services/alpha" || u.Template != "svc" || u.Status != "drift" {
		t.Errorf("unit record = %+v", u)
	}
	if len(u.Files) != 1 || u.Files[0].Path != "services/alpha/main.go" || u.Files[0].Status != "changed" {
		t.Errorf("file record = %+v", u.Files)
	}
}

func TestMakeVarOverride(t *testing.T) {
	root := newTestRepo(t)
	generateUnit(t, root, "epsilon")
	// Override transport: render differs -> drift; not persisted on dry-run.
	stdout, _, code := runArclint(t, root, "make", "--var", "transport=grpc")
	if code != 0 || !strings.Contains(stdout, "drift: services/epsilon") {
		t.Fatalf("override drift wrong (exit %d): %q", code, stdout)
	}
	u, _ := answers.Load(answers.Path(root, "services/epsilon"))
	if u.Answers["transport"] != "http" {
		t.Errorf("dry-run must not persist overrides, got %v", u.Answers)
	}
	// With --apply the override is written and persisted.
	_, _, code = runArclint(t, root, "make", "--var", "transport=grpc", "--apply")
	if code != 0 {
		t.Fatal("apply failed")
	}
	u, _ = answers.Load(answers.Path(root, "services/epsilon"))
	if u.Answers["transport"] != "grpc" {
		t.Errorf("apply must persist overrides, got %v", u.Answers)
	}
	if got := mustReadFile(t, filepath.Join(root, "services/epsilon/main.go")); !strings.Contains(got, "grpc") {
		t.Errorf("override not rendered: %q", got)
	}
	// Unknown override name is exit 2.
	_, stderr, code := runArclint(t, root, "make", "--var", "nosuch=1")
	if code != ExitUsage || !strings.Contains(stderr, `unknown variable "nosuch"`) {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
}

func TestMakeNoInputMissingRequiredExits2(t *testing.T) {
	root := newTestRepo(t)
	generateUnit(t, root, "alpha")
	// The template gains a new required variable (no default) the recorded
	// answers cannot supply; under --no-input that is a hard exit-2 error.
	manifest := mustReadFile(t, filepath.Join(root, ".arclint/templates/svc/template.yaml"))
	manifest += `  - name: owner
    description: "Owning team"
    type: string
`
	writeRepoFiles(t, root, map[string]string{".arclint/templates/svc/template.yaml": manifest})

	_, stderr, code := runArclint(t, root, "make", "--no-input")
	if code != ExitUsage {
		t.Fatalf("exit %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, `missing required input "owner"`) || !strings.Contains(stderr, "--var owner=") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestMakeTakeTemplateRequiresApply(t *testing.T) {
	root := newTestRepo(t)
	generateUnit(t, root, "alpha")
	_, stderr, code := runArclint(t, root, "make", "--take-template")
	if code != ExitUsage || !strings.Contains(stderr, "--take-template") {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
}
