package filesystemobservation_test

import (
	"os"
	"path/filepath"
	"slices"
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
	obs, err := source.Observe([]rule.Language{rule.LanguageGo}, rule.Scan{Exclude: exclude}, nil, nil)
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
	obs, err := source.Observe(nil, rule.Scan{}, nil, nil)
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
	obs, err := source.Observe(nil, rule.Scan{}, nil, nil)
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

// The content capability tells a file the scan leaves out from one the
// repository does not hold: it holds every regular file under the root,
// observed or not, and nothing outside it, no directory, and no link.
func TestContentHoldsFilesTheScanLeavesOut(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a/service.go", "package a")
	write(t, root, "b/generated/skip_test.go", "package b")
	write(t, root, "testdata/fixture_test.go", "package fixture")
	if err := os.Symlink(filepath.Join(root, "a", "service.go"), filepath.Join(root, "a", "link.go")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	exclude, err := rule.NewGlobs([]string{"**/generated/**"})
	if err != nil {
		t.Fatalf("NewGlobs: %v", err)
	}
	source, err := filesystemobservation.NewSource(root)
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	obs, err := source.Observe(nil, rule.Scan{Exclude: exclude}, nil, nil)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	content := obs.Content()
	for path, want := range map[string]bool{
		"a/service.go":             true,
		"b/generated/skip_test.go": true,
		"testdata/fixture_test.go": true,
		"a/missing_test.go":        false,
		"a":                        false,
		"a/link.go":                false,
		"../outside_test.go":       false,
		"/etc/passwd":              false,
	} {
		if got := content.Holds(path); got != want {
			t.Errorf("Holds(%q) = %v, want %v", path, got, want)
		}
	}
}

// requestRecorder records the fact classes its language was asked for.
type requestRecorder struct {
	language  rule.Language
	requested *[]rule.Fact
}

func (r requestRecorder) Language() rule.Language { return r.language }

func (r requestRecorder) Facts(root string, files []conformance.ObservedFile, requested []rule.Fact) (map[string]conformance.LanguageFacts, error) {
	*r.requested = append([]rule.Fact(nil), requested...)
	return map[string]conformance.LanguageFacts{}, nil
}

// The reference facts go only to a language holding a file they are
// needed for; the other languages keep every other class, and no
// selection keeps them for every language.
func TestObserveNarrowsReferenceFactsToTheSelectedLanguages(t *testing.T) {
	root := t.TempDir()
	write(t, root, "app/uninstall.go", "package app")
	write(t, root, "web/src/main.ts", "export {}")
	var goFacts, tsFacts []rule.Fact
	source, err := filesystemobservation.NewSource(root,
		requestRecorder{language: rule.LanguageGo, requested: &goFacts},
		requestRecorder{language: rule.LanguageTypeScript, requested: &tsFacts})
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	languages := []rule.Language{rule.LanguageGo, rule.LanguageTypeScript}
	all := []rule.Fact{rule.FactDeclarations, rule.FactCalls, rule.FactResolvedCalls, rule.FactFieldTypes}
	inApp := func(p string) bool { return p == "app/uninstall.go" }
	if _, err := source.Observe(languages, rule.Scan{}, all, inApp); err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if !slices.Equal(goFacts, all) {
		t.Errorf("go asked for %v, want %v", goFacts, all)
	}
	if want := []rule.Fact{rule.FactDeclarations}; !slices.Equal(tsFacts, want) {
		t.Errorf("typescript asked for %v, want %v", tsFacts, want)
	}
	if _, err := source.Observe(languages, rule.Scan{}, all, nil); err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if !slices.Equal(goFacts, all) || !slices.Equal(tsFacts, all) {
		t.Errorf("without a selection every language is asked for every class: go %v, typescript %v", goFacts, tsFacts)
	}
}
