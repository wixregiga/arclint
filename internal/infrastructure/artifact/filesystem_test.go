package artifactfs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	artifactfs "github.com/wixregiga/arclint/internal/infrastructure/artifact"
)

func TestWriteCreatesDirectoryAndReportsChange(t *testing.T) {
	root := t.TempDir()
	writer, err := artifactfs.NewWriter(root)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	changed, path, err := writer.Write(".arclint/schemas", "rules.arclint.schema.json", []byte("{}\n"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !changed {
		t.Fatal("first write reported no change")
	}
	if want := filepath.Join(root, ".arclint", "schemas", "rules.arclint.schema.json"); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "{}\n" {
		t.Fatalf("content = %q", got)
	}

	changed, _, err = writer.Write(".arclint/schemas", "rules.arclint.schema.json", []byte("{}\n"))
	if err != nil {
		t.Fatalf("identical Write: %v", err)
	}
	if changed {
		t.Fatal("identical bytes reported as a change")
	}

	changed, _, err = writer.Write(".arclint/schemas", "rules.arclint.schema.json", []byte("{\"a\":1}\n"))
	if err != nil {
		t.Fatalf("differing Write: %v", err)
	}
	if !changed {
		t.Fatal("differing bytes reported as unchanged")
	}
	entries, err := os.ReadDir(filepath.Join(root, ".arclint", "schemas"))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".artifact-") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}

func TestWriteHonoursAbsoluteDirectory(t *testing.T) {
	root := t.TempDir()
	elsewhere := t.TempDir()
	writer, err := artifactfs.NewWriter(root)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	_, path, err := writer.Write(elsewhere, "SKILL.md", []byte("# skill\n"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if want := filepath.Join(elsewhere, "SKILL.md"); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	if _, err := os.Stat(filepath.Join(root, "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("absolute dir must not write under root; stat err = %v", err)
	}
}

func TestWriteRejectsEmptyNames(t *testing.T) {
	writer, err := artifactfs.NewWriter(t.TempDir())
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	if _, _, err := writer.Write("", "SKILL.md", []byte("x")); err == nil {
		t.Fatal("empty dir accepted")
	}
	if _, _, err := writer.Write(".agents", "", []byte("x")); err == nil {
		t.Fatal("empty filename accepted")
	}
}
