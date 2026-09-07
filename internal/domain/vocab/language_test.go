package vocab_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// ticketing is a small but complete recorded domain: two contexts, an
// aggregate with a member entity, invariants under both owner kinds,
// an assertion, an event raised by the aggregate, a service, a
// specification, an open question, and one relation.
func ticketing() ([]vocab.BoundedContext, []vocab.ContextRelation) {
	contexts := []vocab.BoundedContext{
		{
			Name:       "sales",
			Definition: "Selling seats for events.",
			Aggregates: []vocab.Aggregate{{
				Name:       "Order",
				Definition: "A customer's purchase of seats for one event.",
				Identity:   "OrderID",
				Aliases:    []string{"booking"},
				Entities: []vocab.Entity{{
					Name:       "OrderLine",
					Definition: "One seat on the order.",
					Identity:   "LineNumber",
				}},
				Invariants: []vocab.Invariant{
					{Key: "total-is-sum-of-lines", Statement: "The order total equals the sum of its lines."},
				},
				Assertions: []vocab.Assertion{
					{Key: "lines-priced", On: "Confirm", Statement: "Every line carries a price once the order is confirmed."},
				},
				Repository: "OrderRepository",
			}},
			ValueObjects: []vocab.ValueObject{{
				Name:       "Money",
				Definition: "An amount in one currency.",
				Invariants: []vocab.Invariant{{Key: "never-negative", Statement: "Money is never negative."}},
			}},
			Events:         []vocab.DomainEvent{{Name: "OrderConfirmed", Definition: "The order was confirmed.", RaisedBy: "Order"}},
			Services:       []vocab.DomainService{{Name: "Pricing", Definition: "Prices an order against the event's tiers."}},
			Specifications: []vocab.Specification{{Name: "PreferredCustomer", Definition: "A customer entitled to early access."}},
			Questions:      []vocab.Question{{Key: "refund-window", Text: "How long after purchase may an order be refunded?"}},
		},
		{
			Name:       "catalog",
			Definition: "The events on sale and their seating.",
			Aggregates: []vocab.Aggregate{{
				Name:       "Event",
				Definition: "A performance with a seating plan.",
				Identity:   "EventID",
			}},
		},
	}
	relations := []vocab.ContextRelation{
		{From: "catalog", To: "sales", Kind: vocab.RelationCustomerSupplier, Description: "Sales reads seating from the catalog."},
	}
	return contexts, relations
}

func mustTicketing(t *testing.T) vocab.UbiquitousLanguage {
	t.Helper()
	contexts, relations := ticketing()
	lang, err := vocab.NewUbiquitousLanguage("boxoffice", "Selling tickets.", contexts, relations)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	return lang
}

func TestNewUbiquitousLanguageAcceptsACompleteDomain(t *testing.T) {
	lang := mustTicketing(t)
	if lang.Empty() {
		t.Fatal("a recorded domain reports Empty")
	}
	want := vocab.Counts{
		Contexts: 2, Aggregates: 2, Entities: 1, ValueObjects: 1, Invariants: 2, Assertions: 1,
		Specifications: 1, Events: 1, Services: 1, Questions: 1, Relations: 1,
	}
	if got := lang.Counts(); got != want {
		t.Errorf("Counts = %+v, want %+v", got, want)
	}
	if names := lang.ContextNames(); strings.Join(names, ",") != "sales,catalog" {
		t.Errorf("ContextNames = %v", names)
	}
	if _, ok := lang.Relation("sales", "catalog"); !ok {
		t.Error("Relation finds the pair in either direction")
	}
}

func TestNewUbiquitousLanguageCopiesItsInput(t *testing.T) {
	contexts, relations := ticketing()
	lang, err := vocab.NewUbiquitousLanguage("boxoffice", "", contexts, relations)
	if err != nil {
		t.Fatal(err)
	}
	contexts[0].Aggregates[0].Invariants[0].Key = "mutated"
	contexts[0].Aggregates[0].Aliases[0] = "mutated"
	relations[0].Kind = vocab.RelationSeparateWays
	sales, _ := lang.Context("sales")
	if sales.Aggregates[0].Invariants[0].Key == "mutated" || sales.Aggregates[0].Aliases[0] == "mutated" || lang.Relations[0].Kind == vocab.RelationSeparateWays {
		t.Fatal("the language shares slices with the input it was built from")
	}
}

