package gitproject_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gitproject "github.com/wixregiga/arclint/internal/infrastructure/project/git"
)

func needGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
}

// toplevel is the work tree git finds from dir, after links.
func toplevel(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-parse in %s: %v\n%s", dir, err, out)
	}
	top, err := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatal(err)
	}
	return top
}

func resolved(t *testing.T, dir string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestMakeCreatesTheDirectoryAsItsOwnRepository(t *testing.T) {
	needGit(t)
	// Inside another repository, the new one is still its own.
	outer := t.TempDir()
	if out, err := exec.Command("git", "init", "--quiet", outer).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	dir := filepath.Join(outer, "orders")
	made, err := gitproject.Make(dir)
	if err != nil || !made {
		t.Fatalf("made %v, err %v", made, err)
	}
	if got, want := toplevel(t, dir), resolved(t, dir); got != want {
		t.Fatalf("work tree = %q, want %q", got, want)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != ".git" {
		t.Fatalf("the new directory holds %v (%v), want only .git", entries, err)
	}
}

func TestMakeAcceptsAnEmptyDirectory(t *testing.T) {
	needGit(t)
	dir := t.TempDir()
	made, err := gitproject.Make(dir)
	if err != nil || made {
		t.Fatalf("made %v, err %v; an empty directory is found, not made", made, err)
	}
	if got, want := toplevel(t, dir), resolved(t, dir); got != want {
		t.Fatalf("work tree = %q, want %q", got, want)
	}
}

func TestMakeRefusesWhatItCannotStartCleanly(t *testing.T) {
	base := t.TempDir()
	occupied := filepath.Join(base, "occupied")
	if err := os.Mkdir(occupied, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(occupied, "notes.txt"), []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(base, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, dir, reason string }{
		{"relative", "orders", "not an absolute path"},
		{"missing parent", filepath.Join(base, "missing", "orders"), "does not exist"},
		{"parent is a file", filepath.Join(file, "orders"), "is not a directory"},
		{"a file in its place", file, "is not a directory"},
		{"holds files", occupied, "already holds files"},
	}
	for _, c := range cases {
		_, err := gitproject.Make(c.dir)
		if err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.reason)
		}
	}
	if _, err := os.Stat(filepath.Join(base, "missing")); !os.IsNotExist(err) {
		t.Fatal("a missing parent was created")
	}
	if entries, err := os.ReadDir(occupied); err != nil || len(entries) != 1 {
		t.Fatalf("the occupied directory changed: %v %v", entries, err)
	}
}

func TestMakeTakesBackADirectoryGitCannotInitialize(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := filepath.Join(t.TempDir(), "orders")
	_, err := gitproject.Make(dir)
	if err == nil || !strings.Contains(err.Error(), "git is not installed") {
		t.Fatalf("err = %v, want git named", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("the directory stayed after git failed")
	}
	empty := t.TempDir()
	if _, err := gitproject.Make(empty); err == nil {
		t.Fatal("Make succeeded without git")
	}
	if entries, err := os.ReadDir(empty); err != nil || len(entries) != 0 {
		t.Fatalf("an empty directory was left holding %v (%v)", entries, err)
	}
}

func TestDiscardTakesBackOnlyWhatMakeMade(t *testing.T) {
	needGit(t)
	base := t.TempDir()
	made := filepath.Join(base, "made")
	if _, err := gitproject.Make(made); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(made, "rules.arclint.yaml"), []byte("runtime: [go]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := gitproject.Discard(made, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(made); !os.IsNotExist(err) {
		t.Fatalf("a directory Make made stayed after Discard: %v", err)
	}

	found := filepath.Join(base, "found")
	if err := os.Mkdir(found, 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := gitproject.Make(found); err != nil {
		t.Fatal(err)
	}
	if err := gitproject.Discard(found, false); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(found)
	if err != nil || len(entries) != 0 {
		t.Fatalf("a directory Make found empty holds %v after Discard (%v), want it kept and empty", entries, err)
	}
	// Taken back, the same directory starts a project again.
	if _, err := gitproject.Make(found); err != nil {
		t.Fatalf("Make after Discard: %v", err)
	}
	if err := gitproject.Discard("relative", false); err == nil || !strings.Contains(err.Error(), "not an absolute path") {
		t.Fatalf("Discard of a relative path: %v", err)
	}
}
