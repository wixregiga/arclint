package vocab_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

const ctx = "Ordering"

func emptyLang(t *testing.T) vocab.UbiquitousLanguage {
	t.Helper()
	l, err := vocab.NewUbiquitousLanguage(nil, nil)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	return l
}

func mustLang(t *testing.T, contexts []vocab.BoundedContext, relations []vocab.ContextRelation) vocab.UbiquitousLanguage {
	t.Helper()
	l, err := vocab.NewUbiquitousLanguage(contexts, relations)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	return l
}

func entity(name string, aggregate bool) vocab.Entity {
	return vocab.Entity{Definition: vocab.Definition{Name: name}, Aggregate: aggregate}
}

func TestNewUbiquitousLanguageRejectsEmptyContextName(t *testing.T) {
	_, err := vocab.NewUbiquitousLanguage(
		[]vocab.BoundedContext{{Name: "  "}},
		nil,
	)
	if err == nil {
		t.Fatal("expected empty-name error")
	}
	if !strings.Contains(err.Error(), "contexts") {
		t.Errorf("error %q should name section", err)
	}
}

func TestNewUbiquitousLanguageRejectsDuplicateContext(t *testing.T) {
	_, err := vocab.NewUbiquitousLanguage(
		[]vocab.BoundedContext{{Name: "A"}, {Name: "A"}},
		nil,
	)
	if err == nil {
		t.Fatal("expected duplicate context error")
	}
}

func TestNewUbiquitousLanguageRejectsDuplicateTermInSection(t *testing.T) {
	_, err := vocab.NewUbiquitousLanguage(
		[]vocab.BoundedContext{{
			Name:         ctx,
			ValueObjects: []vocab.Definition{{Name: "Money"}, {Name: "Money"}},
		}},
		nil,
	)
	if err == nil {
		t.Fatal("expected duplicate-name error")
	}
	if !strings.Contains(err.Error(), "value_objects") || !strings.Contains(err.Error(), "Money") {
		t.Errorf("error %q should name section and name", err)
	}
}

func TestNewUbiquitousLanguageRejectsBadRelation(t *testing.T) {
	_, err := vocab.NewUbiquitousLanguage(
		[]vocab.BoundedContext{{Name: "A"}},
		[]vocab.ContextRelation{{From: "A", To: "B", Kind: vocab.RelationPartnership}},
	)
	if err == nil || !strings.Contains(err.Error(), "B") {
		t.Fatalf("expected unknown context error, got %v", err)
	}
}

func TestNewUbiquitousLanguageAcceptsDesignatedEntity(t *testing.T) {
	l := mustLang(t, []vocab.BoundedContext{{
		Name:     ctx,
		Entities: []vocab.Entity{entity("Order", true)},
	}}, nil)
	if !l.Contexts[0].Entities[0].Aggregate {
		t.Fatal("expected Aggregate designation preserved")
	}
	if l.Contexts[0].Entities[0].Name != "Order" {
		t.Fatalf("promoted Name = %q, want Order", l.Contexts[0].Entities[0].Name)
	}
}

func TestUbiquitousLanguageEmpty(t *testing.T) {
	if !emptyLang(t).Empty() {
		t.Fatal("empty language should report Empty")
	}
	l := mustLang(t, []vocab.BoundedContext{{Name: ctx}}, nil)
	if l.Empty() {
		t.Fatal("language with a context should not report Empty")
	}
}

func TestCounts(t *testing.T) {
	l := mustLang(t,
		[]vocab.BoundedContext{
			{
				Name: ctx,
				Entities: []vocab.Entity{
					entity("Order", true),
					entity("Customer", false),
					entity("Product", false),
				},
				ValueObjects: []vocab.Definition{{Name: "Money"}, {Name: "OrderID"}, {Name: "SKU"}},
				Invariants: []vocab.Invariant{
					{Statement: "total = sum of lines", Owner: "Order"},
					{Statement: "must have customer", Owner: "Order"},
				},
				Events: []vocab.Definition{{Name: "OrderPlaced"}, {Name: "OrderShipped"}},
			},
			{Name: "Shipping"},
		},
		[]vocab.ContextRelation{{From: ctx, To: "Shipping", Kind: vocab.RelationCustomerSupplier}},
	)
	got := l.Counts()
	want := vocab.Counts{
		Contexts: 2, Entities: 3, Aggregates: 1, ValueObjects: 3, Invariants: 2, Events: 2, Relations: 1,
	}
	if got != want {
		t.Errorf("Counts() = %+v, want %+v", got, want)
	}
}