func TestTermsIncludeImpliedIdentities(t *testing.T) {
	lang := mustTicketing(t)
	sales, _ := lang.Context("sales")
	var names []string
	for _, term := range sales.Terms() {
		names = append(names, string(term.Concept)+":"+term.Name)
	}
	want := "aggregate:Order,entity:OrderLine,value_object:Money,value_object:OrderID,value_object:LineNumber,domain_event:OrderConfirmed,domain_service:Pricing,specification:PreferredCustomer"
	if got := strings.Join(names, ","); got != want {
		t.Errorf("Terms = %s\nwant    %s", got, want)
	}
	id, ok := sales.Term("OrderID")
	if !ok || !id.Implied || id.Aggregate != "Order" {
		t.Errorf("OrderID term = %+v, want an implied value object of Order", id)
	}
	if _, ok := sales.ValueObject("OrderID"); ok {
		t.Error("an implied identity has no value object entry")
	}
	line, owner, ok := sales.Entity("OrderLine")
	if !ok || owner.Name != "Order" || line.Identity != "LineNumber" {
		t.Errorf("Entity(OrderLine) = %+v under %q", line, owner.Name)
	}
}

func TestOwnedInvariantsAndAssertions(t *testing.T) {
	lang := mustTicketing(t)
	sales, _ := lang.Context("sales")
	invs := sales.Invariants()
	if len(invs) != 2 || invs[0].Owner != "Order" || invs[0].OwnerConcept != vocab.ConceptAggregate || invs[1].Owner != "Money" || invs[1].OwnerConcept != vocab.ConceptValueObject {
		t.Errorf("Invariants = %+v", invs)
	}
	as := sales.Assertions()
	if len(as) != 1 || as[0].Owner != "Order" || as[0].Assertion.On != "Confirm" {
		t.Errorf("Assertions = %+v", as)
	}
}

func TestRelationInfluence(t *testing.T) {
	cases := []struct {
		kind         vocab.RelationKind
		downUp, upDn bool
	}{
		{vocab.RelationCustomerSupplier, true, false},
		{vocab.RelationConformist, true, false},
		{vocab.RelationAnticorruptionLayer, true, false},
		{vocab.RelationOpenHostService, true, false},
		{vocab.RelationPublishedLanguage, true, false},
		{vocab.RelationPartnership, true, true},
		{vocab.RelationSharedKernel, true, true},
		{vocab.RelationSeparateWays, false, false},
	}
	for _, c := range cases {
		r := vocab.ContextRelation{From: "up", To: "down", Kind: c.kind}
		if got := r.Influences("down", "up"); got != c.downUp {
			t.Errorf("%s: downstream imports upstream = %v, want %v", c.kind, got, c.downUp)
		}
		if got := r.Influences("up", "down"); got != c.upDn {
			t.Errorf("%s: upstream imports downstream = %v, want %v", c.kind, got, c.upDn)
		}
	}
}

