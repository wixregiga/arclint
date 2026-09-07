package application_test

import (
	"errors"
	"strings"
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

const (
	testProject = "shop"
	testContext = "ordering"
)

func mustLang(t *testing.T, contexts []vocab.BoundedContext) vocab.UbiquitousLanguage {
	t.Helper()
	lang, err := vocab.NewUbiquitousLanguage(testProject, "", contexts, nil)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	return lang
}

// orderContext records the ordering context with the given aggregates.
func orderContext(aggregates ...vocab.Aggregate) []vocab.BoundedContext {
	return []vocab.BoundedContext{{
		Name:       testContext,
		Definition: "Taking and fulfilling orders.",
		Aggregates: aggregates,
	}}
}

func orderAggregate() vocab.Aggregate {
	return vocab.Aggregate{
		Name:       "Order",
		Definition: "A purchase request.",
		Identity:   "OrderID",
		Entities:   []vocab.Entity{{Name: "OrderLine", Definition: "One product on an Order."}},
		Invariants: []vocab.Invariant{{Key: "has-lines", Statement: "An Order has at least one line."}},
		Assertions: []vocab.Assertion{{Key: "paid-in-full", On: "Ship", Statement: "An Order ships only once paid."}},
	}
}

func recorded(t *testing.T, contexts ...vocab.BoundedContext) *memoryKnowledge {
	t.Helper()
	return &memoryKnowledge{lang: mustLang(t, contexts), found: true}
}

func set(definition string) vocab.Change {
	return vocab.Change{SetDefinition: true, Definition: definition}
}

func TestInitDomainCreatesMissingFile(t *testing.T) {
	t.Parallel()
	repo := &memoryKnowledge{}
	uc, err := application.NewInitDomain(repo, testProject)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute("")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !out.Created || out.Source != vocab.UbiquitousLanguageFileName || out.Project != testProject {
		t.Fatalf("result = %+v", out)
	}
	if repo.saves != 1 || !repo.found || !repo.lang.Empty() || repo.lang.Project != testProject {
		t.Fatalf("repository after init = %+v", repo)
	}
}

func TestInitDomainNamesTheProjectItIsToldTo(t *testing.T) {
	t.Parallel()
	repo := &memoryKnowledge{}
	uc, err := application.NewInitDomain(repo, testProject)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute("  Box Office  ")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.Project != "Box Office" || repo.lang.Project != "Box Office" {
		t.Fatalf("project = %q recorded %q", out.Project, repo.lang.Project)
	}
}

func TestInitDomainLeavesExistingFileUnchanged(t *testing.T) {
	t.Parallel()
	repo := recorded(t, orderContext(orderAggregate())...)
	uc, err := application.NewInitDomain(repo, "other")
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute("")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.Created || repo.saves != 0 || out.Project != testProject {
		t.Fatalf("result/repository = %+v/%+v", out, repo)
	}
	if _, ok := repo.lang.Contexts[0].Aggregate("Order"); !ok {
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
			uc, err := application.NewInitDomain(tc.repo, testProject)
			if err != nil {
				t.Fatalf("construct: %v", err)
			}
			if _, err := uc.Execute(""); err == nil || err.Error() != tc.want {
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
	uc, err := application.NewGetDomainOverview(recorded(t, orderContext(orderAggregate())...))
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute()
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !out.Found || out.Matrix != nil {
		t.Fatalf("overview = %+v", out)
	}
	want := vocab.Counts{Contexts: 1, Aggregates: 1, Entities: 1, Invariants: 1, Assertions: 1}
	if out.Counts != want {
		t.Fatalf("Counts = %+v, want %+v", out.Counts, want)
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
	uc, err := application.NewListDomainDefinitions(recorded(t, orderContext(orderAggregate())...))
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

func TestListDomainDefinitionsUnknownContextIsUsage(t *testing.T) {
	t.Parallel()
	uc, err := application.NewListDomainDefinitions(recorded(t, orderContext(orderAggregate())...))
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	_, err = uc.Execute("", "billing")
	if !errors.Is(err, application.ErrDomainUsage) || !strings.Contains(err.Error(), "recorded: ordering") {
		t.Fatalf("error = %v, want usage naming the recorded contexts", err)
	}
}

func TestShowDomainDefinitionUsageAndNotFound(t *testing.T) {
	t.Parallel()
	uc, err := application.NewShowDomainDefinition(&memoryKnowledge{})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	_, err = uc.Execute("widget", "", "", "X")
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("unknown concept: %v", err)
	}
	_, err = uc.Execute("entity", "", "", "  ")
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("empty name: %v", err)
	}
	_, err = uc.Execute("entity", "", "", "Order")
	if !errors.Is(err, vocab.ErrDefinitionNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestShowDomainDefinitionFindsEachConcept(t *testing.T) {
	t.Parallel()
	ctx := orderContext(orderAggregate())[0]
	ctx.ValueObjects = []vocab.ValueObject{{Name: "Money", Definition: "An amount.", Invariants: []vocab.Invariant{{Key: "never-negative", Statement: "Money is never negative."}}}}
	ctx.Events = []vocab.DomainEvent{{Name: "OrderShipped", Definition: "An Order left the warehouse.", RaisedBy: "Order"}}
	ctx.Services = []vocab.DomainService{{Name: "Pricing", Definition: "Prices an Order."}}
	ctx.Specifications = []vocab.Specification{{Name: "LateOrder", Definition: "An Order past its promise."}}
	ctx.Questions = []vocab.Question{{Key: "partial-shipments", Text: "Can an Order ship in parts?"}}
	uc, err := application.NewShowDomainDefinition(recorded(t, ctx))
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	for _, tc := range []struct {
		concept, owner, name string
		want                 vocab.Concept
		wantOwner            string
	}{
		{concept: "bounded_context", name: testContext, want: vocab.ConceptBoundedContext},
		{concept: "aggregate", name: "Order", want: vocab.ConceptAggregate},
		{concept: "aggregate_root", name: "Order", want: vocab.ConceptAggregate},
		{concept: "entity", name: "OrderLine", want: vocab.ConceptEntity, wantOwner: "Order"},
		{concept: "value_object", name: "Money", want: vocab.ConceptValueObject},
		{concept: "invariant", name: "has-lines", want: vocab.ConceptInvariant, wantOwner: "Order"},
		{concept: "invariant", owner: "Money", name: "never-negative", want: vocab.ConceptInvariant, wantOwner: "Money"},
		{concept: "assertion", name: "paid-in-full", want: vocab.ConceptAssertion, wantOwner: "Order"},
		{concept: "business_rule", name: "paid-in-full", want: vocab.ConceptAssertion, wantOwner: "Order"},
		{concept: "business_rule", name: "has-lines", want: vocab.ConceptInvariant, wantOwner: "Order"},
		{concept: "domain_event", name: "OrderShipped", want: vocab.ConceptDomainEvent},
		{concept: "domain_service", name: "Pricing", want: vocab.ConceptDomainService},
		{concept: "specification", name: "LateOrder", want: vocab.ConceptSpecification},
		{concept: "question", name: "partial-shipments", want: vocab.ConceptQuestion},
	} {
		out, err := uc.Execute(tc.concept, "", tc.owner, tc.name)
		if err != nil {
			t.Fatalf("show %s %s: %v", tc.concept, tc.name, err)
		}
		if out.Concept != tc.want || out.Name != tc.name || out.Context != testContext || out.Owner != tc.wantOwner {
			t.Errorf("show %s %s = %+v, want concept %s owner %q", tc.concept, tc.name, out, tc.want, tc.wantOwner)
		}
	}
	view, err := uc.Execute("aggregate", "", "", "Order")
	if err != nil || view.Aggregate.Identity != "OrderID" || len(view.Aggregate.Entities) != 1 {
		t.Fatalf("aggregate view = %+v err=%v", view, err)
	}
	view, err = uc.Execute("entity", "", "Money", "OrderLine")
	if !errors.Is(err, vocab.ErrDefinitionNotFound) {
		t.Fatalf("entity under the wrong owner = %+v err=%v, want not found", view, err)
	}
	view, err = uc.Execute("value_object", "", "Order", "Money")
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("owner on a direct concept = %+v err=%v, want usage", view, err)
	}
}

func TestShowDomainDefinitionAsksForOwnerWhenKeyIsShared(t *testing.T) {
	t.Parallel()
	ctx := orderContext(orderAggregate())[0]
	ctx.ValueObjects = []vocab.ValueObject{{Name: "Money", Definition: "An amount.", Invariants: []vocab.Invariant{{Key: "has-lines", Statement: "Odd, but recorded."}}}}
	uc, err := application.NewShowDomainDefinition(recorded(t, ctx))
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	_, err = uc.Execute("invariant", "", "", "has-lines")
	if !errors.Is(err, application.ErrDomainUsage) || !strings.Contains(err.Error(), "--owner") {
		t.Fatalf("shared key without owner: %v", err)
	}
	out, err := uc.Execute("invariant", "", "Money", "has-lines")
	if err != nil || out.Owner != "Money" {
		t.Fatalf("shared key with owner = %+v err=%v", out, err)
	}
}

func TestShowRemoveAmbiguousAcrossContexts(t *testing.T) {
	t.Parallel()
	a := orderAggregate()
	repo := recorded(t,
		vocab.BoundedContext{Name: "a", Definition: "a", Aggregates: []vocab.Aggregate{a}},
		vocab.BoundedContext{Name: "b", Definition: "b", Aggregates: []vocab.Aggregate{a}},
	)
	show, err := application.NewShowDomainDefinition(repo)
	if err != nil {
		t.Fatalf("construct show: %v", err)
	}
	_, err = show.Execute("aggregate", "", "", "Order")
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("ambiguous show: %v", err)
	}
	view, err := show.Execute("aggregate", "b", "", "Order")
	if err != nil || view.Context != "b" {
		t.Fatalf("explicit context show = %+v err=%v", view, err)
	}
	remove, err := application.NewRemoveDomainDefinition(repo)
	if err != nil {
		t.Fatalf("construct remove: %v", err)
	}
	_, err = remove.Execute("aggregate", "", "", "Order")
	if !errors.Is(err, application.ErrDomainUsage) || repo.saves != 0 {
		t.Fatalf("ambiguous remove: %v saves=%d", err, repo.saves)
	}
	out, err := remove.Execute("aggregate", "a", "", "Order")
	if err != nil || out.Context != "a" || repo.saves != 1 {
		t.Fatalf("explicit context remove = %+v err=%v saves=%d", out, err, repo.saves)
	}
	if _, ok := repo.lang.Contexts[1].Aggregate("Order"); !ok {
		t.Fatal("the other context's aggregate was removed too")
	}
}

func TestDefineDomainDefinitionRecordsAFreshProject(t *testing.T) {
	t.Parallel()
	repo := &memoryKnowledge{}
	uc, err := application.NewDefineDomainDefinition(repo, testProject)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute(application.DefineDomainRequest{
		Concept: "bounded_context",
		Name:    testContext,
		Change:  set("Taking and fulfilling orders."),
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.Outcome != vocab.OutcomeCreated || out.Concept != vocab.ConceptBoundedContext || out.Context != testContext || out.Name != testContext {
		t.Fatalf("result = %+v", out)
	}
	if repo.saves != 1 || repo.lang.Project != testProject {
		t.Fatalf("repository = %+v, want one save under the project", repo)
	}
	if strings.Join(out.Changed, ",") != "definition" {
		t.Fatalf("Changed = %v", out.Changed)
	}
}

func TestDefineDomainDefinitionCreatesAndSaves(t *testing.T) {
	t.Parallel()
	repo := recorded(t, orderContext()...)
	uc, err := application.NewDefineDomainDefinition(repo, testProject)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute(application.DefineDomainRequest{
		Concept: "aggregate",
		Name:    "Order",
		Change: vocab.Change{
			SetDefinition: true, Definition: "A purchase request.",
			SetIdentity: true, Identity: "OrderID",
			SetAliases: true, Aliases: []string{"Purchase Order"},
		},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.Outcome != vocab.OutcomeCreated || out.Context != testContext || out.Concept != vocab.ConceptAggregate {
		t.Fatalf("result = %+v", out)
	}
	if strings.Join(out.Changed, ",") != "definition,identity,aliases" {
		t.Fatalf("Changed = %v", out.Changed)
	}
	if repo.saves != 1 {
		t.Fatalf("saves = %d, want 1", repo.saves)
	}
	got, ok := repo.lang.Contexts[0].Aggregate("Order")
	if !ok || got.Definition != "A purchase request." || got.Identity != "OrderID" || len(got.Aliases) != 1 {
		t.Fatalf("stored = %+v ok=%v", got, ok)
	}
}

func TestDefineDomainDefinitionNestsUnderTheOwner(t *testing.T) {
	t.Parallel()
	repo := recorded(t, orderContext(orderAggregate())...)
	uc, err := application.NewDefineDomainDefinition(repo, testProject)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute(application.DefineDomainRequest{
		Concept: "entity",
		Owner:   "Order",
		Name:    "Shipment",
		Change:  set("A parcel of an Order."),
	})
	if err != nil {
		t.Fatalf("entity: %v", err)
	}
	if out.Outcome != vocab.OutcomeCreated || out.Owner != "Order" || out.Concept != vocab.ConceptEntity {
		t.Fatalf("entity result = %+v", out)
	}
	out, err = uc.Execute(application.DefineDomainRequest{
		Concept: "business_rule",
		Owner:   "Order",
		Name:    "one-shipment-open",
		Change:  vocab.Change{SetStatement: true, Statement: "At most one Shipment is open."},
	})
	if err != nil {
		t.Fatalf("invariant: %v", err)
	}
	if out.Concept != vocab.ConceptInvariant || out.Owner != "Order" {
		t.Fatalf("business rule without on = %+v, want an invariant", out)
	}
	out, err = uc.Execute(application.DefineDomainRequest{
		Concept: "business_rule",
		Owner:   "Order",
		Name:    "lines-in-stock",
		Change:  vocab.Change{SetOn: true, On: "Ship", SetStatement: true, Statement: "Every line is in stock."},
	})
	if err != nil {
		t.Fatalf("assertion: %v", err)
	}
	if out.Concept != vocab.ConceptAssertion || out.Owner != "Order" {
		t.Fatalf("business rule with on = %+v, want an assertion", out)
	}
	if repo.saves != 3 {
		t.Fatalf("saves = %d, want 3", repo.saves)
	}
	agg, _ := repo.lang.Contexts[0].Aggregate("Order")
	if len(agg.Entities) != 2 || len(agg.Invariants) != 2 || len(agg.Assertions) != 2 {
		t.Fatalf("aggregate after nesting = %+v", agg)
	}
}

func TestDefineDomainDefinitionUpdatesInPlace(t *testing.T) {
	t.Parallel()
	repo := recorded(t, orderContext(orderAggregate())...)
	uc, err := application.NewDefineDomainDefinition(repo, testProject)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute(application.DefineDomainRequest{
		Concept: "invariant",
		Name:    "has-lines",
		Change:  vocab.Change{SetStatement: true, Statement: "An Order always has a line."},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.Outcome != vocab.OutcomeUpdated || out.Owner != "Order" || strings.Join(out.Changed, ",") != "statement" {
		t.Fatalf("result = %+v", out)
	}
	agg, _ := repo.lang.Contexts[0].Aggregate("Order")
	if agg.Invariants[0].Statement != "An Order always has a line." {
		t.Fatalf("stored invariant = %+v", agg.Invariants[0])
	}
}

func TestDefineDomainDefinitionRequiresContextWhenAmbiguous(t *testing.T) {
	t.Parallel()
	repo := recorded(t,
		vocab.BoundedContext{Name: "a", Definition: "a"},
		vocab.BoundedContext{Name: "b", Definition: "b"},
	)
	uc, err := application.NewDefineDomainDefinition(repo, testProject)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	_, err = uc.Execute(application.DefineDomainRequest{
		Concept: "value_object",
		Name:    "Money",
		Change:  set("An amount."),
	})
	if !errors.Is(err, application.ErrDomainUsage) || !strings.Contains(err.Error(), "--context") {
		t.Fatalf("error = %v, want ErrDomainUsage naming --context", err)
	}
}

func TestDefineDomainDefinitionNeedsARecordedContext(t *testing.T) {
	t.Parallel()
	repo := &memoryKnowledge{}
	uc, err := application.NewDefineDomainDefinition(repo, testProject)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	_, err = uc.Execute(application.DefineDomainRequest{
		Concept: "value_object",
		Name:    "Money",
		Change:  set("An amount."),
	})
	if !errors.Is(err, application.ErrDomainUsage) || !strings.Contains(err.Error(), "define bounded_context") {
		t.Fatalf("no context recorded: %v, want usage that says how to record one", err)
	}
	_, err = uc.Execute(application.DefineDomainRequest{
		Concept: "value_object",
		Context: "billing",
		Name:    "Money",
		Change:  set("An amount."),
	})
	if !errors.Is(err, application.ErrDomainUsage) || !strings.Contains(err.Error(), "define bounded_context billing") {
		t.Fatalf("unrecorded context: %v, want usage that says how to record it", err)
	}
	if repo.saves != 0 {
		t.Fatalf("saves = %d, want 0", repo.saves)
	}
}

func TestDefineDomainDefinitionUnchangedNoSave(t *testing.T) {
	t.Parallel()
	repo := recorded(t, orderContext(orderAggregate())...)
	uc, err := application.NewDefineDomainDefinition(repo, testProject)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute(application.DefineDomainRequest{
		Concept: "aggregate",
		Name:    "Order",
		Change:  set("A purchase request."),
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
	uc, err := application.NewDefineDomainDefinition(recorded(t, orderContext(orderAggregate())...), testProject)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	for _, tc := range []struct {
		name string
		req  application.DefineDomainRequest
	}{
		{name: "unknown concept", req: application.DefineDomainRequest{Concept: "nope", Name: "X"}},
		{name: "empty name", req: application.DefineDomainRequest{Concept: "entity", Name: " "}},
		{name: "required property missing", req: application.DefineDomainRequest{Concept: "value_object", Name: "Money"}},
		{name: "property the concept does not record", req: application.DefineDomainRequest{Concept: "value_object", Name: "Money", Change: vocab.Change{SetDefinition: true, Definition: "x", SetOn: true, On: "Ship"}}},
		{name: "entity without an aggregate", req: application.DefineDomainRequest{Concept: "entity", Name: "Shipment", Change: set("x")}},
		{name: "invariant without an owner", req: application.DefineDomainRequest{Concept: "invariant", Name: "brand-new", Change: vocab.Change{SetStatement: true, Statement: "x"}}},
		{name: "owner on a direct concept", req: application.DefineDomainRequest{Concept: "value_object", Owner: "Order", Name: "Money", Change: set("x")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := uc.Execute(tc.req)
			if !errors.Is(err, application.ErrDomainUsage) {
				t.Fatalf("error = %v, want ErrDomainUsage", err)
			}
		})
	}
}

// A change the recorded language refuses is the meta-model speaking,
// not a usage slip: it names the invariant and is not usage.
func TestDefineDomainDefinitionRefusalNamesTheInvariant(t *testing.T) {
	t.Parallel()
	repo := recorded(t, orderContext(orderAggregate())...)
	uc, err := application.NewDefineDomainDefinition(repo, testProject)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	_, err = uc.Execute(application.DefineDomainRequest{
		Concept: "value_object",
		Name:    "Order",
		Change:  set("The same name again."),
	})
	if err == nil || errors.Is(err, application.ErrDomainUsage) || !strings.Contains(err.Error(), "ubiquitous_language/one-meaning-per-name") {
		t.Fatalf("error = %v, want the meta-model invariant named", err)
	}
	if repo.saves != 0 {
		t.Fatalf("saves = %d, want 0", repo.saves)
	}
}

func TestRemoveDomainDefinitionMissingNoSave(t *testing.T) {
	t.Parallel()
	repo := &memoryKnowledge{}
	uc, err := application.NewRemoveDomainDefinition(repo)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	_, err = uc.Execute("entity", testContext, "", "Order")
	if !errors.Is(err, vocab.ErrDefinitionNotFound) {
		t.Fatalf("error = %v, want ErrDefinitionNotFound", err)
	}
	repo = recorded(t, orderContext(orderAggregate())...)
	uc, err = application.NewRemoveDomainDefinition(repo)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	_, err = uc.Execute("value_object", "", "", "Money")
	if !errors.Is(err, vocab.ErrDefinitionNotFound) {
		t.Fatalf("error = %v, want ErrDefinitionNotFound", err)
	}
	if repo.saves != 0 {
		t.Fatalf("saves = %d, want 0", repo.saves)
	}
}

func TestRemoveDomainDefinitionRemovesAndSaysWhatWentWithIt(t *testing.T) {
	t.Parallel()
	ctx := orderContext(orderAggregate())[0]
	ctx.Events = []vocab.DomainEvent{{Name: "OrderShipped", Definition: "An Order left.", RaisedBy: "Order"}}
	repo := recorded(t, ctx)
	uc, err := application.NewRemoveDomainDefinition(repo)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute("aggregate", "", "", "Order")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.Concept != vocab.ConceptAggregate || out.Context != testContext || out.Name != "Order" {
		t.Fatalf("result = %+v", out)
	}
	want := []string{
		"entity OrderLine removed with it",
		"invariant has-lines removed with it",
		"assertion paid-in-full removed with it",
		"event OrderShipped no longer names what raises it",
	}
	if strings.Join(out.Also, "|") != strings.Join(want, "|") {
		t.Fatalf("Also = %v, want %v", out.Also, want)
	}
	if repo.saves != 1 {
		t.Fatalf("saves = %d, want 1", repo.saves)
	}
	if _, ok := repo.lang.Contexts[0].Aggregate("Order"); ok {
		t.Fatal("aggregate still recorded")
	}
	if ev, _ := repo.lang.Contexts[0].Event("OrderShipped"); ev.RaisedBy != "" {
		t.Fatalf("event still names its raiser: %+v", ev)
	}
}

func TestRemoveDomainDefinitionOfNestedEntry(t *testing.T) {
	t.Parallel()
	repo := recorded(t, orderContext(orderAggregate())...)
	uc, err := application.NewRemoveDomainDefinition(repo)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	out, err := uc.Execute("business_rule", "", "", "paid-in-full")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.Concept != vocab.ConceptAssertion || out.Owner != "Order" || len(out.Also) != 0 {
		t.Fatalf("result = %+v", out)
	}
	agg, _ := repo.lang.Contexts[0].Aggregate("Order")
	if len(agg.Assertions) != 0 || len(agg.Invariants) != 1 {
		t.Fatalf("aggregate after removal = %+v", agg)
	}
}

func TestRemoveDomainDefinitionUsageError(t *testing.T) {
	t.Parallel()
	uc, err := application.NewRemoveDomainDefinition(&memoryKnowledge{})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	_, err = uc.Execute("widget", "", "", "X")
	if !errors.Is(err, application.ErrDomainUsage) {
		t.Fatalf("error = %v, want ErrDomainUsage", err)
	}
}

func TestDomainConstructorsRejectNil(t *testing.T) {
	t.Parallel()
	if _, err := application.NewInitDomain(nil, testProject); err == nil {
		t.Fatal("NewInitDomain(nil) accepted")
	}
	if _, err := application.NewInitDomain(&memoryKnowledge{}, " "); err == nil {
		t.Fatal("NewInitDomain without a project accepted")
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
	if _, err := application.NewDefineDomainDefinition(nil, testProject); err == nil {
		t.Fatal("NewDefineDomainDefinition(nil) accepted")
	}
	if _, err := application.NewDefineDomainDefinition(&memoryKnowledge{}, ""); err == nil {
		t.Fatal("NewDefineDomainDefinition without a project accepted")
	}
	if _, err := application.NewRemoveDomainDefinition(nil); err == nil {
		t.Fatal("NewRemoveDomainDefinition(nil) accepted")
	}
}
