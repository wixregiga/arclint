package studio_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/infrastructure/studio"
)

func TestSourceFilesStayInsideTheProject(t *testing.T) {
	project := t.TempDir()
	outside := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(project, "catalog", "event.go"), "package catalog\n")
	write(filepath.Join(outside, "secret.go"), "outside\n")
	link := func(target, name string) {
		t.Helper()
		if err := os.Symlink(target, filepath.Join(project, name)); err != nil {
			t.Fatal(err)
		}
	}
	link(filepath.Join(outside, "secret.go"), "catalog/escape.go")
	link(outside, "catalog/away")
	link("catalog/event.go", "inside.go")
	files := studio.SourceFiles{Project: project}

	if got, err := files.ReadSource("catalog/event.go", 1024); err != nil || string(got) != "package catalog\n" {
		t.Fatalf("a project file: %q %v", got, err)
	}
	// A link is never followed, not even to a file inside the project:
	// a source file swapped for a link would otherwise be read under its
	// name while it shows another file.
	if got, err := files.ReadSource("inside.go", 1024); err == nil {
		t.Errorf("a link inside the project was followed: %q", got)
	}
	if err := os.Symlink(filepath.Join(project, "catalog"), filepath.Join(project, "linked")); err != nil {
		t.Fatal(err)
	}
	if got, err := files.ReadSource("linked/event.go", 1024); err == nil {
		t.Errorf("a folder link inside the project was followed: %q", got)
	}
	for _, path := range []string{"catalog/escape.go", "catalog/away/secret.go", "../" + filepath.Base(outside) + "/secret.go", filepath.Join(outside, "secret.go")} {
		if got, err := files.ReadSource(path, 1024); err == nil || strings.Contains(string(got), "outside") {
			t.Errorf("%s: read %q, want a refusal", path, got)
		}
	}
	if _, err := files.ReadSource("catalog", 1024); err == nil {
		t.Error("a directory was read")
	}
	if _, err := files.ReadSource("catalog/event.go", 4); err == nil {
		t.Error("a file over the limit was read")
	}
	if _, err := files.ReadSource("catalog/missing.go", 1024); err == nil {
		t.Error("a missing file was read")
	}
}