func TestDefineCreatesUnknownContext(t *testing.T) {
	l := emptyLang(t)
	l, res, err := l.Define(vocab.ConceptEntity, ctx, "Order", vocab.Change{
		SetDefinition:  true,
		DefinitionText: "A purchase request.",
	})
	if err != nil {
		t.Fatalf("define: %v", err)
	}
	if res.Outcome != vocab.OutcomeCreated {
		t.Fatalf("outcome = %q", res.Outcome)
	}
	if len(l.Contexts) != 1 || l.Contexts[0].Name != ctx {
		t.Fatalf("contexts = %+v", l.Contexts)
	}
}

func TestDefineBoundedContext(t *testing.T) {
	l := emptyLang(t)
	l, res, err := l.Define(vocab.ConceptBoundedContext, "Shipping", "Shipping", vocab.Change{})
	if err != nil {
		t.Fatalf("define context: %v", err)
	}
	if res.Outcome != vocab.OutcomeCreated {
		t.Fatalf("outcome = %q", res.Outcome)
	}
	_, res, err = l.Define(vocab.ConceptBoundedContext, "Shipping", "Shipping", vocab.Change{})
	if err != nil {
		t.Fatalf("redefine context: %v", err)
	}
	if res.Outcome != vocab.OutcomeUnchanged {
		t.Fatalf("redefine outcome = %q", res.Outcome)
	}
}

func TestDefineCreateUpdateClearUnchangedMatrix(t *testing.T) {
	concepts := []vocab.Concept{
		vocab.ConceptEntity,
		vocab.ConceptValueObject,
		vocab.ConceptDomainEvent,
	}
	for _, c := range concepts {
		t.Run(string(c), func(t *testing.T) {
			l := emptyLang(t)

			l, res, err := l.Define(c, ctx, "Term", vocab.Change{
				SetDefinition:  true,
				DefinitionText: "meaning",
			})
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if res.Outcome != vocab.OutcomeCreated {
				t.Fatalf("create outcome = %q, want created", res.Outcome)
			}
			if !reflect.DeepEqual(res.Changed, []string{"definition"}) {
				t.Fatalf("create Changed = %v, want [definition]", res.Changed)
			}
			def, ok := l.Find(c, ctx, "Term")
			if !ok || def.Definition != "meaning" {
				t.Fatalf("after create Find = %+v ok=%v", def, ok)
			}

			l, res, err = l.Define(c, ctx, "Term", vocab.Change{
				SetDefinition:  true,
				DefinitionText: "meaning",
			})
			if err != nil {
				t.Fatalf("unchanged: %v", err)
			}
			if res.Outcome != vocab.OutcomeUnchanged {
				t.Fatalf("unchanged outcome = %q", res.Outcome)
			}

			l, res, err = l.Define(c, ctx, "Term", vocab.Change{
				SetDefinition:  true,
				DefinitionText: "new meaning",
			})
			if err != nil {
				t.Fatalf("update: %v", err)
			}
			if res.Outcome != vocab.OutcomeUpdated {
				t.Fatalf("update outcome = %q", res.Outcome)
			}

			l, res, err = l.Define(c, ctx, "Term", vocab.Change{
				SetDefinition:  true,
				DefinitionText: "",
			})
			if err != nil {
				t.Fatalf("clear: %v", err)
			}
			def, _ = l.Find(c, ctx, "Term")
			if def.Definition != "" {
				t.Fatalf("after clear Definition = %q", def.Definition)
			}

			l, res, err = l.Define(c, ctx, "Term", vocab.Change{
				SetAliases: true,
				Aliases:    []string{"A", "B"},
			})
			if err != nil {
				t.Fatalf("aliases: %v", err)
			}
			if !reflect.DeepEqual(res.Changed, []string{"aliases"}) {
				t.Fatalf("aliases Changed = %v", res.Changed)
			}

			l, _, err = l.Define(c, ctx, "Term", vocab.Change{
				SetAliases: true,
				Aliases:    []string{"C"},
			})
			if err != nil {
				t.Fatalf("replace aliases: %v", err)
			}
			def, _ = l.Find(c, ctx, "Term")
			if !reflect.DeepEqual(def.Aliases, []string{"C"}) {
				t.Fatalf("replaced aliases = %v", def.Aliases)
			}

			l, res, err = l.Define(c, ctx, "Term", vocab.Change{ClearAliases: true})
			if err != nil {
				t.Fatalf("clear aliases: %v", err)
			}
			if !reflect.DeepEqual(res.Changed, []string{"aliases"}) {
				t.Fatalf("clear aliases Changed = %v", res.Changed)
			}

			_, res, err = l.Define(c, ctx, "Term", vocab.Change{})
			if err != nil {
				t.Fatalf("noop: %v", err)
			}
			if res.Outcome != vocab.OutcomeUnchanged {
				t.Fatalf("noop outcome = %q", res.Outcome)
			}
		})
	}
}

