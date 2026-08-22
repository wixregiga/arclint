package yamlvocab_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	yamlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/yaml"
)

func TestLoadMissingFile(t *testing.T) {
	repo := newRepo(t, t.TempDir())
	lang, found, err := repo.RecordedLanguage()
	if err != nil {
		t.Fatalf("RecordedLanguage: %v", err)
	}
	if found {
		t.Fatal("found = true, want false")
	}
	if !lang.Empty() {
		t.Fatalf("empty model expected, got %#v", lang)
	}
}

func TestLoadRejectsMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	writeModel(t, dir, ":\n  - not yaml\n")
	repo := newRepo(t, dir)
	if _, _, err := repo.RecordedLanguage(); err == nil {
		t.Fatal("RecordedLanguage accepted malformed YAML")
	}
}

func TestLoadRejectsVersion2(t *testing.T) {
	dir := t.TempDir()
	writeModel(t, dir, "version: 2\nentities: []\n")
	repo := newRepo(t, dir)
	_, found, err := repo.RecordedLanguage()
	if err == nil {
		t.Fatal("RecordedLanguage accepted version 2")
	}
	if !found {
		t.Fatal("found = false for present invalid file")
	}
	if !strings.Contains(err.Error(), "unsupported version 2") {
		t.Fatalf("error = %v, want unsupported version 2", err)
	}
}

func TestLoadRejectsMissingVersion(t *testing.T) {
	dir := t.TempDir()
	writeModel(t, dir, "entities:\n  - name: Order\n")
	repo := newRepo(t, dir)
	_, _, err := repo.RecordedLanguage()
	if err == nil {
		t.Fatal("RecordedLanguage accepted missing version")
	}
	if !strings.Contains(err.Error(), "missing version") {
		t.Fatalf("error = %v, want missing version", err)
	}
}

func TestLoadRejectsEmptyAndDuplicateNames(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		dir := t.TempDir()
		writeModel(t, dir, "version: 1\nentities:\n  - name: \"\"\n")
		repo := newRepo(t, dir)
		if _, _, err := repo.RecordedLanguage(); err == nil {
			t.Fatal("RecordedLanguage accepted empty name")
		}
	})
	t.Run("duplicate name", func(t *testing.T) {
		dir := t.TempDir()
		writeModel(t, dir, "version: 1\nentities:\n  - name: Order\n  - name: Order\n")
		repo := newRepo(t, dir)
		if _, _, err := repo.RecordedLanguage(); err == nil {
			t.Fatal("RecordedLanguage accepted duplicate name")
		}
	})
}

func TestLoadRejectsAggregateOnValueObject(t *testing.T) {
	dir := t.TempDir()
	writeModel(t, dir, `version: 1
value_objects:
  - name: Money
    aggregate: true
`)
	repo := newRepo(t, dir)
	if _, _, err := repo.RecordedLanguage(); err == nil {
		t.Fatal("RecordedLanguage accepted aggregate on value_objects")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	repo := newRepo(t, dir)

	lang, err := vocab.NewUbiquitousLanguage(
		[]vocab.Entity{
			{Definition: vocab.Definition{Name: "Order", Definition: "A customer's request.", Aliases: []string{"Purchase Order"}}, Aggregate: true},
			{Definition: vocab.Definition{Name: "Customer", Definition: "Places Orders."}},
		},
		[]vocab.Definition{
			{Name: "Money", Definition: "A monetary amount."},
			{Name: "OrderID", Definition: "Order identity."},
		},
		[]vocab.Definition{
			{Name: "OrderMustHaveCustomer", Definition: "Every Order identifies its Customer."},
		},
		[]vocab.Definition{
			{Name: "OrderPlaced", Definition: "An Order was accepted."},
		},
	)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	if err := repo.Record(lang); err != nil {
		t.Fatalf("Record: %v", err)
	}

	got, found, err := repo.RecordedLanguage()
	if err != nil {
		t.Fatalf("RecordedLanguage: %v", err)
	}
	if !found {
		t.Fatal("found = false after Record")
	}
	assertEntities(t, "entities", got.Entities, lang.Entities)
	assertDefs(t, "value_objects", got.ValueObjects, lang.ValueObjects)
	assertDefs(t, "business_rules", got.BusinessRules, lang.BusinessRules)
	assertDefs(t, "events", got.Events, lang.Events)

	raw, err := os.ReadFile(repo.Path())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, schemaHint) {
		t.Fatalf("fresh file missing schema modeline:\n%s", text)
	}
	if !strings.Contains(text, "version: 1") {
		t.Fatalf("fresh file missing version:\n%s", text)
	}
}

