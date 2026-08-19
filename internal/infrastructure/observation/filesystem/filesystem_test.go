package filesystemobservation_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	filesystemobservation "github.com/wixregiga/arclint/internal/infrastructure/observation/filesystem"
)

type stubProducer struct {
	language rule.Language
	claimed  string
}

func (s stubProducer) Language() rule.Language { return s.language }

func (s stubProducer) Facts(root string, files []conformance.ObservedFile, requested []rule.Fact) (map[string]conformance.LanguageFacts, error) {
	return map[string]conformance.LanguageFacts{
		s.claimed: {Language: s.language, ImportsAvailable: true},
	}, nil
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestObserveWalksDeterministically(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a/service.go", "package a")
	write(t, root, "b/generated/skip.go", "package b")
	write(t, root, "node_modules/dep/index.js", "x")
	write(t, root, ".git/config", "x")
	write(t, root, "testdata/fixture.go", "package fixture")
	write(t, root, "b/keep.go", "package b")
	if err := os.Symlink(filepath.Join(root, "a"), filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	exclude, err := rule.NewGlobs([]string{"**/generated/**"})
	if err != nil {
		t.Fatalf("NewGlobs: %v", err)
	}
	source, err := filesystemobservation.NewSource(root,
		stubProducer{language: rule.LanguageGo, claimed: "a/service.go"})
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	obs, err := source.Observe([]rule.Language{rule.LanguageGo}, rule.Scan{Exclude: exclude}, nil)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	var paths []string
	for _, f := range obs.Files() {
		paths = append(paths, f.Path)
	}
	want := []string{"a/service.go", "b/keep.go"}
	if len(paths) != len(want) {
		t.Fatalf("walked %v, want %v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("walked %v, want %v", paths, want)
		}
	}
	if _, ok := obs.FactsFor("a/service.go"); !ok {
		t.Errorf("producer facts missing for a/service.go")
	}
}

func TestObserveAttachesLazyRepositoryContent(t *testing.T) {
	root := t.TempDir()
	write(t, root, "m/a.go", "package m // production")
	source, err := filesystemobservation.NewSource(root)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	obs, err := source.Observe(nil, rule.Scan{}, nil)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	content := obs.Content()
	if content == nil {
		t.Fatal("production observations must lend a content capability")
	}
	got, err := content.Read("m/a.go")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != "package m // production" {
		t.Errorf("Read = %q, want production bytes", got)
	}
	// Prove laziness: a post-Observe write is visible on the next Read.
	write(t, root, "m/a.go", "package m // updated")
	got, err = content.Read("m/a.go")
	if err != nil {
		t.Fatalf("Read after update: %v", err)
	}
	if got != "package m // updated" {
		t.Errorf("Read after update = %q, want lazy filesystem bytes", got)
	}
}

func TestObserveSkipsUnrequestedLanguages(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a/service.go", "package a")
	source, err := filesystemobservation.NewSource(root,
		stubProducer{language: rule.LanguageGo, claimed: "a/service.go"})
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	obs, err := source.Observe(nil, rule.Scan{}, nil)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if _, ok := obs.FactsFor("a/service.go"); ok {
		t.Errorf("facts produced for a language nobody requested")
	}
}

func TestNewSourceRejectsDuplicateProducers(t *testing.T) {
	root := t.TempDir()
	_, err := filesystemobservation.NewSource(root,
		stubProducer{language: rule.LanguageGo, claimed: "x"},
		stubProducer{language: rule.LanguageGo, claimed: "y"})
	if err == nil {
		t.Errorf("two producers for one language must be rejected")
	}
}