func TestDefineInvariantAndBusinessRule(t *testing.T) {
	l := emptyLang(t)

	// business_rule resolves to invariant; requires owner.
	_, _, err := l.Define(vocab.ConceptBusinessRule, ctx, "total = sum of lines", vocab.Change{})
	if err == nil {
		t.Fatal("expected owner required error")
	}

	l, res, err := l.Define(vocab.ConceptBusinessRule, ctx, "total = sum of lines", vocab.Change{
		Owner: "Order",
	})
	if err != nil {
		t.Fatalf("define business_rule: %v", err)
	}
	if res.Outcome != vocab.OutcomeCreated {
		t.Fatalf("outcome = %q", res.Outcome)
	}
	if !reflect.DeepEqual(res.Changed, []string{"owner"}) {
		t.Fatalf("Changed = %v", res.Changed)
	}
	inv, ok := l.FindInvariant(ctx, "total = sum of lines")
	if !ok || inv.Owner != "Order" {
		t.Fatalf("FindInvariant = %+v ok=%v", inv, ok)
	}

	// assertion is its own section and requires owner, id, and on
	_, _, err = l.Define(vocab.ConceptAssertion, ctx, "post: shipped implies paid", vocab.Change{
		Owner: "Order",
	})
	if err == nil {
		t.Fatal("expected assertion id required error")
	}
	l, res, err = l.Define(vocab.ConceptAssertion, ctx, "post: shipped implies paid", vocab.Change{
		Owner: "Order",
		ID:    "tiers-priced",
		On:    "Publish",
	})
	if err != nil {
		t.Fatalf("define assertion: %v", err)
	}
	if res.Outcome != vocab.OutcomeCreated {
		t.Fatalf("assertion outcome = %q", res.Outcome)
	}
	a, ok := l.FindAssertion(ctx, "post: shipped implies paid")
	if !ok || a.Owner != "Order" || a.ID != "tiers-priced" || a.On != "Publish" {
		t.Fatalf("FindAssertion = %+v ok=%v", a, ok)
	}

	// update owner
	l, res, err = l.Define(vocab.ConceptInvariant, ctx, "total = sum of lines", vocab.Change{
		SetOwner: true,
		Owner:    "OrderRoot",
	})
	if err != nil {
		t.Fatalf("update owner: %v", err)
	}
	if res.Outcome != vocab.OutcomeUpdated {
		t.Fatalf("update outcome = %q", res.Outcome)
	}
	inv, _ = l.FindInvariant(ctx, "total = sum of lines")
	if inv.Owner != "OrderRoot" {
		t.Fatalf("owner = %q", inv.Owner)
	}

	// unchanged
	_, res, err = l.Define(vocab.ConceptInvariant, ctx, "total = sum of lines", vocab.Change{
		SetOwner: true,
		Owner:    "OrderRoot",
	})
	if err != nil {
		t.Fatalf("unchanged: %v", err)
	}
	if res.Outcome != vocab.OutcomeUnchanged {
		t.Fatalf("unchanged outcome = %q", res.Outcome)
	}

	if got := l.ListInvariants(ctx); len(got) != 1 {
		t.Fatalf("ListInvariants len = %d", len(got))
	}
	if got := l.ListAssertions(ctx); len(got) != 1 {
		t.Fatalf("ListAssertions len = %d", len(got))
	}

	l, res, err = l.Define(vocab.ConceptSpecification, ctx, "PreferredCustomer", vocab.Change{
		SetDefinition:  true,
		DefinitionText: "An attendee the house treats as preferred.",
	})
	if err != nil {
		t.Fatalf("define specification: %v", err)
	}
	if res.Outcome != vocab.OutcomeCreated {
		t.Fatalf("specification outcome = %q", res.Outcome)
	}
	spec, ok := l.FindSpecification(ctx, "PreferredCustomer")
	if !ok || spec.Definition != "An attendee the house treats as preferred." {
		t.Fatalf("FindSpecification = %+v ok=%v", spec, ok)
	}
	l, res, err = l.Define(vocab.ConceptSpecification, ctx, "PreferredCustomer", vocab.Change{
		SetDefinition:  true,
		DefinitionText: "Updated preferred attendee.",
	})
	if err != nil {
		t.Fatalf("update specification: %v", err)
	}
	if res.Outcome != vocab.OutcomeUpdated {
		t.Fatalf("specification update outcome = %q", res.Outcome)
	}
	if got := l.ListSpecifications(ctx); len(got) != 1 || got[0].Definition != "Updated preferred attendee." {
		t.Fatalf("ListSpecifications = %+v", got)
	}
}