func TestCommentPreservation(t *testing.T) {
	dir := t.TempDir()
	// Head/line/foot comments on Customer; Order is defined later via the model.
	initial := `version: 1

entities:
  # head comment on Customer
  - name: Customer # line comment on Customer
    definition: A person or organization that places Orders.
    # foot comment after Customer mapping

value_objects:
  - name: Money
    definition: A monetary amount.
`
	writeModel(t, dir, initial)
	repo := newRepo(t, dir)

	lang, found, err := repo.RecordedLanguage()
	if err != nil {
		t.Fatalf("RecordedLanguage: %v", err)
	}
	if !found {
		t.Fatal("expected found model")
	}

	lang, _, err = lang.Define(vocab.ConceptEntity, "Order", vocab.Change{
		SetDefinition:  true,
		DefinitionText: "A customer's request to purchase products.",
		SetAggregate:   true,
		Aggregate:      true,
	})
	if err != nil {
		t.Fatalf("Define: %v", err)
	}
	if err := repo.Record(lang); err != nil {
		t.Fatalf("Record: %v", err)
	}

	raw, err := os.ReadFile(repo.Path())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	text := string(raw)
	for _, want := range []string{
		"# head comment on Customer",
		"# line comment on Customer",
		"# foot comment after Customer mapping",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q after Record:\n%s", want, text)
		}
	}
	// Customer remains before Order (file order of surviving entries).
	customerAt := strings.Index(text, "name: Customer")
	orderAt := strings.Index(text, "name: Order")
	if customerAt < 0 || orderAt < 0 {
		t.Fatalf("missing Customer or Order after Record:\n%s", text)
	}
	if customerAt > orderAt {
		t.Fatalf("entry order changed; Customer should precede appended Order:\n%s", text)
	}
}

func TestClearDefinitionRemovesKey(t *testing.T) {
	dir := t.TempDir()
	writeModel(t, dir, `version: 1
entities:
  - name: Order
    definition: keep me briefly
`)
	repo := newRepo(t, dir)
	lang, _, err := repo.RecordedLanguage()
	if err != nil {
		t.Fatalf("RecordedLanguage: %v", err)
	}
	lang, _, err = lang.Define(vocab.ConceptEntity, "Order", vocab.Change{
		SetDefinition:  true,
		DefinitionText: "",
	})
	if err != nil {
		t.Fatalf("Define: %v", err)
	}
	if err := repo.Record(lang); err != nil {
		t.Fatalf("Record: %v", err)
	}
	raw := readModel(t, repo.Path())
	if strings.Contains(raw, "definition:") {
		t.Fatalf("definition key still present:\n%s", raw)
	}
	if !strings.Contains(raw, "name: Order") {
		t.Fatalf("Order missing:\n%s", raw)
	}
}

func TestClearAliasesRemovesKey(t *testing.T) {
	dir := t.TempDir()
	writeModel(t, dir, `version: 1
entities:
  - name: Order
    aliases:
      - Purchase Order
`)
	repo := newRepo(t, dir)
	lang, _, err := repo.RecordedLanguage()
	if err != nil {
		t.Fatalf("RecordedLanguage: %v", err)
	}
	lang, _, err = lang.Define(vocab.ConceptEntity, "Order", vocab.Change{
		ClearAliases: true,
	})
	if err != nil {
		t.Fatalf("Define: %v", err)
	}
	if err := repo.Record(lang); err != nil {
		t.Fatalf("Record: %v", err)
	}
	raw := readModel(t, repo.Path())
	if strings.Contains(raw, "aliases:") {
		t.Fatalf("aliases key still present:\n%s", raw)
	}
}

func TestAggregateKeyAddAndRemove(t *testing.T) {
	dir := t.TempDir()
	writeModel(t, dir, `version: 1
entities:
  - name: Order
    definition: A customer's request.
`)
	repo := newRepo(t, dir)
	lang, _, err := repo.RecordedLanguage()
	if err != nil {
		t.Fatalf("RecordedLanguage: %v", err)
	}

	lang, _, err = lang.Define(vocab.ConceptAggregate, "Order", vocab.Change{})
	if err != nil {
		t.Fatalf("Define aggregate: %v", err)
	}
	if err := repo.Record(lang); err != nil {
		t.Fatalf("Record designate: %v", err)
	}
	raw := readModel(t, repo.Path())
	if !strings.Contains(raw, "aggregate: true") {
		t.Fatalf("aggregate key not added:\n%s", raw)
	}

	lang, _, err = lang.Remove(vocab.ConceptAggregate, "Order")
	if err != nil {
		t.Fatalf("Remove aggregate: %v", err)
	}
	if err := repo.Record(lang); err != nil {
		t.Fatalf("Record clear aggregate: %v", err)
	}
	raw = readModel(t, repo.Path())
	if strings.Contains(raw, "aggregate:") {
		t.Fatalf("aggregate key still present:\n%s", raw)
	}
	if !strings.Contains(raw, "name: Order") {
		t.Fatalf("Order entity not preserved:\n%s", raw)
	}
}

