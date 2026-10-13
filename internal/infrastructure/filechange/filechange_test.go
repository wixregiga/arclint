package filechange

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func writeFixture(t *testing.T, path, content string, mode fs.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func requireFile(t *testing.T, path, content string, mode fs.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content || info.Mode().Perm() != mode {
		t.Fatalf("%s holds %q with mode %o, want %q with mode %o", path, data, info.Mode().Perm(), content, mode)
	}
}

func requireAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("%s exists: %v", path, err)
	}
}

// Planning writes nothing; applying writes the new content and keeps the
// file's mode; reverting puts the old content back.
func TestWriteAppliesAndRevertsAnExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.arclint.yaml")
	writeFixture(t, path, "before\n", 0o644)
	change, err := Write(path, []byte("after\n"))
	if err != nil {
		t.Fatal(err)
	}
	requireFile(t, path, "before\n", 0o644)
	if err := change.Apply(); err != nil {
		t.Fatal(err)
	}
	requireFile(t, path, "after\n", 0o644)
	if err := change.Revert(); err != nil {
		t.Fatal(err)
	}
	requireFile(t, path, "before\n", 0o644)
}

// A new file and the directories it needs are created on Apply and all
// removed again on Revert.
func TestWriteRevertsANewFileAndItsDirectories(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "docs", "model", "domain.cml")
	change, err := Write(path, []byte("Domain shop\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := change.Apply(); err != nil {
		t.Fatal(err)
	}
	requireFile(t, path, "Domain shop\n", newFileMode)
	if err := change.Revert(); err != nil {
		t.Fatal(err)
	}
	requireAbsent(t, filepath.Join(root, "docs"))
}

// A write that changes nothing writes nothing, and so has nothing to put
// back.
func TestWriteOfTheSameContentIsNoChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "domain.arclint.yaml")
	writeFixture(t, path, "same\n", 0o600)
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	change, err := Write(path, []byte("same\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := change.Apply(); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("an unchanged file was replaced")
	}
}

// Removing a file and putting it back restores its content and mode.
func TestRemoveAppliesAndReverts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "domain.arclint.yaml")
	writeFixture(t, path, "language\n", 0o640)
	change, err := Remove(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := change.Apply(); err != nil {
		t.Fatal(err)
	}
	requireAbsent(t, path)
	if err := change.Revert(); err != nil {
		t.Fatal(err)
	}
	requireFile(t, path, "language\n", 0o640)
}

// A symbolic link is removed itself, and put back as the same link.
func TestRemoveRevertsASymbolicLink(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, filepath.Join(root, "actual.yaml"), "language\n", 0o600)
	link := filepath.Join(root, "domain.arclint.yaml")
	if err := os.Symlink("actual.yaml", link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	change, err := Remove(link)
	if err != nil {
		t.Fatal(err)
	}
	if err := change.Apply(); err != nil {
		t.Fatal(err)
	}
	requireAbsent(t, link)
	requireFile(t, filepath.Join(root, "actual.yaml"), "language\n", 0o600)
	if err := change.Revert(); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(link)
	if err != nil || target != "actual.yaml" {
		t.Fatalf("the link was not put back: %q, %v", target, err)
	}
}

// A write the file system refuses leaves no file and no directory it
// created behind.
func TestWriteThatFailsLeavesNothingBehind(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permissions do not bind the superuser")
	}
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	if err := os.Mkdir(locked, 0o750); err != nil {
		t.Fatal(err)
	}
	change, err := Write(filepath.Join(locked, "docs", "domain.cml"), []byte("Domain shop\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o750) })
	if err := change.Apply(); err == nil {
		t.Fatal("a write into a read-only directory succeeded")
	}
	requireAbsent(t, filepath.Join(locked, "docs"))
}

// Writing through a symbolic link changes the file it points to and keeps
// the link; putting the change back restores that file.
func TestWriteKeepsASymbolicLink(t *testing.T) {
	root := t.TempDir()
	actual := filepath.Join(root, "config", "rules.yaml")
	writeFixture(t, actual, "before\n", 0o644)
	link := filepath.Join(root, "rules.arclint.yaml")
	if err := os.Symlink(filepath.Join("config", "rules.yaml"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	change, err := Write(link, []byte("after\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := change.Apply(); err != nil {
		t.Fatal(err)
	}
	if target, err := os.Readlink(link); err != nil || target != filepath.Join("config", "rules.yaml") {
		t.Fatalf("the link was replaced: %q, %v", target, err)
	}
	requireFile(t, actual, "after\n", 0o644)
	if err := change.Revert(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Readlink(link); err != nil {
		t.Fatalf("putting the change back replaced the link: %v", err)
	}
	requireFile(t, actual, "before\n", 0o644)
}

// A link that points to nothing is refused, not replaced by a file.
func TestWriteRefusesADanglingLink(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(root, "rules.arclint.yaml")
	if err := os.Symlink("missing.yaml", link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := Write(link, []byte("after\n")); err == nil {
		t.Fatal("a dangling link was accepted as a write target")
	}
}
