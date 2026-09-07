package vocab_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

func def(text string) vocab.Change {
	return vocab.Change{SetDefinition: true, Definition: text}
}

func TestDefineLeavesTheReceiverUntouched(t *testing.T) {
	lang := mustTicketing(t)
	before := lang.Counts()
	next, res, err := lang.Define(vocab.ConceptDomainService, vocab.Locator{Context: "sales", Name: "Seating"}, def("Assigns seats."))
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != vocab.OutcomeCreated || res.Concept != vocab.ConceptDomainService {
		t.Errorf("result = %+v", res)
	}
	if lang.Counts() != before {
		t.Error("Define mutated its receiver")
	}
	if next.Counts().Services != before.Services+1 {
		t.Error("Define did not record the service in the result")
	}
}

func TestDefineContextCreatesAndUpdates(t *testing.T) {
	lang := mustTicketing(t)
	_, _, err := lang.Define(vocab.ConceptBoundedContext, vocab.Locator{Name: "billing"}, vocab.Change{})
	if err == nil || !strings.Contains(err.Error(), "needs definition") {
		t.Fatalf("creating without a definition: err = %v", err)
	}
	next, res, err := lang.Define(vocab.ConceptBoundedContext, vocab.Locator{Name: "billing"}, def("Charging customers."))
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != vocab.OutcomeCreated || strings.Join(res.Changed, ",") != "definition" {
		t.Errorf("result = %+v", res)
	}
	next, res, err = next.Define(vocab.ConceptBoundedContext, vocab.Locator{Name: "billing"}, def("Charging customers for what they ordered."))
	if err != nil {
		t.Fatal(err)
	}
	billing, _ := next.Context("billing")
	if res.Outcome != vocab.OutcomeUpdated || billing.Definition != "Charging customers for what they ordered." {
		t.Errorf("result = %+v, definition = %q", res, billing.Definition)
	}
	_, res, err = next.Define(vocab.ConceptBoundedContext, vocab.Locator{Name: "billing"}, def("Charging customers for what they ordered."))
	if err != nil || res.Outcome != vocab.OutcomeUnchanged || len(res.Changed) != 0 {
		t.Errorf("same definition again: res = %+v, err = %v", res, err)
	}
	_, _, err = next.Define(vocab.ConceptBoundedContext, vocab.Locator{Context: "sales", Name: "billing"}, def("x"))
	if err == nil || !strings.Contains(err.Error(), "located by its own name") {
		t.Errorf("context inside a context: err = %v", err)
	}
	_, _, err = next.Define(vocab.ConceptBoundedContext, vocab.Locator{Name: "Billing"}, def("x"))
	if err == nil || !strings.Contains(err.Error(), "bounded_context/named-once") {
		t.Errorf("badly spelled context name: err = %v", err)
	}
}

