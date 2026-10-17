package hooks

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"testing"
	"time"
)

// gitRepository creates a git repository with one commit of files and
// returns its root.
func gitRepository(t *testing.T, files map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	runGit(t, root, "init", "-q")
	for name, content := range files {
		writeFile(t, root, name, content)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "-c", "user.email=test@example.com", "-c", "user.name=test", "commit", "-q", "--allow-empty", "-m", "start")
	return root
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

// writeFile writes a file of the project as a shell command would.
func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func changedPaths(changed []changedFile) []string {
	paths := []string{}
	for _, file := range changed {
		paths = append(paths, file.path)
	}
	return paths
}

func takeSnapshot(t *testing.T, view checkout) snapshot {
	t.Helper()
	taken, err := view.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	return taken
}

// Every way a shell changes a file shows: a tracked file modified for the
// first time or again, a new file, a deleted file, and a dirty file
// restored to its committed content. A commit changes no file, ignored
// files are not the project's, and ArcLint's cache is left out.
func TestCheckoutShowsEveryChangeSinceASnapshot(t *testing.T) {
	root := gitRepository(t, map[string]string{
		".gitignore":       "bin/\n",
		"internal/a.go":    "package a\n",
		"internal/b.go":    "package b\n",
		"internal/c.go":    "package c\n",
		"internal/gone.go": "package gone\n",
	})
	view := checkout{root: root, now: time.Now}
	writeFile(t, root, "internal/b.go", "package b // dirty\n")
	writeFile(t, root, "internal/c.go", "package c // dirty\n")
	writeFile(t, root, "internal/committed.go", "package committed\n")
	before := takeSnapshot(t, view)

	writeFile(t, root, "internal/a.go", "package a // changed\n")
	writeFile(t, root, "internal/b.go", "package b // changed again\n")
	writeFile(t, root, "internal/new.go", "package created\n")
	writeFile(t, root, "bin/arclint", "binary")
	writeFile(t, root, ".arclint/cache/hooks/x.json", "{}")
	if err := os.Remove(filepath.Join(root, "internal/gone.go")); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "checkout", "--", "internal/c.go")
	runGit(t, root, "add", "internal/committed.go")
	runGit(t, root, "-c", "user.email=test@example.com", "-c", "user.name=test", "commit", "-q", "-m", "commit one file")

	changed := view.changes(before, takeSnapshot(t, view))
	got := changedPaths(changed)
	want := []string{"internal/a.go", "internal/b.go", "internal/c.go", "internal/gone.go", "internal/new.go"}
	if !reflect.DeepEqual(slices.Sorted(slices.Values(got)), want) {
		t.Fatalf("changed %v, want %v", got, want)
	}
	for index := 1; index < len(changed); index++ {
		if changed[index].at.Before(changed[index-1].at) {
			t.Fatalf("changes are not in the order they happened: %+v", changed)
		}
	}
}

func TestCheckoutWithoutChangesShowsNone(t *testing.T) {
	root := gitRepository(t, map[string]string{"internal/a.go": "package a\n"})
	view := checkout{root: root, now: time.Now}
	writeFile(t, root, "internal/a.go", "package a // dirty\n")
	before := takeSnapshot(t, view)
	if changed := view.changes(before, takeSnapshot(t, view)); len(changed) != 0 {
		t.Fatalf("an unchanged checkout showed %v", changedPaths(changed))
	}
}

// A project in a directory of a larger repository sees only its own files,
// named from its own root.
func TestCheckoutOfAProjectInsideARepository(t *testing.T) {
	repository := gitRepository(t, map[string]string{"service/internal/a.go": "package a\n", "other/b.go": "package b\n"})
	view := checkout{root: filepath.Join(repository, "service"), now: time.Now}
	before := takeSnapshot(t, view)
	writeFile(t, repository, "service/internal/a.go", "package a // changed\n")
	writeFile(t, repository, "other/b.go", "package b // changed\n")
	if got := changedPaths(view.changes(before, takeSnapshot(t, view))); !reflect.DeepEqual(got, []string{"internal/a.go"}) {
		t.Fatalf("changed %v, want [internal/a.go]", got)
	}
}

func TestCheckoutOutsideAGitWorkTreeHasNoSnapshot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	if _, err := (checkout{root: root, now: time.Now}).snapshot(); !errors.Is(err, errNoGit) {
		t.Fatalf("a directory outside any git work tree: %v", err)
	}
}

// A copy that keeps the source's size, modification time and mode, as
// `cp -p` and shutil.copy2 do, still changes the file.
func TestCheckoutSeesACopyThatKeepsTheMetadata(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file information records no change time")
	}
	root := gitRepository(t, map[string]string{"internal/a.go": "package a\n"})
	view := checkout{root: root, now: time.Now}
	writeFile(t, root, "internal/a.go", "package a // one\n")
	path := filepath.Join(root, "internal/a.go")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	before := takeSnapshot(t, view)
	time.Sleep(20 * time.Millisecond)
	writeFile(t, root, "internal/a.go", "package a // two\n")
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if got := changedPaths(view.changes(before, takeSnapshot(t, view))); !reflect.DeepEqual(got, []string{"internal/a.go"}) {
		t.Fatalf("a copy keeping the metadata showed %v", got)
	}
}
