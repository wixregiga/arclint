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
	writeModel(t, dir, "version: 2\ncontexts: []\n")
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
	writeModel(t, dir, "contexts:\n  - name: Ordering\n")
	repo := newRepo(t, dir)
	_, _, err := repo.RecordedLanguage()
	if err == nil {
		t.Fatal("RecordedLanguage accepted missing version")
	}
	if !strings.Contains(err.Error(), "missing version") {
		t.Fatalf("error = %v, want missing version", err)
	}
}

func TestLoadRejectsUnknownKey(t *testing.T) {
	dir := t.TempDir()
	writeModel(t, dir, `version: 1
extra: true
contexts: []
`)
	repo := newRepo(t, dir)
	if _, _, err := repo.RecordedLanguage(); err == nil {
		t.Fatal("RecordedLanguage accepted unknown top-level key")
	}
}

func TestLoadRejectsEmptyAndDuplicateNames(t *testing.T) {
	t.Run("empty entity name", func(t *testing.T) {
		dir := t.TempDir()
		writeModel(t, dir, `version: 1
contexts:
  - name: Ordering
    entities:
      - name: ""
`)
		repo := newRepo(t, dir)
		if _, _, err := repo.RecordedLanguage(); err == nil {
			t.Fatal("RecordedLanguage accepted empty name")
		}
	})
	t.Run("duplicate entity name within context", func(t *testing.T) {
		dir := t.TempDir()
		writeModel(t, dir, `version: 1
contexts:
  - name: Ordering
    entities:
      - name: Order
      - name: Order
`)
		repo := newRepo(t, dir)
		if _, _, err := repo.RecordedLanguage(); err == nil {
			t.Fatal("RecordedLanguage accepted duplicate name")
		}
	})
	t.Run("duplicate context name", func(t *testing.T) {
		dir := t.TempDir()
		writeModel(t, dir, `version: 1
contexts:
  - name: Ordering
  - name: Ordering
`)
		repo := newRepo(t, dir)
		if _, _, err := repo.RecordedLanguage(); err == nil {
			t.Fatal("RecordedLanguage accepted duplicate context name")
		}
	})
}

func TestLoadRejectsAggregateOnValueObject(t *testing.T) {
	dir := t.TempDir()
	writeModel(t, dir, `version: 1
contexts:
  - name: Ordering
    value_objects:
      - name: Money
        aggregate: true
`)
	repo := newRepo(t, dir)
	if _, _, err := repo.RecordedLanguage(); err == nil {
		t.Fatal("RecordedLanguage accepted aggregate on value_objects")
	}
}

func TestLoadRejectsRelationToUndeclaredContext(t *testing.T) {
	dir := t.TempDir()
	writeModel(t, dir, `version: 1
contexts:
  - name: Ordering
relations:
  - from: Ordering
    to: Billing
    kind: customer_supplier
`)
	repo := newRepo(t, dir)
	if _, _, err := repo.RecordedLanguage(); err == nil {
		t.Fatal("RecordedLanguage accepted relation to undeclared context")
	}
}