func TestDefineAggregateRequiresDefinitionAndIdentity(t *testing.T) {
	lang := mustTicketing(t)
	_, _, err := lang.Define(vocab.ConceptAggregateRoot, vocab.Locator{Context: "catalog", Name: "Venue"}, def("A place with halls."))
	if err == nil || !strings.Contains(err.Error(), "needs identity") {
		t.Fatalf("err = %v", err)
	}
	next, res, err := lang.Define(vocab.ConceptAggregateRoot, vocab.Locator{Context: "catalog", Name: "Venue"},
		vocab.Change{SetDefinition: true, Definition: "A place with halls.", SetIdentity: true, Identity: "VenueID", SetRepository: true, Repository: "VenueRepository"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Concept != vocab.ConceptAggregate || res.Outcome != vocab.OutcomeCreated || strings.Join(res.Changed, ",") != "definition,identity,repository" {
		t.Errorf("result = %+v", res)
	}
	catalog, _ := next.Context("catalog")
	if venue, ok := catalog.Aggregate("Venue"); !ok || venue.Identity != "VenueID" || venue.Repository != "VenueRepository" {
		t.Errorf("Venue = %+v", venue)
	}
	if id, ok := catalog.Term("VenueID"); !ok || !id.Implied {
		t.Error("the identity is an implied value object of the context")
	}
}

func TestDefineRefusesPropertiesTheBlockDoesNotRecord(t *testing.T) {
	lang := mustTicketing(t)
	_, _, err := lang.Define(vocab.ConceptValueObject, vocab.Locator{Context: "sales", Name: "Money"}, vocab.Change{SetIdentity: true, Identity: "x"})
	if err == nil || !strings.Contains(err.Error(), "a value object records no identity; it records") {
		t.Fatalf("err = %v", err)
	}
	_, _, err = lang.Define(vocab.ConceptDomainEvent, vocab.Locator{Context: "sales", Name: "OrderConfirmed"}, vocab.Change{SetOn: true, On: "Confirm"})
	if err == nil || !strings.Contains(err.Error(), "records no on") {
		t.Fatalf("err = %v", err)
	}
}

func TestDefineRefusesReclassingAName(t *testing.T) {
	lang := mustTicketing(t)
	_, _, err := lang.Define(vocab.ConceptValueObject, vocab.Locator{Context: "sales", Name: "Order"}, def("x"))
	if err == nil || !strings.Contains(err.Error(), "ubiquitous_language/one-meaning-per-name") || !strings.Contains(err.Error(), "as an aggregate") {
		t.Fatalf("err = %v", err)
	}
	_, _, err = lang.Define(vocab.ConceptAggregate, vocab.Locator{Context: "sales", Name: "OrderLine"}, def("x"))
	if err == nil || !strings.Contains(err.Error(), "as an entity of Order") {
		t.Fatalf("err = %v", err)
	}
	// An implied identity may be given an entry of its own.
	next, res, err := lang.Define(vocab.ConceptValueObject, vocab.Locator{Context: "sales", Name: "OrderID"}, def("The order's number."))
	if err != nil || res.Outcome != vocab.OutcomeCreated {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	sales, _ := next.Context("sales")
	if id, ok := sales.Term("OrderID"); !ok || id.Implied {
		t.Error("OrderID should now be a recorded value object")
	}
}

func TestDefineEntityUnderItsAggregate(t *testing.T) {
	lang := mustTicketing(t)
	_, _, err := lang.Define(vocab.ConceptEntity, vocab.Locator{Context: "sales", Name: "Ticket"}, def("An admission."))
	if err == nil || !strings.Contains(err.Error(), "names the aggregate it belongs to") {
		t.Fatalf("no owner: err = %v", err)
	}
	_, _, err = lang.Define(vocab.ConceptEntity, vocab.Locator{Context: "sales", Owner: "Money", Name: "Ticket"}, def("An admission."))
	if !errors.Is(err, vocab.ErrDefinitionNotFound) {
		t.Fatalf("owner that is not an aggregate: err = %v", err)
	}
	next, res, err := lang.Define(vocab.ConceptEntity, vocab.Locator{Context: "sales", Owner: "Order", Name: "Ticket"}, def("An admission."))
	if err != nil || res.Owner != "Order" || res.Outcome != vocab.OutcomeCreated {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	_, res, err = next.Define(vocab.ConceptEntity, vocab.Locator{Context: "sales", Name: "Ticket"}, vocab.Change{SetIdentity: true, Identity: "TicketNumber"})
	if err != nil || res.Owner != "Order" || strings.Join(res.Changed, ",") != "identity" {
		t.Fatalf("update without owner: res = %+v, err = %v", res, err)
	}
	withRefund, _, err := next.Define(vocab.ConceptAggregate, vocab.Locator{Context: "sales", Name: "Refund"},
		vocab.Change{SetDefinition: true, Definition: "Money returned.", SetIdentity: true, Identity: "RefundID"})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = withRefund.Define(vocab.ConceptEntity, vocab.Locator{Context: "sales", Owner: "Refund", Name: "Ticket"}, def("x"))
	if err == nil || !strings.Contains(err.Error(), "belongs to aggregate \"Order\"") {
		t.Fatalf("moving an entity: err = %v", err)
	}
}

func TestDefineInvariantFindsItsOwner(t *testing.T) {
	lang := mustTicketing(t)
	next, res, err := lang.Define(vocab.ConceptBusinessRule, vocab.Locator{Context: "sales", Owner: "Order", Name: "at-least-one-line"},
		vocab.Change{SetStatement: true, Statement: "An order has at least one line."})
	if err != nil || res.Concept != vocab.ConceptInvariant || res.Owner != "Order" || res.Outcome != vocab.OutcomeCreated {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	_, res, err = next.Define(vocab.ConceptInvariant, vocab.Locator{Context: "sales", Name: "never-negative"},
		vocab.Change{SetStatement: true, Statement: "Money is never below zero."})
	if err != nil || res.Owner != "Money" || res.Outcome != vocab.OutcomeUpdated {
		t.Fatalf("owner resolved from the key: res = %+v, err = %v", res, err)
	}
	_, _, err = next.Define(vocab.ConceptInvariant, vocab.Locator{Context: "sales", Name: "brand-new"}, vocab.Change{SetStatement: true, Statement: "x"})
	if !errors.Is(err, vocab.ErrChangeIncomplete) || !strings.Contains(err.Error(), "names the aggregate or value object that owns it") {
		t.Fatalf("new key without owner: err = %v", err)
	}
	_, _, err = next.Define(vocab.ConceptInvariant, vocab.Locator{Context: "sales", Owner: "OrderLine", Name: "priced"}, vocab.Change{SetStatement: true, Statement: "x"})
	if err == nil || !strings.Contains(err.Error(), "invariant/owned-by-aggregate-or-value-object") {
		t.Fatalf("entity owner: err = %v", err)
	}
	_, _, err = next.Define(vocab.ConceptInvariant, vocab.Locator{Context: "sales", Owner: "OrderID", Name: "positive"}, vocab.Change{SetStatement: true, Statement: "x"})
	if err == nil || !strings.Contains(err.Error(), "has no entry; record it as a value object") {
		t.Fatalf("implied identity owner: err = %v", err)
	}
	_, _, err = next.Define(vocab.ConceptInvariant, vocab.Locator{Context: "sales", Owner: "Order", Name: "validate"}, vocab.Change{SetStatement: true, Statement: "x"})
	if err == nil || !strings.Contains(err.Error(), "invariant/key-names-the-rule") {
		t.Fatalf("check-only key: err = %v", err)
	}
	// The same key under two owners must be located by owner.
	both, _, err := next.Define(vocab.ConceptInvariant, vocab.Locator{Context: "sales", Owner: "Money", Name: "at-least-one-line"}, vocab.Change{SetStatement: true, Statement: "x"})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = both.Define(vocab.ConceptInvariant, vocab.Locator{Context: "sales", Name: "at-least-one-line"}, vocab.Change{SetStatement: true, Statement: "y"})
	if err == nil || !strings.Contains(err.Error(), "under Order and Money; name the owner") {
		t.Fatalf("ambiguous owner: err = %v", err)
	}
}

func TestDefineAssertionIsOwnedByAnAggregate(t *testing.T) {
	lang := mustTicketing(t)
	next, res, err := lang.Define(vocab.ConceptBusinessRule, vocab.Locator{Context: "sales", Owner: "Order", Name: "seats-held"},
		vocab.Change{SetOn: true, On: "Confirm", SetStatement: true, Statement: "Every seat on the order is held."})
	if err != nil || res.Concept != vocab.ConceptAssertion || res.Owner != "Order" {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	sales, _ := next.Context("sales")
	order, _ := sales.Aggregate("Order")
	if as, ok := order.Assertion("seats-held"); !ok || as.On != "Confirm" {
		t.Errorf("assertion = %+v", as)
	}
	_, _, err = next.Define(vocab.ConceptAssertion, vocab.Locator{Context: "sales", Owner: "Money", Name: "rounded"}, vocab.Change{SetOn: true, On: "Add", SetStatement: true, Statement: "x"})
	if err == nil || !strings.Contains(err.Error(), "assertion/owned-by-an-aggregate") {
		t.Fatalf("value object owner: err = %v", err)
	}
	_, _, err = next.Define(vocab.ConceptAssertion, vocab.Locator{Context: "sales", Owner: "Order", Name: "total-is-sum-of-lines"}, vocab.Change{SetOn: true, On: "Add", SetStatement: true, Statement: "x"})
	if err == nil || !strings.Contains(err.Error(), "both an invariant and an assertion") {
		t.Fatalf("key clash: err = %v", err)
	}
	_, _, err = next.Define(vocab.ConceptAssertion, vocab.Locator{Context: "sales", Owner: "Order", Name: "late"}, vocab.Change{SetStatement: true, Statement: "x"})
	if err == nil || !strings.Contains(err.Error(), "needs on") {
		t.Fatalf("assertion without on: err = %v", err)
	}
}

func TestDefineEventServiceSpecificationQuestion(t *testing.T) {
	lang := mustTicketing(t)
	next, _, err := lang.Define(vocab.ConceptDomainEvent, vocab.Locator{Context: "sales", Name: "OrderCancelled"},
		vocab.Change{SetDefinition: true, Definition: "The order was cancelled.", SetRaisedBy: true, RaisedBy: "Order"})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = next.Define(vocab.ConceptDomainEvent, vocab.Locator{Context: "sales", Name: "OrderCancelled"}, vocab.Change{SetRaisedBy: true, RaisedBy: "Pricing"})
	if err == nil || !strings.Contains(err.Error(), "domain_event/raised-by-an-aggregate") {
		t.Fatalf("raised by a service: err = %v", err)
	}
	next, res, err := next.Define(vocab.ConceptDomainEvent, vocab.Locator{Context: "sales", Name: "OrderCancelled"}, vocab.Change{SetRaisedBy: true})
	if err != nil || strings.Join(res.Changed, ",") != "raised_by" {
		t.Fatalf("clearing raised_by: res = %+v, err = %v", res, err)
	}
	next, _, err = next.Define(vocab.ConceptSpecification, vocab.Locator{Context: "sales", Name: "SoldOut"}, def("No seat is left."))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = next.Define(vocab.ConceptQuestion, vocab.Locator{Context: "sales", Name: "resale"}, vocab.Change{})
	if err == nil || !strings.Contains(err.Error(), "needs text") {
		t.Fatalf("question without text: err = %v", err)
	}
	next, res, err = next.Define(vocab.ConceptQuestion, vocab.Locator{Context: "sales", Name: "resale"}, vocab.Change{SetText: true, Text: "May a ticket be resold?"})
	if err != nil || res.Outcome != vocab.OutcomeCreated || res.Concept != vocab.ConceptQuestion {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	_, _, err = next.Define(vocab.ConceptQuestion, vocab.Locator{Context: "sales", Owner: "Order", Name: "resale"}, vocab.Change{SetText: true, Text: "x"})
	if err == nil || !strings.Contains(err.Error(), "recorded under the context, not under") {
		t.Fatalf("question under an owner: err = %v", err)
	}
	sales, _ := next.Context("sales")
	if _, ok := sales.Question("resale"); !ok {
		t.Error("question not recorded")
	}
	if _, ok := sales.Specification("SoldOut"); !ok {
		t.Error("specification not recorded")
	}
}

func TestDefineNeedsAContext(t *testing.T) {
	lang := mustTicketing(t)
	_, _, err := lang.Define(vocab.ConceptValueObject, vocab.Locator{Name: "Seat"}, def("x"))
	if err == nil || !strings.Contains(err.Error(), "name the bounded context") {
		t.Fatalf("err = %v", err)
	}
	_, _, err = lang.Define(vocab.ConceptValueObject, vocab.Locator{Context: "billing", Name: "Seat"}, def("x"))
	if !errors.Is(err, vocab.ErrDefinitionNotFound) {
		t.Fatalf("err = %v", err)
	}
	_, _, err = lang.Define(vocab.ConceptValueObject, vocab.Locator{Context: "sales", Name: "  "}, def("x"))
	if err == nil || !strings.Contains(err.Error(), "a name is required") {
		t.Fatalf("err = %v", err)
	}
}

// A caller telling usage errors apart from the language's refusals
// keys on the two request sentinels: incomplete (say more) and
// misapplied (the concept cannot take that). A refused invariant and a
// missing entry carry neither.
func TestRequestErrorsCarryTheirSentinel(t *testing.T) {
	lang := mustTicketing(t)
	incomplete := []struct {
		name string
		c    vocab.Concept
		at   vocab.Locator
		ch   vocab.Change
	}{
		{"no name", vocab.ConceptValueObject, vocab.Locator{Context: "sales"}, def("x")},
		{"no context", vocab.ConceptValueObject, vocab.Locator{Name: "Seat"}, def("x")},
		{"required property unset", vocab.ConceptBoundedContext, vocab.Locator{Name: "billing"}, vocab.Change{}},
		{"entity without its aggregate", vocab.ConceptEntity, vocab.Locator{Context: "sales", Name: "Ticket"}, def("x")},
		{"invariant with no owner named", vocab.ConceptInvariant, vocab.Locator{Context: "sales", Name: "fresh-rule"}, vocab.Change{SetStatement: true, Statement: "x"}},
	}
	for _, c := range incomplete {
		_, _, err := lang.Define(c.c, c.at, c.ch)
		if !errors.Is(err, vocab.ErrChangeIncomplete) {
			t.Errorf("%s: err = %v, want ErrChangeIncomplete", c.name, err)
		}
		if errors.Is(err, vocab.ErrChangeMisapplied) {
			t.Errorf("%s: also misapplied", c.name)
		}
	}
	misapplied := []struct {
		name string
		c    vocab.Concept
		at   vocab.Locator
		ch   vocab.Change
	}{
		{"unrecorded property", vocab.ConceptValueObject, vocab.Locator{Context: "sales", Name: "Money"}, vocab.Change{SetIdentity: true, Identity: "x"}},
		{"owner on a direct concept", vocab.ConceptValueObject, vocab.Locator{Context: "sales", Owner: "Order", Name: "Money"}, def("x")},
		{"context inside a context", vocab.ConceptBoundedContext, vocab.Locator{Context: "sales", Name: "billing"}, def("x")},
	}
	for _, c := range misapplied {
		_, _, err := lang.Define(c.c, c.at, c.ch)
		if !errors.Is(err, vocab.ErrChangeMisapplied) {
			t.Errorf("%s: err = %v, want ErrChangeMisapplied", c.name, err)
		}
		if errors.Is(err, vocab.ErrChangeIncomplete) {
			t.Errorf("%s: also incomplete", c.name)
		}
	}
	_, _, err := lang.Define(vocab.ConceptValueObject, vocab.Locator{Context: "sales", Name: "Order"}, def("x"))
	if errors.Is(err, vocab.ErrChangeIncomplete) || errors.Is(err, vocab.ErrChangeMisapplied) || err == nil {
		t.Errorf("a refused invariant is neither request sentinel: %v", err)
	}
	_, _, err = lang.Remove(vocab.ConceptValueObject, vocab.Locator{Context: "sales", Name: "Nothing"})
	if !errors.Is(err, vocab.ErrDefinitionNotFound) || errors.Is(err, vocab.ErrChangeIncomplete) {
		t.Errorf("a missing entry is not found, nothing else: %v", err)
	}
	_, _, err = lang.Remove(vocab.ConceptValueObject, vocab.Locator{Context: "sales"})
	if !errors.Is(err, vocab.ErrChangeIncomplete) {
		t.Errorf("remove without a name: %v", err)
	}
}

func TestRemoveContextRemovesItsRelations(t *testing.T) {
	lang := mustTicketing(t)
	next, res, err := lang.Remove(vocab.ConceptBoundedContext, vocab.Locator{Name: "catalog"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Concept != vocab.ConceptBoundedContext || strings.Join(res.Also, ";") != "relation catalog -> sales (customer_supplier) removed" {
		t.Errorf("res = %+v", res)
	}
	if len(next.Relations) != 0 || len(next.Contexts) != 1 {
		t.Errorf("contexts = %d, relations = %d", len(next.Contexts), len(next.Relations))
	}
	if len(lang.Relations) != 1 {
		t.Error("Remove mutated its receiver")
	}
}

func TestRemoveAggregateCarriesItsMembers(t *testing.T) {
	lang := mustTicketing(t)
	next, res, err := lang.Remove(vocab.ConceptAggregateRoot, vocab.Locator{Context: "sales", Name: "Order"})
	if err != nil {
		t.Fatal(err)
	}
	want := "entity OrderLine removed with it;invariant total-is-sum-of-lines removed with it;assertion lines-priced removed with it;event OrderConfirmed no longer names what raises it"
	if res.Concept != vocab.ConceptAggregate || strings.Join(res.Also, ";") != want {
		t.Errorf("Also = %v", res.Also)
	}
	sales, _ := next.Context("sales")
	if ev, _ := sales.Event("OrderConfirmed"); ev.RaisedBy != "" {
		t.Errorf("event still raised by %q", ev.RaisedBy)
	}
	if _, _, ok := sales.Entity("OrderLine"); ok {
		t.Error("member entity survived")
	}
}

func TestRemoveValueObjectLeavesTheIdentityImplied(t *testing.T) {
	lang := mustTicketing(t)
	withID, _, err := lang.Define(vocab.ConceptValueObject, vocab.Locator{Context: "sales", Name: "OrderID"},
		vocab.Change{SetDefinition: true, Definition: "The order's number."})
	if err != nil {
		t.Fatal(err)
	}
	next, res, err := withID.Remove(vocab.ConceptValueObject, vocab.Locator{Context: "sales", Name: "OrderID"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(res.Also, ";") != "OrderID stays the identity of Order, implied" {
		t.Errorf("Also = %v", res.Also)
	}
	sales, _ := next.Context("sales")
	if id, ok := sales.Term("OrderID"); !ok || !id.Implied {
		t.Errorf("OrderID = %+v", id)
	}
	_, res, err = next.Remove(vocab.ConceptValueObject, vocab.Locator{Context: "sales", Name: "Money"})
	if err != nil || strings.Join(res.Also, ";") != "invariant never-negative removed with it" {
		t.Errorf("res = %+v, err = %v", res, err)
	}
}

func TestRemoveRulesAndMembers(t *testing.T) {
	lang := mustTicketing(t)
	next, res, err := lang.Remove(vocab.ConceptBusinessRule, vocab.Locator{Context: "sales", Name: "lines-priced"})
	if err != nil || res.Concept != vocab.ConceptAssertion || res.Owner != "Order" {
		t.Fatalf("assertion by key: res = %+v, err = %v", res, err)
	}
	next, res, err = next.Remove(vocab.ConceptInvariant, vocab.Locator{Context: "sales", Name: "never-negative"})
	if err != nil || res.Owner != "Money" {
		t.Fatalf("invariant by key: res = %+v, err = %v", res, err)
	}
	_, _, err = next.Remove(vocab.ConceptInvariant, vocab.Locator{Context: "sales", Owner: "Order", Name: "never-negative"})
	if !errors.Is(err, vocab.ErrDefinitionNotFound) {
		t.Fatalf("wrong owner: err = %v", err)
	}
	_, _, err = next.Remove(vocab.ConceptEntity, vocab.Locator{Context: "sales", Owner: "Event", Name: "OrderLine"})
	if err == nil || !strings.Contains(err.Error(), "belongs to aggregate \"Order\", not \"Event\"") {
		t.Fatalf("wrong owner: err = %v", err)
	}
	next, res, err = next.Remove(vocab.ConceptEntity, vocab.Locator{Context: "sales", Name: "OrderLine"})
	if err != nil || res.Owner != "Order" {
		t.Fatalf("entity: res = %+v, err = %v", res, err)
	}
	for _, c := range []vocab.Concept{vocab.ConceptDomainEvent, vocab.ConceptDomainService, vocab.ConceptSpecification, vocab.ConceptQuestion} {
		_, _, err := next.Remove(c, vocab.Locator{Context: "sales", Name: "nothing"})
		if !errors.Is(err, vocab.ErrDefinitionNotFound) {
			t.Errorf("%s: err = %v", c, err)
		}
	}
	next, _, err = next.Remove(vocab.ConceptQuestion, vocab.Locator{Context: "sales", Name: "refund-window"})
	if err != nil {
		t.Fatal(err)
	}
	if next.Counts().Questions != 0 {
		t.Error("question survived")
	}
}