func TestDefineRejectsEmptyName(t *testing.T) {
	l := emptyLang(t)
	_, _, err := l.Define(vocab.ConceptEntity, ctx, "  ", vocab.Change{})
	if err == nil {
		t.Fatal("expected empty-name error")
	}
}

func TestDefineRejectsMutualExclusion(t *testing.T) {
	l := emptyLang(t)
	_, _, err := l.Define(vocab.ConceptEntity, ctx, "Order", vocab.Change{
		SetAliases:   true,
		Aliases:      []string{"PO"},
		ClearAliases: true,
	})
	if err == nil {
		t.Fatal("expected mutual-exclusion error")
	}
}

func TestDefineAggregateCreatesAndDesignates(t *testing.T) {
	l := emptyLang(t)

	l, res, err := l.Define(vocab.ConceptAggregate, ctx, "Order", vocab.Change{
		SetDefinition:  true,
		DefinitionText: "A customer's request to purchase products.",
	})
	if err != nil {
		t.Fatalf("define aggregate: %v", err)
	}
	if res.Outcome != vocab.OutcomeCreated {
		t.Fatalf("outcome = %q, want created", res.Outcome)
	}
	if !reflect.DeepEqual(res.Changed, []string{"definition", "aggregate"}) {
		t.Fatalf("Changed order = %v", res.Changed)
	}

	def, ok := l.Find(vocab.ConceptEntity, ctx, "Order")
	if !ok || def.Definition == "" {
		t.Fatalf("entity after aggregate define = %+v ok=%v", def, ok)
	}
	ent, ok := l.FindEntity(ctx, "Order")
	if !ok || !ent.Aggregate {
		t.Fatalf("FindEntity after aggregate define = %+v ok=%v", ent, ok)
	}
	if _, ok := l.Find(vocab.ConceptAggregate, ctx, "Order"); !ok {
		t.Fatal("Find(aggregate, Order) should succeed")
	}
	if _, ok := l.Find(vocab.ConceptAggregateRoot, ctx, "Order"); !ok {
		t.Fatal("Find(aggregate_root, Order) should succeed")
	}

	_, res, err = l.Define(vocab.ConceptAggregate, ctx, "Order", vocab.Change{})
	if err != nil {
		t.Fatalf("re-define aggregate: %v", err)
	}
	if res.Outcome != vocab.OutcomeUnchanged {
		t.Fatalf("re-define outcome = %q, want unchanged", res.Outcome)
	}

	l2 := emptyLang(t)
	l2, _, err = l2.Define(vocab.ConceptEntity, ctx, "Customer", vocab.Change{})
	if err != nil {
		t.Fatalf("define entity: %v", err)
	}
	l2, res, err = l2.Define(vocab.ConceptAggregateRoot, ctx, "Customer", vocab.Change{})
	if err != nil {
		t.Fatalf("designate: %v", err)
	}
	if res.Outcome != vocab.OutcomeUpdated {
		t.Fatalf("designate outcome = %q, want updated", res.Outcome)
	}
	if !reflect.DeepEqual(res.Changed, []string{"aggregate"}) {
		t.Fatalf("designate Changed = %v", res.Changed)
	}
}