// Each loader-level invariant of the meta-model rejects the one
// representation that breaks it, and the error names the invariant.
func TestNewUbiquitousLanguageAppliesTheLoaderInvariants(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation)
		wants string
	}{
		{"context name is not a zone name", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[1].Name = "Catalog"
			r[0].From = "Catalog"
			return c, r
		}, "bounded_context/named-once"},
		{"context recorded twice", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[1].Name = "sales"
			return c, nil
		}, "bounded_context/named-once"},
		{"context without definition", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[1].Definition = " "
			return c, r
		}, "ubiquitous_language/terms-carry-definitions"},
		{"one name two concepts", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[0].Events = append(c[0].Events, vocab.DomainEvent{Name: "Money", Definition: "Clashes."})
			return c, r
		}, "ubiquitous_language/one-meaning-per-name"},
		{"member entity named like an aggregate", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[0].Aggregates[0].Entities[0].Name = "Order"
			return c, r
		}, "ubiquitous_language/one-meaning-per-name"},
		{"aggregate without definition", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[0].Aggregates[0].Definition = ""
			return c, r
		}, "ubiquitous_language/terms-carry-definitions"},
		{"aggregate without identity", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[1].Aggregates[0].Identity = ""
			return c, r
		}, "aggregate/identity-recorded"},
		{"identity recorded as an event", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[0].Aggregates[0].Identity = "OrderConfirmed"
			return c, r
		}, "aggregate/identity-recorded"},
		{"member identity recorded as a service", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[0].Aggregates[0].Entities[0].Identity = "Pricing"
			return c, r
		}, "aggregate/identity-recorded"},
		{"entity without definition", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[0].Aggregates[0].Entities[0].Definition = ""
			return c, r
		}, "ubiquitous_language/terms-carry-definitions"},
		{"invariant key not kebab", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[0].ValueObjects[0].Invariants[0].Key = "NeverNegative"
			return c, r
		}, "invariant/keyed-within-owner"},
		{"invariant key names the check", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[0].Aggregates[0].Invariants[0].Key = "validate"
			return c, r
		}, "invariant/key-names-the-rule"},
		{"invariant recorded twice under one owner", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[0].Aggregates[0].Invariants = append(c[0].Aggregates[0].Invariants, c[0].Aggregates[0].Invariants[0])
			return c, r
		}, "invariant/keyed-within-owner"},
		{"invariant with empty statement", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[0].Aggregates[0].Invariants[0].Statement = ""
			return c, r
		}, "invariant/keyed-within-owner"},
		{"invariant and assertion share a key", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[0].Aggregates[0].Assertions[0].Key = "total-is-sum-of-lines"
			return c, r
		}, "invariant/keyed-within-owner"},
		{"assertion without operation", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[0].Aggregates[0].Assertions[0].On = ""
			return c, r
		}, "assertion/names-owner-operation-and-check"},
		{"assertion without statement", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[0].Aggregates[0].Assertions[0].Statement = ""
			return c, r
		}, "assertion/names-owner-operation-and-check"},
		{"event raised by a value object", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[0].Events[0].RaisedBy = "Money"
			return c, r
		}, "domain_event/raised-by-an-aggregate"},
		{"event raised by a stranger", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[0].Events[0].RaisedBy = "Event"
			return c, r
		}, "domain_event/raised-by-an-aggregate"},
		{"alias listed twice", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[0].Aggregates[0].Aliases = []string{"booking", "booking"}
			return c, r
		}, "listed twice"},
		{"question key not kebab", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[0].Questions[0].Key = "Refund Window"
			return c, r
		}, "is not lowercase kebab-case"},
		{"question without text", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			c[0].Questions[0].Text = ""
			return c, r
		}, "has no text"},
		{"relation names a stranger", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			r[0].To = "billing"
			return c, r
		}, "context_map/relations-name-recorded-contexts"},
		{"relation to itself", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			r[0].To = "catalog"
			return c, r
		}, "context_map/one-relation-per-pair"},
		{"pair related twice", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			r = append(r, vocab.ContextRelation{From: "sales", To: "catalog", Kind: vocab.RelationConformist})
			return c, r
		}, "context_map/one-relation-per-pair"},
		{"unpublished kind", func(c []vocab.BoundedContext, r []vocab.ContextRelation) ([]vocab.BoundedContext, []vocab.ContextRelation) {
			r[0].Kind = "shared-kernel"
			return c, r
		}, "context_relation/published-kind"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			contexts, relations := c.mut(ticketing())
			_, err := vocab.NewUbiquitousLanguage("boxoffice", "", contexts, relations)
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), c.wants) {
				t.Errorf("error %q does not name %q", err, c.wants)
			}
		})
	}
}

func TestNewUbiquitousLanguageRequiresAProject(t *testing.T) {
	_, err := vocab.NewUbiquitousLanguage(" ", "", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "project is empty") {
		t.Fatalf("err = %v", err)
	}
	lang, err := vocab.NewUbiquitousLanguage("boxoffice", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !lang.Empty() {
		t.Error("a project name alone is not a recorded domain")
	}
}

func TestLineIsReportedWhenKnown(t *testing.T) {
	contexts, relations := ticketing()
	contexts[0].Aggregates[0].Line = 12
	contexts[0].Aggregates[0].Identity = ""
	_, err := vocab.NewUbiquitousLanguage("boxoffice", "", contexts, relations)
	if err == nil || !strings.HasPrefix(err.Error(), "aggregate/identity-recorded: line 12: ") {
		t.Fatalf("err = %v", err)
	}
}

func TestErrDefinitionNotFoundIsWrapped(t *testing.T) {
	lang := mustTicketing(t)
	_, _, err := lang.Remove(vocab.ConceptAggregate, vocab.Locator{Context: "sales", Name: "Nothing"})
	if !errors.Is(err, vocab.ErrDefinitionNotFound) {
		t.Fatalf("err = %v, want ErrDefinitionNotFound", err)
	}
}
