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
}

func (m *memoryKnowledge) RecordedLanguage() (vocab.UbiquitousLanguage, bool, error) {
	if m.err != nil {
		return vocab.UbiquitousLanguage{}, false, m.err
	}
	return m.lang, m.found, nil
}

func (m *memoryKnowledge) Record(lang vocab.UbiquitousLanguage) error {
	if m.err != nil {
		return m.err
	}
	m.lang = lang
	m.found = true
	m.saves++
	return nil
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
	lang, err := vocab.NewUbiquitousLanguage(
		[]vocab.Entity{{Definition: vocab.Definition{Name: "Order"}, Aggregate: true}},
		nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
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
	if out.Counts.Entities != 1 || out.Counts.Aggregates != 1 {
		t.Fatalf("Counts = %+v", out.Counts)
	}
}

func TestListDomainDefinitionsUsageError(t *testing.T) {
	t.Parallel()
	uc, err := application.NewListDomainDefinitions(&memoryKnowledge{})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	_, err = uc.Execute("widgets")
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
	out, err := uc.Execute("")
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
	lang, err := vocab.NewUbiquitousLanguage(
		[]vocab.Entity{{Definition: vocab.Definition{Name: "Order"}, Aggregate: true}},
		nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	uc, err := application.NewListDomainDefinitions(&memoryKnowledge{lang: lang, found: true})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute("aggregates")
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
	_, err = uc.Execute("widget", "X")
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("unknown concept: %v", err)
	}
	_, err = uc.Execute("entity", "  ")
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("empty name: %v", err)
	}
	_, err = uc.Execute("entity", "Order")
	if !errors.Is(err, vocab.ErrDefinitionNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestShowDomainDefinitionFinds(t *testing.T) {
	t.Parallel()
	lang, err := vocab.NewUbiquitousLanguage(
		[]vocab.Entity{{
			Definition: vocab.Definition{Name: "Order", Definition: "A purchase request."},
			Aggregate:  true,
		}},
		nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	uc, err := application.NewShowDomainDefinition(&memoryKnowledge{lang: lang, found: true})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute("aggregate", "Order")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.Concept != vocab.ConceptAggregate || out.Definition.Name != "Order" {
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
	if repo.saves != 1 {
		t.Fatalf("saves = %d, want 1", repo.saves)
	}
	if len(out.Aliases) != 1 || out.Aliases[0] != "Purchase Order" {
		t.Fatalf("Aliases = %v", out.Aliases)
	}
	got, ok := repo.lang.Find(vocab.ConceptEntity, "Order")
	if !ok || got.Definition != def {
		t.Fatalf("stored = %+v ok=%v", got, ok)
	}
}

func TestDefineDomainDefinitionUnchangedNoSave(t *testing.T) {
	t.Parallel()
	lang, err := vocab.NewUbiquitousLanguage(
		[]vocab.Entity{{Definition: vocab.Definition{Name: "Order", Definition: "A purchase request."}}},
		nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
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
	_, err = uc.Execute(application.DefineDomainRequest{Concept: "nope", Name: "X"})
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("bad concept: %v", err)
	}
	_, err = uc.Execute(application.DefineDomainRequest{Concept: "entity", Name: ""})
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("empty name: %v", err)
	}
	_, err = uc.Execute(application.DefineDomainRequest{
		Concept:      "entity",
		Name:         "Order",
		Aliases:      []string{"PO"},
		ClearAliases: true,
	})
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("mutual exclusion: %v", err)
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
	got, ok := repo.lang.FindEntity("Order")
	if !ok || !got.Aggregate {
		t.Fatalf("stored aggregate = %+v ok=%v", got, ok)
	}
}

func TestRemoveDomainDefinitionMissingNoSave(t *testing.T) {
	t.Parallel()
	repo := &memoryKnowledge{}
	uc, err := application.NewRemoveDomainDefinition(repo)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	_, err = uc.Execute("entity", "Order")
	if !errors.Is(err, vocab.ErrDefinitionNotFound) {
		t.Fatalf("error = %v, want ErrDefinitionNotFound", err)
	}
	if repo.saves != 0 {
		t.Fatalf("saves = %d, want 0", repo.saves)
	}
}

func TestRemoveDomainDefinitionRemovesAndSaves(t *testing.T) {
	t.Parallel()
	lang, err := vocab.NewUbiquitousLanguage(
		[]vocab.Entity{{Definition: vocab.Definition{Name: "Order"}, Aggregate: true}},
		nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	repo := &memoryKnowledge{lang: lang, found: true}
	uc, err := application.NewRemoveDomainDefinition(repo)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute("aggregate", "Order")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !out.EntityPreserved {
		t.Fatal("expected EntityPreserved")
	}
	if repo.saves != 1 {
		t.Fatalf("saves = %d, want 1", repo.saves)
	}
	got, ok := repo.lang.FindEntity("Order")
	if !ok || got.Aggregate {
		t.Fatalf("entity after clear = %+v ok=%v", got, ok)
	}
}

func TestRemoveDomainDefinitionUsageError(t *testing.T) {
	t.Parallel()
	uc, err := application.NewRemoveDomainDefinition(&memoryKnowledge{})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	_, err = uc.Execute("widget", "X")
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("error = %v, want ErrDomainUsage", err)
	}
}

func TestDomainConstructorsRejectNil(t *testing.T) {
	t.Parallel()
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