func TestDefineEntityPreservesAggregateDesignation(t *testing.T) {
	l := emptyLang(t)
	l, _, err := l.Define(vocab.ConceptAggregate, ctx, "Order", vocab.Change{
		SetDefinition:  true,
		DefinitionText: "orig",
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	l, res, err := l.Define(vocab.ConceptEntity, ctx, "Order", vocab.Change{
		SetDefinition:  true,
		DefinitionText: "updated",
	})
	if err != nil {
		t.Fatalf("update entity: %v", err)
	}
	if res.Outcome != vocab.OutcomeUpdated {
		t.Fatalf("outcome = %q", res.Outcome)
	}
	if containsAll(res.Changed, "aggregate") {
		t.Fatalf("Changed should not include aggregate: %v", res.Changed)
	}
	ent, _ := l.FindEntity(ctx, "Order")
	if !ent.Aggregate {
		t.Fatal("entity define cleared Aggregate designation")
	}
	if ent.Definition.Definition != "updated" {
		t.Fatalf("Definition = %q", ent.Definition.Definition)
	}
}

func TestDefineEntitySetAggregateGuided(t *testing.T) {
	l := emptyLang(t)
	l, res, err := l.Define(vocab.ConceptEntity, ctx, "Order", vocab.Change{
		SetDefinition:  true,
		DefinitionText: "meaning",
		SetAggregate:   true,
		Aggregate:      true,
	})
	if err != nil {
		t.Fatalf("guided define: %v", err)
	}
	if res.Outcome != vocab.OutcomeCreated {
		t.Fatalf("outcome = %q", res.Outcome)
	}
	ent, _ := l.FindEntity(ctx, "Order")
	if !ent.Aggregate {
		t.Fatal("expected Aggregate designation")
	}

	l, res, err = l.Define(vocab.ConceptEntity, ctx, "Order", vocab.Change{
		SetAggregate: true,
		Aggregate:    false,
	})
	if err != nil {
		t.Fatalf("clear designation: %v", err)
	}
	if res.Outcome != vocab.OutcomeUpdated {
		t.Fatalf("outcome = %q", res.Outcome)
	}
	ent, _ = l.FindEntity(ctx, "Order")
	if ent.Aggregate {
		t.Fatal("expected Aggregate cleared")
	}
}

func TestDefineAppendsStableOrder(t *testing.T) {
	l := emptyLang(t)
	for _, name := range []string{"Customer", "Order", "Product"} {
		var err error
		l, _, err = l.Define(vocab.ConceptEntity, ctx, name, vocab.Change{})
		if err != nil {
			t.Fatalf("define %s: %v", name, err)
		}
	}
	got := l.List(vocab.ConceptEntity, ctx)
	if len(got) != 3 || got[0].Name != "Customer" || got[1].Name != "Order" || got[2].Name != "Product" {
		t.Fatalf("order = %v", namesOf(got))
	}
}

func TestDefineTrimsName(t *testing.T) {
	l := emptyLang(t)
	l, _, err := l.Define(vocab.ConceptEntity, "  "+ctx+"  ", "  Order  ", vocab.Change{})
	if err != nil {
		t.Fatalf("define: %v", err)
	}
	if _, ok := l.Find(vocab.ConceptEntity, ctx, "Order"); !ok {
		t.Fatal("expected trimmed name Order")
	}
}

func TestRemoveMatrix(t *testing.T) {
	l := emptyLang(t)
	var err error
	l, _, err = l.Define(vocab.ConceptEntity, ctx, "Order", vocab.Change{
		SetDefinition:  true,
		DefinitionText: "A customer's request to purchase products.",
		SetAggregate:   true,
		Aggregate:      true,
	})
	if err != nil {
		t.Fatalf("setup order: %v", err)
	}
	l, _, err = l.Define(vocab.ConceptValueObject, ctx, "Money", vocab.Change{})
	if err != nil {
		t.Fatalf("setup money: %v", err)
	}
	l, _, err = l.Define(vocab.ConceptBusinessRule, ctx, "OrderMustHaveCustomer", vocab.Change{Owner: "Order"})
	if err != nil {
		t.Fatalf("setup rule: %v", err)
	}
	l, _, err = l.Define(vocab.ConceptDomainEvent, ctx, "OrderPlaced", vocab.Change{})
	if err != nil {
		t.Fatalf("setup event: %v", err)
	}

	l, res, err := l.Remove(vocab.ConceptAggregate, ctx, "Order")
	if err != nil {
		t.Fatalf("remove aggregate: %v", err)
	}
	if !res.EntityPreserved {
		t.Fatal("EntityPreserved = false")
	}
	ent, ok := l.FindEntity(ctx, "Order")
	if !ok || ent.Aggregate {
		t.Fatalf("aggregate designation should be cleared: %+v ok=%v", ent, ok)
	}

	_, _, err = l.Remove(vocab.ConceptAggregate, ctx, "Order")
	if !errors.Is(err, vocab.ErrDefinitionNotFound) {
		t.Fatalf("second aggregate remove err = %v, want ErrDefinitionNotFound", err)
	}

	l, res, err = l.Remove(vocab.ConceptEntity, ctx, "Order")
	if err != nil {
		t.Fatalf("remove entity: %v", err)
	}
	if res.EntityPreserved {
		t.Fatal("EntityPreserved should be false for entity removal")
	}

	for _, c := range []struct {
		concept vocab.Concept
		name    string
	}{
		{vocab.ConceptValueObject, "Money"},
		{vocab.ConceptBusinessRule, "OrderMustHaveCustomer"},
		{vocab.ConceptDomainEvent, "OrderPlaced"},
	} {
		var rerr error
		l, _, rerr = l.Remove(c.concept, ctx, c.name)
		if rerr != nil {
			t.Fatalf("remove %s %s: %v", c.concept, c.name, rerr)
		}
		if _, ok := l.Find(c.concept, ctx, c.name); ok {
			t.Fatalf("%s %s still present", c.concept, c.name)
		}
	}

	_, _, err = l.Remove(vocab.ConceptEntity, ctx, "Missing")
	if !errors.Is(err, vocab.ErrDefinitionNotFound) {
		t.Fatalf("missing remove err = %v", err)
	}
	if !strings.Contains(err.Error(), `no entity named "Missing" is defined in the project domain model`) {
		t.Fatalf("error vocabulary = %v", err)
	}
}

func TestFindAndListExactMatching(t *testing.T) {
	l := mustLang(t,
		[]vocab.BoundedContext{{
			Name: ctx,
			Entities: []vocab.Entity{
				entity("Order", true),
				entity("Customer", false),
				entity("order", false),
			},
			ValueObjects: []vocab.Definition{{Name: "Money"}},
		}},
		nil,
	)

	if _, ok := l.Find(vocab.ConceptEntity, ctx, "ORDER"); ok {
		t.Fatal("Find should be case-sensitive")
	}
	if _, ok := l.Find(vocab.ConceptEntity, ctx, "Order "); ok {
		t.Fatal("Find should not trim")
	}
	if _, ok := l.Find(vocab.ConceptEntity, ctx, "Order"); !ok {
		t.Fatal("Find(Order) should succeed")
	}

	if _, ok := l.Find(vocab.ConceptAggregate, ctx, "Order"); !ok {
		t.Fatal("Find(aggregate, Order) should succeed")
	}
	if _, ok := l.Find(vocab.ConceptAggregate, ctx, "Customer"); ok {
		t.Fatal("Find(aggregate, Customer) should fail")
	}

	entities := l.List(vocab.ConceptEntity, ctx)
	if got := namesOf(entities); !reflect.DeepEqual(got, []string{"Order", "Customer", "order"}) {
		t.Fatalf("List(entity) = %v", got)
	}
	aggregates := l.List(vocab.ConceptAggregate, ctx)
	if got := namesOf(aggregates); !reflect.DeepEqual(got, []string{"Order"}) {
		t.Fatalf("List(aggregate) = %v", got)
	}
	vos := l.List(vocab.ConceptValueObject, ctx)
	if got := namesOf(vos); !reflect.DeepEqual(got, []string{"Money"}) {
		t.Fatalf("List(value_object) = %v", got)
	}
}

func TestFindEntityListEntitiesListAggregates(t *testing.T) {
	l := mustLang(t,
		[]vocab.BoundedContext{{
			Name: ctx,
			Entities: []vocab.Entity{
				{
					Definition: vocab.Definition{
						Name:       "Order",
						Definition: "A customer's request to purchase products.",
						Aliases:    []string{"PO"},
					},
					Aggregate: true,
				},
				entity("Customer", false),
				entity("Product", false),
			},
		}},
		nil,
	)

	ent, ok := l.FindEntity(ctx, "Order")
	if !ok {
		t.Fatal("FindEntity(Order) missing")
	}
	if !ent.Aggregate {
		t.Fatal("FindEntity should return Aggregate designation")
	}
	if ent.Name != "Order" {
		t.Fatalf("promoted Name = %q", ent.Name)
	}
	if !reflect.DeepEqual(ent.Aliases, []string{"PO"}) {
		t.Fatalf("promoted Aliases = %v", ent.Aliases)
	}

	all := l.ListEntities(ctx)
	if got := entityNamesOf(all); !reflect.DeepEqual(got, []string{"Order", "Customer", "Product"}) {
		t.Fatalf("ListEntities = %v", got)
	}
	aggs := l.ListAggregates(ctx)
	if got := entityNamesOf(aggs); !reflect.DeepEqual(got, []string{"Order"}) {
		t.Fatalf("ListAggregates = %v", got)
	}
}

func TestEntityEmbeddingPromotion(t *testing.T) {
	e := vocab.Entity{
		Definition: vocab.Definition{
			Name:       "Shipment",
			Definition: "Goods in transit.",
			Aliases:    []string{"Delivery"},
		},
		Aggregate: false,
	}
	if e.Name != "Shipment" {
		t.Errorf("Name promotion = %q", e.Name)
	}
	if e.Aliases[0] != "Delivery" {
		t.Errorf("Aliases promotion = %v", e.Aliases)
	}
	if e.Definition.Definition != "Goods in transit." {
		t.Errorf("nested Definition text = %q", e.Definition.Definition)
	}
}

func TestDefineAliasUnchangedElementWise(t *testing.T) {
	l := emptyLang(t)
	l, _, err := l.Define(vocab.ConceptEntity, ctx, "Order", vocab.Change{
		SetAliases: true,
		Aliases:    []string{"Purchase Order", "PO"},
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	_, res, err := l.Define(vocab.ConceptEntity, ctx, "Order", vocab.Change{
		SetAliases: true,
		Aliases:    []string{"Purchase Order", "PO"},
	})
	if err != nil {
		t.Fatalf("redefine: %v", err)
	}
	if res.Outcome != vocab.OutcomeUnchanged {
		t.Fatalf("outcome = %q, want unchanged", res.Outcome)
	}

	_, res, err = l.Define(vocab.ConceptEntity, ctx, "Order", vocab.Change{
		SetAliases: true,
		Aliases:    []string{"PO", "Purchase Order"},
	})
	if err != nil {
		t.Fatalf("reorder: %v", err)
	}
	if res.Outcome != vocab.OutcomeUpdated {
		t.Fatalf("reorder outcome = %q, want updated", res.Outcome)
	}
}

func TestNewUbiquitousLanguageRejectsValueObjectInvariantID(t *testing.T) {
	_, err := vocab.NewUbiquitousLanguage([]vocab.BoundedContext{{
		Name:         ctx,
		ValueObjects: []vocab.Definition{{Name: "Price", Definition: "Whole cents."}},
		Invariants:   []vocab.Invariant{{Statement: "A Price is never negative.", Owner: "Price", ID: "never-negative"}},
	}}, nil)
	if err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("expected value-object id forbidden, got %v", err)
	}
}

func TestNewUbiquitousLanguageRejectsAssertionWithoutOn(t *testing.T) {
	_, err := vocab.NewUbiquitousLanguage([]vocab.BoundedContext{{
		Name:     ctx,
		Entities: []vocab.Entity{entity("Order", true)},
		Assertions: []vocab.Assertion{{
			Statement: "Every line is priced.",
			Owner:     "Order",
			ID:        "lines-priced",
		}},
	}}, nil)
	if err == nil || !strings.Contains(err.Error(), "on must be non-empty") {
		t.Fatalf("expected assertion on required, got %v", err)
	}
}

func TestNewUbiquitousLanguageRejectsNameInValueObjectAndSpecification(t *testing.T) {
	_, err := vocab.NewUbiquitousLanguage([]vocab.BoundedContext{{
		Name:           ctx,
		ValueObjects:   []vocab.Definition{{Name: "Preferred", Definition: "A marking."}},
		Specifications: []vocab.Specification{{Name: "Preferred", Definition: "A named predicate."}},
	}}, nil)
	if err == nil || !strings.Contains(err.Error(), "both a value object and a specification") {
		t.Fatalf("expected name clash, got %v", err)
	}
}

func TestNewUbiquitousLanguageRejectsDuplicateContractIDs(t *testing.T) {
	_, err := vocab.NewUbiquitousLanguage([]vocab.BoundedContext{{
		Name:     ctx,
		Entities: []vocab.Entity{entity("Order", true)},
		Invariants: []vocab.Invariant{{
			Statement: "Lines never change.", Owner: "Order", ID: "lines-frozen",
		}},
		Assertions: []vocab.Assertion{{
			Statement: "Priced before place.", Owner: "Order", ID: "lines-frozen", On: "Place",
		}},
	}}, nil)
	if err == nil || !strings.Contains(err.Error(), "duplicate id") {
		t.Fatalf("expected duplicate id, got %v", err)
	}
}

func TestNewUbiquitousLanguageAcceptsClusterAndValueIntegrity(t *testing.T) {
	l := mustLang(t, []vocab.BoundedContext{{
		Name: ctx,
		Entities: []vocab.Entity{
			entity("Order", true),
		},
		ValueObjects: []vocab.Definition{{Name: "Price", Definition: "Whole cents."}},
		Invariants: []vocab.Invariant{
			{Statement: "A placed Order's lines never change.", Owner: "Order", ID: "lines-frozen"},
			{Statement: "A Price is never negative.", Owner: "Price"},
		},
	}}, nil)
	if l.Counts().Invariants != 2 {
		t.Fatalf("invariants = %d", l.Counts().Invariants)
	}
}

func namesOf(defs []vocab.Definition) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}

func entityNamesOf(entities []vocab.Entity) []string {
	out := make([]string, len(entities))
	for i, e := range entities {
		out[i] = e.Name
	}
	return out
}

func containsAll(got []string, want ...string) bool {
	set := make(map[string]struct{}, len(got))
	for _, g := range got {
		set[g] = struct{}{}
	}
	for _, w := range want {
		if _, ok := set[w]; !ok {
			return false
		}
	}
	return true
}