func TestRemovalDeletesOnlyOneEntry(t *testing.T) {
	dir := t.TempDir()
	writeModel(t, dir, `version: 1
entities:
  - name: Order
  - name: Customer
  - name: Product
`)
	repo := newRepo(t, dir)
	lang, _, err := repo.RecordedLanguage()
	if err != nil {
		t.Fatalf("RecordedLanguage: %v", err)
	}
	lang, _, err = lang.Remove(vocab.ConceptEntity, "Customer")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := repo.Record(lang); err != nil {
		t.Fatalf("Record: %v", err)
	}
	raw := readModel(t, repo.Path())
	if strings.Contains(raw, "name: Customer") {
		t.Fatalf("Customer still present:\n%s", raw)
	}
	if !strings.Contains(raw, "name: Order") || !strings.Contains(raw, "name: Product") {
		t.Fatalf("sibling entities missing:\n%s", raw)
	}
}

func TestAtomicWriteLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	repo := newRepo(t, dir)
	lang, err := vocab.NewUbiquitousLanguage(
		[]vocab.Entity{{Definition: vocab.Definition{Name: "Order"}}},
		nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	if err := repo.Record(lang); err != nil {
		t.Fatalf("Record: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".ubiquitous-language-") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
	if _, found, err := repo.RecordedLanguage(); err != nil || !found {
		t.Fatalf("target does not parse after Record: found=%v err=%v", found, err)
	}
}

func TestPathBindsFileName(t *testing.T) {
	dir := t.TempDir()
	repo := newRepo(t, dir)
	want := filepath.Join(dir, vocab.UbiquitousLanguageFileName)
	got, err := filepath.EvalSymlinks(repo.Path())
	if err != nil {
		// Path may not exist yet; compare cleaned abs forms.
		if filepath.Clean(repo.Path()) != filepath.Clean(want) {
			// Abs may resolve differently; require suffix.
			if !strings.HasSuffix(repo.Path(), string(filepath.Separator)+vocab.UbiquitousLanguageFileName) {
				t.Fatalf("Path() = %q, want suffix %s", repo.Path(), vocab.UbiquitousLanguageFileName)
			}
		}
		return
	}
	wantAbs, _ := filepath.Abs(want)
	wantEval, err := filepath.EvalSymlinks(wantAbs)
	if err != nil {
		wantEval = wantAbs
	}
	if got != wantEval {
		t.Fatalf("Path() = %q, want %q", got, wantEval)
	}
}

const schemaHint = "# yaml-language-server: $schema=https://raw.githubusercontent.com/wixregiga/arclint/main/docs/ubiquitous-language.schema.json"

func newRepo(t *testing.T, root string) yamlvocab.Repository {
	t.Helper()
	repo, err := yamlvocab.NewRepository(root)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	return repo
}

func writeModel(t *testing.T, root, content string) {
	t.Helper()
	path := filepath.Join(root, vocab.UbiquitousLanguageFileName)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func readModel(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(raw)
}

func assertEntities(t *testing.T, section string, got, want []vocab.Entity) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: len=%d, want %d", section, len(got), len(want))
	}
	for i := range want {
		if got[i].Name != want[i].Name ||
			got[i].Definition.Definition != want[i].Definition.Definition ||
			got[i].Aggregate != want[i].Aggregate ||
			!stringSlicesEqual(got[i].Aliases, want[i].Aliases) {
			t.Fatalf("%s[%d] = %#v, want %#v", section, i, got[i], want[i])
		}
	}
}

func assertDefs(t *testing.T, section string, got, want []vocab.Definition) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: len=%d, want %d", section, len(got), len(want))
	}
	for i := range want {
		if got[i].Name != want[i].Name ||
			got[i].Definition != want[i].Definition ||
			!stringSlicesEqual(got[i].Aliases, want[i].Aliases) {
			t.Fatalf("%s[%d] = %#v, want %#v", section, i, got[i], want[i])
		}
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
