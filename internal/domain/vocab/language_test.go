package vocab_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

func emptyLang(t *testing.T) vocab.UbiquitousLanguage {
	t.Helper()
	l, err := vocab.NewUbiquitousLanguage(nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	return l
}

func mustLang(t *testing.T, entities []vocab.Entity, vos, rules, events []vocab.Definition) vocab.UbiquitousLanguage {
	t.Helper()
	l, err := vocab.NewUbiquitousLanguage(entities, vos, rules, events)
	if err != nil {
		t.Fatalf("NewUbiquitousLanguage: %v", err)
	}
	return l
}

func entity(name string, aggregate bool) vocab.Entity {
	return vocab.Entity{Definition: vocab.Definition{Name: name}, Aggregate: aggregate}
}

func TestNewUbiquitousLanguageRejectsEmptyName(t *testing.T) {
	_, err := vocab.NewUbiquitousLanguage(
		[]vocab.Entity{{Definition: vocab.Definition{Name: "  "}}},
		nil, nil, nil,
	)
	if err == nil {
		t.Fatal("expected empty-name error")
	}
	if !strings.Contains(err.Error(), "entities") {
		t.Errorf("error %q should name section", err)
	}
}

func TestNewUbiquitousLanguageRejectsDuplicateName(t *testing.T) {
	_, err := vocab.NewUbiquitousLanguage(
		nil,
		[]vocab.Definition{{Name: "Money"}, {Name: "Money"}},
		nil, nil,
	)
	if err == nil {
		t.Fatal("expected duplicate-name error")
	}
	if !strings.Contains(err.Error(), "value_objects") || !strings.Contains(err.Error(), "Money") {
		t.Errorf("error %q should name section and name", err)
	}
}

func TestNewUbiquitousLanguageAcceptsDesignatedEntity(t *testing.T) {
	l := mustLang(t, []vocab.Entity{entity("Order", true)}, nil, nil, nil)
	if !l.Entities[0].Aggregate {
		t.Fatal("expected Aggregate designation preserved")
	}
	// Embedding promotion: Name is reachable on Entity without going
	// through the nested Definition field explicitly.
	if l.Entities[0].Name != "Order" {
		t.Fatalf("promoted Name = %q, want Order", l.Entities[0].Name)
	}
}

func TestUbiquitousLanguageEmpty(t *testing.T) {
	if !emptyLang(t).Empty() {
		t.Fatal("empty language should report Empty")
	}
	l := mustLang(t, []vocab.Entity{entity("Order", false)}, nil, nil, nil)
	if l.Empty() {
		t.Fatal("non-empty language should not report Empty")
	}
}

func TestCounts(t *testing.T) {
	l := mustLang(t,
		[]vocab.Entity{
			entity("Order", true),
			entity("Customer", false),
			entity("Product", false),
		},
		[]vocab.Definition{{Name: "Money"}, {Name: "OrderID"}, {Name: "SKU"}},
		[]vocab.Definition{{Name: "OrderMustHaveCustomer"}, {Name: "OrderMustHaveLines"}},
		[]vocab.Definition{{Name: "OrderPlaced"}, {Name: "OrderShipped"}},
	)
	got := l.Counts()
	want := vocab.Counts{
		Entities: 3, Aggregates: 1, ValueObjects: 3, BusinessRules: 2, Events: 2,
	}
	if got != want {
		t.Errorf("Counts() = %+v, want %+v", got, want)
	}
}

func TestDefineCreateUpdateClearUnchangedMatrix(t *testing.T) {
	concepts := []vocab.Concept{
		vocab.ConceptEntity,
		vocab.ConceptValueObject,
		vocab.ConceptBusinessRule,
		vocab.ConceptEvent,
	}
	for _, c := range concepts {
		t.Run(string(c), func(t *testing.T) {
			l := emptyLang(t)

			// Create with definition.
			l, res, err := l.Define(c, "Term", vocab.Change{
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
			def, ok := l.Find(c, "Term")
			if !ok || def.Definition != "meaning" {
				t.Fatalf("after create Find = %+v ok=%v", def, ok)
			}

			// Unchanged re-define.
			l, res, err = l.Define(c, "Term", vocab.Change{
				SetDefinition:  true,
				DefinitionText: "meaning",
			})
			if err != nil {
				t.Fatalf("unchanged: %v", err)
			}
			if res.Outcome != vocab.OutcomeUnchanged {
				t.Fatalf("unchanged outcome = %q", res.Outcome)
			}
			if len(res.Changed) != 0 {
				t.Fatalf("unchanged Changed = %v, want empty", res.Changed)
			}

			// Update definition.
			l, res, err = l.Define(c, "Term", vocab.Change{
				SetDefinition:  true,
				DefinitionText: "new meaning",
			})
			if err != nil {
				t.Fatalf("update: %v", err)
			}
			if res.Outcome != vocab.OutcomeUpdated {
				t.Fatalf("update outcome = %q", res.Outcome)
			}
			if !reflect.DeepEqual(res.Changed, []string{"definition"}) {
				t.Fatalf("update Changed = %v", res.Changed)
			}

			// Clear definition.
			l, res, err = l.Define(c, "Term", vocab.Change{
				SetDefinition:  true,
				DefinitionText: "",
			})
			if err != nil {
				t.Fatalf("clear: %v", err)
			}
			if res.Outcome != vocab.OutcomeUpdated {
				t.Fatalf("clear outcome = %q", res.Outcome)
			}
			def, _ = l.Find(c, "Term")
			if def.Definition != "" {
				t.Fatalf("after clear Definition = %q", def.Definition)
			}

			// Alias replacement.
			l, res, err = l.Define(c, "Term", vocab.Change{
				SetAliases: true,
				Aliases:    []string{"A", "B"},
			})
			if err != nil {
				t.Fatalf("aliases: %v", err)
			}
			if res.Outcome != vocab.OutcomeUpdated {
				t.Fatalf("aliases outcome = %q", res.Outcome)
			}
			if !reflect.DeepEqual(res.Changed, []string{"aliases"}) {
				t.Fatalf("aliases Changed = %v", res.Changed)
			}
			def, _ = l.Find(c, "Term")
			if !reflect.DeepEqual(def.Aliases, []string{"A", "B"}) {
				t.Fatalf("aliases = %v", def.Aliases)
			}

			// Alias set replacement (not merge).
			l, res, err = l.Define(c, "Term", vocab.Change{
				SetAliases: true,
				Aliases:    []string{"C"},
			})
			if err != nil {
				t.Fatalf("replace aliases: %v", err)
			}
			def, _ = l.Find(c, "Term")
			if !reflect.DeepEqual(def.Aliases, []string{"C"}) {
				t.Fatalf("replaced aliases = %v", def.Aliases)
			}

			// Clear aliases.
			l, res, err = l.Define(c, "Term", vocab.Change{ClearAliases: true})
			if err != nil {
				t.Fatalf("clear aliases: %v", err)
			}
			if !reflect.DeepEqual(res.Changed, []string{"aliases"}) {
				t.Fatalf("clear aliases Changed = %v", res.Changed)
			}
			def, _ = l.Find(c, "Term")
			if len(def.Aliases) != 0 {
				t.Fatalf("after clear aliases = %v", def.Aliases)
			}

			// Unchanged with no fields.
			_, res, err = l.Define(c, "Term", vocab.Change{})
			if err != nil {
				t.Fatalf("noop: %v", err)
			}
			if res.Outcome != vocab.OutcomeUnchanged {
				t.Fatalf("noop outcome = %q", res.Outcome)
			}
		})
	}
}

func TestDefineRejectsEmptyName(t *testing.T) {
	l := emptyLang(t)
	_, _, err := l.Define(vocab.ConceptEntity, "  ", vocab.Change{})
	if err == nil {
		t.Fatal("expected empty-name error")
	}
}

func TestDefineRejectsMutualExclusion(t *testing.T) {
	l := emptyLang(t)
	_, _, err := l.Define(vocab.ConceptEntity, "Order", vocab.Change{
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

	// Create-through-define aggregate.
	l, res, err := l.Define(vocab.ConceptAggregate, "Order", vocab.Change{
		SetDefinition:  true,
		DefinitionText: "A customer's request to purchase products.",
	})
	if err != nil {
		t.Fatalf("define aggregate: %v", err)
	}
	if res.Outcome != vocab.OutcomeCreated {
		t.Fatalf("outcome = %q, want created", res.Outcome)
	}
	if !containsAll(res.Changed, "definition", "aggregate") {
		t.Fatalf("Changed = %v, want definition+aggregate", res.Changed)
	}
	// Changed order is definition, aliases, aggregate.
	if !reflect.DeepEqual(res.Changed, []string{"definition", "aggregate"}) {
		t.Fatalf("Changed order = %v", res.Changed)
	}

	def, ok := l.Find(vocab.ConceptEntity, "Order")
	if !ok || def.Definition == "" {
		t.Fatalf("entity after aggregate define = %+v ok=%v", def, ok)
	}
	ent, ok := l.FindEntity("Order")
	if !ok || !ent.Aggregate {
		t.Fatalf("FindEntity after aggregate define = %+v ok=%v", ent, ok)
	}
	if _, ok := l.Find(vocab.ConceptAggregate, "Order"); !ok {
		t.Fatal("Find(aggregate, Order) should succeed")
	}

	// Already-aggregate with no other change → unchanged.
	_, res, err = l.Define(vocab.ConceptAggregate, "Order", vocab.Change{})
	if err != nil {
		t.Fatalf("re-define aggregate: %v", err)
	}
	if res.Outcome != vocab.OutcomeUnchanged {
		t.Fatalf("re-define outcome = %q, want unchanged", res.Outcome)
	}

	// Existing non-aggregate entity → updated with aggregate.
	l2 := emptyLang(t)
	l2, _, err = l2.Define(vocab.ConceptEntity, "Customer", vocab.Change{})
	if err != nil {
		t.Fatalf("define entity: %v", err)
	}
	l2, res, err = l2.Define(vocab.ConceptAggregate, "Customer", vocab.Change{})
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
	l, _, err := l.Define(vocab.ConceptAggregate, "Order", vocab.Change{
		SetDefinition:  true,
		DefinitionText: "orig",
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Update definition without touching aggregate.
	l, res, err := l.Define(vocab.ConceptEntity, "Order", vocab.Change{
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
	ent, _ := l.FindEntity("Order")
	if !ent.Aggregate {
		t.Fatal("entity define cleared Aggregate designation")
	}
	if ent.Definition.Definition != "updated" {
		t.Fatalf("Definition = %q", ent.Definition.Definition)
	}
}

func TestDefineEntitySetAggregateGuided(t *testing.T) {
	l := emptyLang(t)
	agg := true
	l, res, err := l.Define(vocab.ConceptEntity, "Order", vocab.Change{
		SetDefinition:  true,
		DefinitionText: "meaning",
		SetAggregate:   true,
		Aggregate:      agg,
	})
	if err != nil {
		t.Fatalf("guided define: %v", err)
	}
	if res.Outcome != vocab.OutcomeCreated {
		t.Fatalf("outcome = %q", res.Outcome)
	}
	if !containsAll(res.Changed, "definition", "aggregate") {
		t.Fatalf("Changed = %v", res.Changed)
	}
	ent, _ := l.FindEntity("Order")
	if !ent.Aggregate {
		t.Fatal("expected Aggregate designation")
	}

	// Guided clear designation.
	l, res, err = l.Define(vocab.ConceptEntity, "Order", vocab.Change{
		SetAggregate: true,
		Aggregate:    false,
	})
	if err != nil {
		t.Fatalf("clear designation: %v", err)
	}
	if res.Outcome != vocab.OutcomeUpdated {
		t.Fatalf("outcome = %q", res.Outcome)
	}
	if !reflect.DeepEqual(res.Changed, []string{"aggregate"}) {
		t.Fatalf("Changed = %v", res.Changed)
	}
	ent, _ = l.FindEntity("Order")
	if ent.Aggregate {
		t.Fatal("expected Aggregate cleared")
	}
}

func TestDefineAppendsStableOrder(t *testing.T) {
	l := emptyLang(t)
	for _, name := range []string{"Customer", "Order", "Product"} {
		var err error
		l, _, err = l.Define(vocab.ConceptEntity, name, vocab.Change{})
		if err != nil {
			t.Fatalf("define %s: %v", name, err)
		}
	}
	got := l.List(vocab.ConceptEntity)
	if len(got) != 3 || got[0].Name != "Customer" || got[1].Name != "Order" || got[2].Name != "Product" {
		t.Fatalf("order = %v", namesOf(got))
	}
	entities := l.ListEntities()
	if len(entities) != 3 || entities[0].Name != "Customer" || entities[1].Name != "Order" || entities[2].Name != "Product" {
		t.Fatalf("ListEntities order = %v", entityNamesOf(entities))
	}
}

func TestDefineTrimsName(t *testing.T) {
	l := emptyLang(t)
	l, _, err := l.Define(vocab.ConceptEntity, "  Order  ", vocab.Change{})
	if err != nil {
		t.Fatalf("define: %v", err)
	}
	if _, ok := l.Find(vocab.ConceptEntity, "Order"); !ok {
		t.Fatal("expected trimmed name Order")
	}
	if _, ok := l.Find(vocab.ConceptEntity, "  Order  "); ok {
		t.Fatal("padded name should not match")
	}
}

func TestRemoveMatrix(t *testing.T) {
	l := emptyLang(t)
	var err error
	l, _, err = l.Define(vocab.ConceptEntity, "Order", vocab.Change{
		SetDefinition:  true,
		DefinitionText: "A customer's request to purchase products.",
		SetAggregate:   true,
		Aggregate:      true,
	})
	if err != nil {
		t.Fatalf("setup order: %v", err)
	}
	l, _, err = l.Define(vocab.ConceptValueObject, "Money", vocab.Change{})
	if err != nil {
		t.Fatalf("setup money: %v", err)
	}
	l, _, err = l.Define(vocab.ConceptBusinessRule, "OrderMustHaveCustomer", vocab.Change{})
	if err != nil {
		t.Fatalf("setup rule: %v", err)
	}
	l, _, err = l.Define(vocab.ConceptEvent, "OrderPlaced", vocab.Change{})
	if err != nil {
		t.Fatalf("setup event: %v", err)
	}

	// Remove aggregate preserves entity.
	l, res, err := l.Remove(vocab.ConceptAggregate, "Order")
	if err != nil {
		t.Fatalf("remove aggregate: %v", err)
	}
	if !res.EntityPreserved {
		t.Fatal("EntityPreserved = false")
	}
	def, ok := l.Find(vocab.ConceptEntity, "Order")
	if !ok {
		t.Fatal("entity should remain after aggregate removal")
	}
	ent, ok := l.FindEntity("Order")
	if !ok || ent.Aggregate {
		t.Fatalf("aggregate designation should be cleared: %+v ok=%v", ent, ok)
	}
	if def.Name != "Order" {
		t.Fatalf("remaining entity name = %q", def.Name)
	}
	if _, ok := l.Find(vocab.ConceptAggregate, "Order"); ok {
		t.Fatal("Find(aggregate) should fail after removal")
	}

	// Removing aggregate again (not designated) → ErrDefinitionNotFound.
	_, _, err = l.Remove(vocab.ConceptAggregate, "Order")
	if !errors.Is(err, vocab.ErrDefinitionNotFound) {
		t.Fatalf("second aggregate remove err = %v, want ErrDefinitionNotFound", err)
	}
	if err == nil || !strings.Contains(err.Error(), `no aggregate named "Order"`) {
		t.Fatalf("error vocabulary = %v", err)
	}

	// Remove entity deletes it.
	l, res, err = l.Remove(vocab.ConceptEntity, "Order")
	if err != nil {
		t.Fatalf("remove entity: %v", err)
	}
	if res.EntityPreserved {
		t.Fatal("EntityPreserved should be false for entity removal")
	}
	if _, ok := l.Find(vocab.ConceptEntity, "Order"); ok {
		t.Fatal("entity should be gone")
	}

	// Remove other concepts.
	for _, c := range []struct {
		concept vocab.Concept
		name    string
	}{
		{vocab.ConceptValueObject, "Money"},
		{vocab.ConceptBusinessRule, "OrderMustHaveCustomer"},
		{vocab.ConceptEvent, "OrderPlaced"},
	} {
		var rerr error
		l, _, rerr = l.Remove(c.concept, c.name)
		if rerr != nil {
			t.Fatalf("remove %s %s: %v", c.concept, c.name, rerr)
		}
		if _, ok := l.Find(c.concept, c.name); ok {
			t.Fatalf("%s %s still present", c.concept, c.name)
		}
	}

	// Missing name → ErrDefinitionNotFound with vocabulary.
	_, _, err = l.Remove(vocab.ConceptEntity, "Missing")
	if !errors.Is(err, vocab.ErrDefinitionNotFound) {
		t.Fatalf("missing remove err = %v", err)
	}
	if !strings.Contains(err.Error(), `no entity named "Missing" is defined in the project domain model`) {
		t.Fatalf("error vocabulary = %v", err)
	}
}

func TestFindAndListExactMatching(t *testing.T) {
	l := mustLang(t,
		[]vocab.Entity{
			entity("Order", true),
			entity("Customer", false),
			entity("order", false), // different case
		},
		[]vocab.Definition{{Name: "Money"}},
		nil, nil,
	)

	// Exact match only.
	if _, ok := l.Find(vocab.ConceptEntity, "ORDER"); ok {
		t.Fatal("Find should be case-sensitive")
	}
	if _, ok := l.Find(vocab.ConceptEntity, "Order "); ok {
		t.Fatal("Find should not trim")
	}
	if _, ok := l.Find(vocab.ConceptEntity, "Order"); !ok {
		t.Fatal("Find(Order) should succeed")
	}
	if _, ok := l.Find(vocab.ConceptEntity, "order"); !ok {
		t.Fatal("Find(order) should succeed for lowercase entry")
	}

	// Aggregate matches only designated.
	if _, ok := l.Find(vocab.ConceptAggregate, "Order"); !ok {
		t.Fatal("Find(aggregate, Order) should succeed")
	}
	if _, ok := l.Find(vocab.ConceptAggregate, "Customer"); ok {
		t.Fatal("Find(aggregate, Customer) should fail")
	}

	// List entities = all, aggregates = designated, file order.
	entities := l.List(vocab.ConceptEntity)
	if got := namesOf(entities); !reflect.DeepEqual(got, []string{"Order", "Customer", "order"}) {
		t.Fatalf("List(entity) = %v", got)
	}
	aggregates := l.List(vocab.ConceptAggregate)
	if got := namesOf(aggregates); !reflect.DeepEqual(got, []string{"Order"}) {
		t.Fatalf("List(aggregate) = %v", got)
	}
	vos := l.List(vocab.ConceptValueObject)
	if got := namesOf(vos); !reflect.DeepEqual(got, []string{"Money"}) {
		t.Fatalf("List(value-object) = %v", got)
	}
}

func TestFindEntityListEntitiesListAggregates(t *testing.T) {
	l := mustLang(t,
		[]vocab.Entity{
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
		nil, nil, nil,
	)

	ent, ok := l.FindEntity("Order")
	if !ok {
		t.Fatal("FindEntity(Order) missing")
	}
	if !ent.Aggregate {
		t.Fatal("FindEntity should return Aggregate designation")
	}
	// Embedding promotion: Name, Aliases reachable on Entity.
	if ent.Name != "Order" {
		t.Fatalf("promoted Name = %q", ent.Name)
	}
	if !reflect.DeepEqual(ent.Aliases, []string{"PO"}) {
		t.Fatalf("promoted Aliases = %v", ent.Aliases)
	}
	// Nested definition text still lives on the embedded Definition.
	if ent.Definition.Definition != "A customer's request to purchase products." {
		t.Fatalf("Definition text = %q", ent.Definition.Definition)
	}
	if _, ok := l.FindEntity("Missing"); ok {
		t.Fatal("FindEntity(Missing) should fail")
	}

	all := l.ListEntities()
	if got := entityNamesOf(all); !reflect.DeepEqual(got, []string{"Order", "Customer", "Product"}) {
		t.Fatalf("ListEntities = %v", got)
	}
	if !all[0].Aggregate || all[1].Aggregate || all[2].Aggregate {
		t.Fatalf("ListEntities designations = %+v", all)
	}

	aggs := l.ListAggregates()
	if got := entityNamesOf(aggs); !reflect.DeepEqual(got, []string{"Order"}) {
		t.Fatalf("ListAggregates = %v", got)
	}
	if len(aggs) != 1 || !aggs[0].Aggregate {
		t.Fatalf("ListAggregates content = %+v", aggs)
	}
}

func TestEntityEmbeddingPromotion(t *testing.T) {
	// Constructing via composite literal and reading promoted fields
	// proves Entity is Definition plus Aggregate, not a parallel shape.
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
	if e.Definition.Name != e.Name {
		t.Errorf("embedded Name %q != promoted %q", e.Definition.Name, e.Name)
	}
	// The string field Definition is nested under the embedded struct
	// (same identifier as the embed); access it through the nest.
	if e.Definition.Definition != "Goods in transit." {
		t.Errorf("nested Definition text = %q", e.Definition.Definition)
	}
}

func TestDefineAliasUnchangedElementWise(t *testing.T) {
	l := emptyLang(t)
	l, _, err := l.Define(vocab.ConceptEntity, "Order", vocab.Change{
		SetAliases: true,
		Aliases:    []string{"Purchase Order", "PO"},
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	_, res, err := l.Define(vocab.ConceptEntity, "Order", vocab.Change{
		SetAliases: true,
		Aliases:    []string{"Purchase Order", "PO"},
	})
	if err != nil {
		t.Fatalf("redefine: %v", err)
	}
	if res.Outcome != vocab.OutcomeUnchanged {
		t.Fatalf("outcome = %q, want unchanged", res.Outcome)
	}

	// Different order counts as change.
	_, res, err = l.Define(vocab.ConceptEntity, "Order", vocab.Change{
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
