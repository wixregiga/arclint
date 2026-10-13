package yamlvocab_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	yamlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/yaml"
)

// Recording a language keeps the domain file's mode, so a file the
// project shares stays shared; a new file is written owner-only, as the
// ruleset writer writes one. The proof edits this repository's real
// domain file.
func TestRecordKeepsTheFileMode(t *testing.T) {
	source, err := os.ReadFile("../../../../domain.arclint.yaml")
	if err != nil {
		t.Fatal(err)
	}
	rec := mustDefaultRecording(t)
	for _, mode := range []fs.FileMode{0o644, 0o640, 0o600} {
		root := t.TempDir()
		path := filepath.Join(root, rec.Path())
		if err := os.WriteFile(path, source, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		repo, err := yamlvocab.NewRepository(root, rec)
		if err != nil {
			t.Fatal(err)
		}
		lang, _, err := repo.RecordedLanguage()
		if err != nil {
			t.Fatal(err)
		}
		lang.Contexts[0].Definition += " This sentence is the edit."
		if err := repo.Record(lang); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != mode {
			t.Errorf("a %o file became %o after Record", mode, info.Mode().Perm())
		}
	}

	root := t.TempDir()
	repo, err := yamlvocab.NewRepository(root, rec)
	if err != nil {
		t.Fatal(err)
	}
	lang, err := yamlvocab.Parse(source, rec)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Record(lang); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(root, rec.Path()))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("a new file was written %o, want 600", info.Mode().Perm())
	}
}
