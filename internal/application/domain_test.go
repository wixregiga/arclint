package application_test

import (
	"errors"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// memoryKnowledge is an in-memory vocab.Repository that records
// Record calls for assertions.
type memoryKnowledge struct {
	lang  vocab.UbiquitousLanguage
	found bool
	saves int
	err   error
	save  error
}

func (m *memoryKnowledge) RecordedLanguage() (vocab.UbiquitousLanguage, bool, error) {
	if m.err != nil {
		return vocab.UbiquitousLanguage{}, false, m.err
	}
	return m.lang, m.found, nil
}

func (m *memoryKnowledge) Record(lang vocab.UbiquitousLanguage) error {
	if m.save != nil {
		return m.save
	}
	m.lang = lang
	m.found = true
	m.saves++
	return nil
}

const testContext = "Ordering"

func mustLang(t *testing.T, contexts []vocab.BoundedContext) vocab.UbiquitousLanguage {
	t.Helper()
	lang, err := vocab.NewUbiquitousLanguage(contexts, nil)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	return lang
}

func orderContext(entities ...vocab.Entity) []vocab.BoundedContext {
	return []vocab.BoundedContext{{
		Name:     testContext,
		Entities: entities,
	}}
}

func TestInitDomainCreatesMissingFile(t *testing.T) {
	t.Parallel()
	repo := &memoryKnowledge{}
	uc, err := application.NewInitDomain(repo)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute()
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !out.Created || out.Source != vocab.UbiquitousLanguageFileName {
		t.Fatalf("result = %+v", out)
	}
	if repo.saves != 1 || !repo.found || !repo.lang.Empty() {
		t.Fatalf("repository after init = %+v", repo)
	}
}

func TestInitDomainLeavesExistingFileUnchanged(t *testing.T) {
	t.Parallel()
	lang := mustLang(t, orderContext(vocab.Entity{Definition: vocab.Definition{Name: "Order"}}))
	repo := &memoryKnowledge{lang: lang, found: true}
	uc, err := application.NewInitDomain(repo)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute()
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.Created || repo.saves != 0 {
		t.Fatalf("result/repository = %+v/%+v", out, repo)
	}
	if _, ok := repo.lang.FindEntity(testContext, "Order"); !ok {
		t.Fatal("existing definition was not preserved")
	}
}

func TestInitDomainContainsRepositoryErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		repo *memoryKnowledge
		want string
	}{
		{name: "load", repo: &memoryKnowledge{err: errors.New("read failed")}, want: "load domain model: read failed"},
		{name: "save", repo: &memoryKnowledge{save: errors.New("write failed")}, want: "save domain model: write failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc, err := application.NewInitDomain(tc.repo)
			if err != nil {
				t.Fatalf("construct: %v", err)
			}
			if _, err := uc.Execute(); err == nil || err.Error() != tc.want {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestGetDomainOverviewMissingFile(t *testing.T) {
	t.Parallel()
	repo := &memoryKnowledge{}
	uc, err := application.NewGetDomainOverview(repo)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute()
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.Found {
		t.Fatal("expected Found=false for missing file")
	}
	if out.Source != vocab.UbiquitousLanguageFileName {
		t.Fatalf("Source = %q, want %q", out.Source, vocab.UbiquitousLanguageFileName)
	}
	if out.Counts != (vocab.Counts{}) {
		t.Fatalf("Counts = %+v, want zero", out.Counts)
	}
}

func TestGetDomainOverviewFound(t *testing.T) {
	t.Parallel()
	lang := mustLang(t, orderContext(vocab.Entity{
		Definition: vocab.Definition{Name: "Order"},
		Aggregate:  true,
	}))
	repo := &memoryKnowledge{lang: lang, found: true}
	uc, err := application.NewGetDomainOverview(repo)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute()
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !out.Found {
		t.Fatal("expected Found=true")
	}
	if out.Counts.Entities != 1 || out.Counts.Aggregates != 1 || out.Counts.Contexts != 1 {
		t.Fatalf("Counts = %+v", out.Counts)
	}
}

func TestListDomainDefinitionsUsageError(t *testing.T) {
	t.Parallel()
	uc, err := application.NewListDomainDefinitions(&memoryKnowledge{})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	_, err = uc.Execute("widgets", "")
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("error = %v, want ErrDomainUsage", err)
	}
}

func TestListDomainDefinitionsMissingFile(t *testing.T) {
	t.Parallel()
	uc, err := application.NewListDomainDefinitions(&memoryKnowledge{})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute("", "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.Found {
		t.Fatal("expected Found=false")
	}
	if out.Filtered {
		t.Fatal("empty listing must not be Filtered")
	}
}

func TestListDomainDefinitionsFiltered(t *testing.T) {
	t.Parallel()
	lang := mustLang(t, orderContext(vocab.Entity{
		Definition: vocab.Definition{Name: "Order"},
		Aggregate:  true,
	}))
	uc, err := application.NewListDomainDefinitions(&memoryKnowledge{lang: lang, found: true})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute("aggregates", "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !out.Filtered || out.Concept != vocab.ConceptAggregate {
		t.Fatalf("Filtered/Concept = %v/%q", out.Filtered, out.Concept)
	}
}

func TestShowDomainDefinitionUsageAndNotFound(t *testing.T) {
	t.Parallel()
	uc, err := application.NewShowDomainDefinition(&memoryKnowledge{})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	_, err = uc.Execute("widget", "", "X")
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("unknown concept: %v", err)
	}
	_, err = uc.Execute("entity", "", "  ")
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("empty name: %v", err)
	}
	_, err = uc.Execute("entity", "", "Order")
	if !errors.Is(err, vocab.ErrDefinitionNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestShowDomainDefinitionFinds(t *testing.T) {
	t.Parallel()
	lang := mustLang(t, orderContext(vocab.Entity{
		Definition: vocab.Definition{Name: "Order", Definition: "A purchase request."},
		Aggregate:  true,
	}))
	uc, err := application.NewShowDomainDefinition(&memoryKnowledge{lang: lang, found: true})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute("aggregate", "", "Order")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.Concept != vocab.ConceptAggregate || out.Definition.Name != "Order" || out.Context != testContext {
		t.Fatalf("view = %+v", out)
	}
	if !out.Aggregate {
		t.Fatal("aggregate concept match must set Aggregate")
	}
}

func TestDefineDomainDefinitionCreatesAndSaves(t *testing.T) {
	t.Parallel()
	repo := &memoryKnowledge{}
	uc, err := application.NewDefineDomainDefinition(repo)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	def := "A purchase request."
	out, err := uc.Execute(application.DefineDomainRequest{
		Concept:    "entity",
		Context:    testContext,
		Name:       "Order",
		Definition: &def,
		Aliases:    []string{"Purchase Order"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.Outcome != vocab.OutcomeCreated {
		t.Fatalf("Outcome = %q, want created", out.Outcome)
	}
	if out.Context != testContext {
		t.Fatalf("Context = %q", out.Context)
	}
	if repo.saves != 1 {
		t.Fatalf("saves = %d, want 1", repo.saves)
	}
	if len(out.Aliases) != 1 || out.Aliases[0] != "Purchase Order" {
		t.Fatalf("Aliases = %v", out.Aliases)
	}
	got, ok := repo.lang.Find(vocab.ConceptEntity, testContext, "Order")
	if !ok || got.Definition != def {
		t.Fatalf("stored = %+v ok=%v", got, ok)
	}
}

func TestDefineDomainDefinitionRequiresContextWhenAmbiguous(t *testing.T) {
	t.Parallel()
	lang := mustLang(t, []vocab.BoundedContext{
		{Name: "A"},
		{Name: "B"},
	})
	uc, err := application.NewDefineDomainDefinition(&memoryKnowledge{lang: lang, found: true})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	def := "x"
	_, err = uc.Execute(application.DefineDomainRequest{
		Concept:    "entity",
		Name:       "Order",
		Definition: &def,
	})
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("error = %v, want ErrDomainUsage", err)
	}
}

func TestDefineDomainDefinitionUnchangedNoSave(t *testing.T) {
	t.Parallel()
	lang := mustLang(t, orderContext(vocab.Entity{
		Definition: vocab.Definition{Name: "Order", Definition: "A purchase request."},
	}))
	repo := &memoryKnowledge{lang: lang, found: true}
	uc, err := application.NewDefineDomainDefinition(repo)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	def := "A purchase request."
	out, err := uc.Execute(application.DefineDomainRequest{
		Concept:    "entity",
		Name:       "Order",
		Definition: &def,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.Outcome != vocab.OutcomeUnchanged {
		t.Fatalf("Outcome = %q, want unchanged", out.Outcome)
	}
	if repo.saves != 0 {
		t.Fatalf("saves = %d, want 0", repo.saves)
	}
}

func TestDefineDomainDefinitionUsageErrors(t *testing.T) {
	t.Parallel()
	uc, err := application.NewDefineDomainDefinition(&memoryKnowledge{})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	_, err = uc.Execute(application.DefineDomainRequest{Concept: "nope", Name: "X", Context: testContext})
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("bad concept: %v", err)
	}
	_, err = uc.Execute(application.DefineDomainRequest{Concept: "entity", Name: "", Context: testContext})
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("empty name: %v", err)
	}
	def := "x"
	_, err = uc.Execute(application.DefineDomainRequest{
		Concept:      "entity",
		Context:      testContext,
		Name:         "Order",
		Definition:   &def,
		Aliases:      []string{"PO"},
		ClearAliases: true,
	})
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("mutual exclusion: %v", err)
	}
	_, err = uc.Execute(application.DefineDomainRequest{
		Concept: "entity",
		Context: testContext,
		Name:    "Order",
	})
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("missing definition at create: %v", err)
	}
	_, err = uc.Execute(application.DefineDomainRequest{
		Concept: "invariant",
		Context: testContext,
		Name:    "must hold",
	})
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("missing owner at create: %v", err)
	}
}

func TestDefineDomainDefinitionGuidedAggregate(t *testing.T) {
	t.Parallel()
	repo := &memoryKnowledge{}
	uc, err := application.NewDefineDomainDefinition(repo)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	agg := true
	def := "Consistency boundary for a purchase."
	out, err := uc.Execute(application.DefineDomainRequest{
		Concept:    "entity",
		Context:    testContext,
		Name:       "Order",
		Definition: &def,
		Aggregate:  &agg,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.Outcome != vocab.OutcomeCreated {
		t.Fatalf("Outcome = %q", out.Outcome)
	}
	got, ok := repo.lang.FindEntity(testContext, "Order")
	if !ok || !got.Aggregate {
		t.Fatalf("stored aggregate = %+v ok=%v", got, ok)
	}
}

func TestDefineDomainDefinitionInvariantAndBoundedContext(t *testing.T) {
	t.Parallel()
	repo := &memoryKnowledge{}
	uc, err := application.NewDefineDomainDefinition(repo)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute(application.DefineDomainRequest{
		Concept: "bounded_context",
		Name:    testContext,
	})
	if err != nil {
		t.Fatalf("define context: %v", err)
	}
	if out.Outcome != vocab.OutcomeCreated || out.Context != testContext {
		t.Fatalf("bounded_context result = %+v", out)
	}
	out, err = uc.Execute(application.DefineDomainRequest{
		Concept: "invariant",
		Context: testContext,
		Name:    "Every Order must identify its Customer.",
		Owner:   "Order",
	})
	if err != nil {
		t.Fatalf("define invariant: %v", err)
	}
	if out.Outcome != vocab.OutcomeCreated {
		t.Fatalf("invariant outcome = %q", out.Outcome)
	}
	inv, ok := repo.lang.FindInvariant(testContext, "Every Order must identify its Customer.")
	if !ok || inv.Owner != "Order" {
		t.Fatalf("stored invariant = %+v ok=%v", inv, ok)
	}
}

func TestRemoveDomainDefinitionMissingNoSave(t *testing.T) {
	t.Parallel()
	repo := &memoryKnowledge{}
	uc, err := application.NewRemoveDomainDefinition(repo)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	_, err = uc.Execute("entity", testContext, "Order")
	if !errors.Is(err, vocab.ErrDefinitionNotFound) {
		t.Fatalf("error = %v, want ErrDefinitionNotFound", err)
	}
	if repo.saves != 0 {
		t.Fatalf("saves = %d, want 0", repo.saves)
	}
}

func TestRemoveDomainDefinitionRemovesAndSaves(t *testing.T) {
	t.Parallel()
	lang := mustLang(t, orderContext(vocab.Entity{
		Definition: vocab.Definition{Name: "Order"},
		Aggregate:  true,
	}))
	repo := &memoryKnowledge{lang: lang, found: true}
	uc, err := application.NewRemoveDomainDefinition(repo)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute("aggregate", "", "Order")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !out.EntityPreserved || out.Context != testContext {
		t.Fatalf("result = %+v", out)
	}
	if repo.saves != 1 {
		t.Fatalf("saves = %d, want 1", repo.saves)
	}
	got, ok := repo.lang.FindEntity(testContext, "Order")
	if !ok || got.Aggregate {
		t.Fatalf("entity after clear = %+v ok=%v", got, ok)
	}
}

func TestShowRemoveAmbiguousAcrossContexts(t *testing.T) {
	t.Parallel()
	lang := mustLang(t, []vocab.BoundedContext{
		{Name: "A", Entities: []vocab.Entity{{Definition: vocab.Definition{Name: "Order", Definition: "a"}}}},
		{Name: "B", Entities: []vocab.Entity{{Definition: vocab.Definition{Name: "Order", Definition: "b"}}}},
	})
	show, err := application.NewShowDomainDefinition(&memoryKnowledge{lang: lang, found: true})
	if err != nil {
		t.Fatalf("construct show: %v", err)
	}
	_, err = show.Execute("entity", "", "Order")
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("ambiguous show: %v", err)
	}
	view, err := show.Execute("entity", "B", "Order")
	if err != nil || view.Context != "B" || view.Definition.Definition != "b" {
		t.Fatalf("explicit context show = %+v err=%v", view, err)
	}
}

func TestRemoveDomainDefinitionUsageError(t *testing.T) {
	t.Parallel()
	uc, err := application.NewRemoveDomainDefinition(&memoryKnowledge{})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	_, err = uc.Execute("widget", "", "X")
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("error = %v, want ErrDomainUsage", err)
	}
}

func TestDomainConstructorsRejectNil(t *testing.T) {
	t.Parallel()
	if _, err := application.NewInitDomain(nil); err == nil {
		t.Fatal("NewInitDomain(nil) accepted")
	}
	if _, err := application.NewGetDomainOverview(nil); err == nil {
		t.Fatal("NewGetDomainOverview(nil) accepted")
	}
	if _, err := application.NewListDomainDefinitions(nil); err == nil {
		t.Fatal("NewListDomainDefinitions(nil) accepted")
	}
	if _, err := application.NewShowDomainDefinition(nil); err == nil {
		t.Fatal("NewShowDomainDefinition(nil) accepted")
	}
	if _, err := application.NewDefineDomainDefinition(nil); err == nil {
		t.Fatal("NewDefineDomainDefinition(nil) accepted")
	}
	if _, err := application.NewRemoveDomainDefinition(nil); err == nil {
		t.Fatal("NewRemoveDomainDefinition(nil) accepted")
	}
}