func TestLoadRejectsInvalidRelationKind(t *testing.T) {
	dir := t.TempDir()
	writeModel(t, dir, `version: 1
contexts:
  - name: Ordering
  - name: Billing
relations:
  - from: Ordering
    to: Billing
    kind: not_a_kind
`)
	repo := newRepo(t, dir)
	if _, _, err := repo.RecordedLanguage(); err == nil {
		t.Fatal("RecordedLanguage accepted invalid relation kind")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	// Fresh file without local schema → remote modeline.
	repo := newRepo(t, dir)

	lang, err := vocab.NewUbiquitousLanguage(
		[]vocab.BoundedContext{
			{
				Name: "Ordering",
				Entities: []vocab.Entity{
					{Definition: vocab.Definition{Name: "Order", Definition: "A customer's request.", Aliases: []string{"Purchase Order"}}, Aggregate: true},
					{Definition: vocab.Definition{Name: "Customer", Definition: "Places Orders."}},
				},
				ValueObjects: []vocab.Definition{
					{Name: "Money", Definition: "A monetary amount."},
					{Name: "OrderID", Definition: "Order identity."},
				},
				Invariants: []vocab.Invariant{
					{Statement: "Every Order identifies its Customer.", Owner: "Order"},
				},
				Events: []vocab.Definition{
					{Name: "OrderPlaced", Definition: "An Order was accepted."},
				},
			},
			{
				Name: "Billing",
				Entities: []vocab.Entity{
					{Definition: vocab.Definition{Name: "Invoice", Definition: "A bill for payment."}},
				},
			},
		},
		[]vocab.ContextRelation{
			{From: "Ordering", To: "Billing", Kind: vocab.RelationKind("customer_supplier")},
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
	assertContexts(t, got.Contexts, lang.Contexts)
	assertRelations(t, got.Relations, lang.Relations)

	raw, err := os.ReadFile(repo.Path())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, remoteSchemaHint) {
		t.Fatalf("fresh file missing remote schema modeline:\n%s", text)
	}
	if !strings.Contains(text, "version: 1") {
		t.Fatalf("fresh file missing version:\n%s", text)
	}
	if !strings.Contains(text, "kind: customer_supplier") {
		t.Fatalf("fresh file missing relation kind:\n%s", text)
	}
}

func TestFreshModelineUsesLocalSchemaWhenPresent(t *testing.T) {
	dir := t.TempDir()
	schemaDir := filepath.Join(dir, ".agents", "skills", "domain-librarian")
	if err := os.MkdirAll(schemaDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(schemaDir, "library.schema.json"), []byte(`{"$id":"x"}`), 0o600); err != nil {
		t.Fatalf("WriteFile schema: %v", err)
	}
	repo := newRepo(t, dir)
	lang, err := vocab.NewUbiquitousLanguage(
		[]vocab.BoundedContext{{Name: "Ordering"}},
		nil,
	)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	if err := repo.Record(lang); err != nil {
		t.Fatalf("Record: %v", err)
	}
	raw := readModel(t, repo.Path())
	if !strings.Contains(raw, localSchemaHint) {
		t.Fatalf("fresh file missing local schema modeline:\n%s", raw)
	}
	if strings.Contains(raw, remoteSchemaHint) {
		t.Fatalf("fresh file used remote modeline despite local schema:\n%s", raw)
	}
}

func TestCommentPreservation(t *testing.T) {
	dir := t.TempDir()
	initial := `version: 1

contexts:
  # head comment on Ordering
  - name: Ordering # line comment on Ordering
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
	if len(lang.Contexts) != 1 {
		t.Fatalf("contexts = %d, want 1", len(lang.Contexts))
	}

	// Append Order entity while leaving Customer untouched.
	c := lang.Contexts[0]
	c.Entities = append(c.Entities, vocab.Entity{
		Definition: vocab.Definition{
			Name:       "Order",
			Definition: "A customer's request to purchase products.",
		},
		Aggregate: true,
	})
	lang, err = vocab.NewUbiquitousLanguage([]vocab.BoundedContext{c}, lang.Relations)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
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
		"# head comment on Ordering",
		"# line comment on Ordering",
		"# head comment on Customer",
		"# line comment on Customer",
		"# foot comment after Customer mapping",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q after Record:\n%s", want, text)
		}
	}
	// Customer remains before Order (file order of surviving entries).
	// "name: Order\n" avoids matching the "name: Ordering" context line.
	customerAt := strings.Index(text, "name: Customer")
	orderAt := strings.Index(text, "name: Order\n")
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
contexts:
  - name: Ordering
    entities:
      - name: Order
        definition: keep me briefly
`)
	repo := newRepo(t, dir)
	lang, _, err := repo.RecordedLanguage()
	if err != nil {
		t.Fatalf("RecordedLanguage: %v", err)
	}
	c := lang.Contexts[0]
	c.Entities[0].Definition.Definition = ""
	lang, err = vocab.NewUbiquitousLanguage([]vocab.BoundedContext{c}, nil)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
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
contexts:
  - name: Ordering
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
	c := lang.Contexts[0]
	c.Entities[0].Aliases = nil
	lang, err = vocab.NewUbiquitousLanguage([]vocab.BoundedContext{c}, nil)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
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
contexts:
  - name: Ordering
    entities:
      - name: Order
        definition: A customer's request.
`)
	repo := newRepo(t, dir)
	lang, _, err := repo.RecordedLanguage()
	if err != nil {
		t.Fatalf("RecordedLanguage: %v", err)
	}

	c := lang.Contexts[0]
	c.Entities[0].Aggregate = true
	lang, err = vocab.NewUbiquitousLanguage([]vocab.BoundedContext{c}, nil)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage designate: %v", err)
	}
	if err := repo.Record(lang); err != nil {
		t.Fatalf("Record designate: %v", err)
	}
	raw := readModel(t, repo.Path())
	if !strings.Contains(raw, "aggregate: true") {
		t.Fatalf("aggregate key not added:\n%s", raw)
	}

	c = lang.Contexts[0]
	c.Entities[0].Aggregate = false
	lang, err = vocab.NewUbiquitousLanguage([]vocab.BoundedContext{c}, nil)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage clear: %v", err)
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
contexts:
  - name: Ordering
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
	c := lang.Contexts[0]
	kept := make([]vocab.Entity, 0, 2)
	for _, e := range c.Entities {
		if e.Name == "Customer" {
			continue
		}
		kept = append(kept, e)
	}
	c.Entities = kept
	lang, err = vocab.NewUbiquitousLanguage([]vocab.BoundedContext{c}, nil)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
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

func TestInvariantAndRelationSurgery(t *testing.T) {
	dir := t.TempDir()
	writeModel(t, dir, `version: 1
contexts:
  - name: Ordering
    entities:
      - name: Order
        definition: A customer's request.
    invariants:
      # keep this invariant
      - statement: Every Order identifies its Customer.
        owner: Order
  - name: Billing
    entities:
      - name: Invoice
        definition: A bill.
relations:
  # keep this edge
  - from: Ordering
    to: Billing
    kind: customer_supplier
`)
	repo := newRepo(t, dir)
	lang, _, err := repo.RecordedLanguage()
	if err != nil {
		t.Fatalf("RecordedLanguage: %v", err)
	}
	// Add a second invariant and flip relation kind; preserve comments on survivors.
	var ordering vocab.BoundedContext
	var billing vocab.BoundedContext
	for _, c := range lang.Contexts {
		switch c.Name {
		case "Ordering":
			ordering = c
		case "Billing":
			billing = c
		}
	}
	ordering.Invariants = append(ordering.Invariants, vocab.Invariant{
		Statement: "Order total is non-negative.",
		Owner:     "Order",
	})
	lang, err = vocab.NewUbiquitousLanguage(
		[]vocab.BoundedContext{ordering, billing},
		[]vocab.ContextRelation{
			{From: "Ordering", To: "Billing", Kind: vocab.RelationKind("conformist")},
		},
	)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	if err := repo.Record(lang); err != nil {
		t.Fatalf("Record: %v", err)
	}
	raw := readModel(t, repo.Path())
	if !strings.Contains(raw, "# keep this invariant") {
		t.Fatalf("invariant comment lost:\n%s", raw)
	}
	if !strings.Contains(raw, "# keep this edge") {
		t.Fatalf("relation comment lost:\n%s", raw)
	}
	if !strings.Contains(raw, "Order total is non-negative.") {
		t.Fatalf("new invariant missing:\n%s", raw)
	}
	if !strings.Contains(raw, "kind: conformist") {
		t.Fatalf("relation kind not updated:\n%s", raw)
	}
}

func TestAtomicWriteLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	repo := newRepo(t, dir)
	lang, err := vocab.NewUbiquitousLanguage(
		[]vocab.BoundedContext{
			{Name: "Ordering", Entities: []vocab.Entity{{Definition: vocab.Definition{Name: "Order"}}}},
		},
		nil,
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

const (
	localSchemaHint  = "# yaml-language-server: $schema=.agents/skills/domain-librarian/library.schema.json"
	remoteSchemaHint = "# yaml-language-server: $schema=https://raw.githubusercontent.com/wixregiga/arclint/main/.agents/skills/domain-librarian/library.schema.json"
)

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

func assertContexts(t *testing.T, got, want []vocab.BoundedContext) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("contexts: len=%d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Name != want[i].Name {
			t.Fatalf("contexts[%d].Name = %q, want %q", i, got[i].Name, want[i].Name)
		}
		assertEntities(t, want[i].Name+".entities", got[i].Entities, want[i].Entities)
		assertDefs(t, want[i].Name+".value_objects", got[i].ValueObjects, want[i].ValueObjects)
		assertInvariants(t, want[i].Name+".invariants", got[i].Invariants, want[i].Invariants)
		assertDefs(t, want[i].Name+".events", got[i].Events, want[i].Events)
	}
}

func assertRelations(t *testing.T, got, want []vocab.ContextRelation) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("relations: len=%d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].From != want[i].From || got[i].To != want[i].To || got[i].Kind != want[i].Kind {
			t.Fatalf("relations[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
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

func assertInvariants(t *testing.T, section string, got, want []vocab.Invariant) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: len=%d, want %d", section, len(got), len(want))
	}
	for i := range want {
		if got[i].Statement != want[i].Statement || got[i].Owner != want[i].Owner {
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
