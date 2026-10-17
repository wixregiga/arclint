package application_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// A name alone finds the entry its name, key, or alias spells, in any
// concept; a spelling equal but for case is accepted when nothing
// matches exactly.
func TestShowDomainDefinitionFindsAnEntryByNameAlone(t *testing.T) {
	t.Parallel()
	a := orderAggregate()
	a.Aliases = []string{"Purchase Order"}
	ctx := orderContext(a)[0]
	ctx.ValueObjects = []vocab.ValueObject{{Name: "Money", Definition: "An amount."}}
	ctx.Services = []vocab.DomainService{{Name: "Pricing", Definition: "Prices an Order.",
		Assertions: []vocab.Assertion{{Key: "priced-once", On: "Price", Statement: "An Order is priced once."}}}}
	uc, err := application.NewShowDomainDefinition(recorded(t, ctx))
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	for _, tc := range []struct {
		name      string
		want      vocab.Concept
		wantName  string
		wantOwner string
	}{
		{"Order", vocab.ConceptAggregate, "Order", ""},
		{"Purchase Order", vocab.ConceptAggregate, "Order", ""},
		{"order", vocab.ConceptAggregate, "Order", ""},
		{"OrderLine", vocab.ConceptEntity, "OrderLine", "Order"},
		{"Money", vocab.ConceptValueObject, "Money", ""},
		{"has-lines", vocab.ConceptInvariant, "has-lines", "Order"},
		{"paid-in-full", vocab.ConceptAssertion, "paid-in-full", "Order"},
		{"priced-once", vocab.ConceptAssertion, "priced-once", "Pricing"},
		{"Pricing", vocab.ConceptDomainService, "Pricing", ""},
		{testContext, vocab.ConceptBoundedContext, testContext, ""},
	} {
		view, err := uc.Execute("", "", "", tc.name)
		if err != nil {
			t.Fatalf("show %s: %v", tc.name, err)
		}
		if view.Concept != tc.want || view.Name != tc.wantName || view.Owner != tc.wantOwner {
			t.Errorf("show %s = %s %s owner %q, want %s %s owner %q", tc.name, view.Concept, view.Name, view.Owner, tc.want, tc.wantName, tc.wantOwner)
		}
	}
}

// A name several entries spell lists the command showing each one; a
// concept word and an unrecorded name each say what to do instead.
func TestShowDomainDefinitionListsTheCandidatesOfAnAmbiguousName(t *testing.T) {
	t.Parallel()
	ctx := orderContext(orderAggregate())[0]
	ctx.ValueObjects = []vocab.ValueObject{{Name: "Money", Definition: "An amount.",
		Invariants: []vocab.Invariant{{Key: "has-lines", Statement: "Odd, but recorded."}}}}
	uc, err := application.NewShowDomainDefinition(recorded(t, ctx))
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	_, err = uc.Execute("", "", "", "has-lines")
	if !errors.Is(err, application.ErrDomainUsage) ||
		!strings.Contains(err.Error(), "arclint domain show invariant has-lines --owner Order\n") ||
		!strings.HasSuffix(err.Error(), "arclint domain show invariant has-lines --owner Money") {
		t.Fatalf("ambiguous key: %v", err)
	}
	view, err := uc.Execute("", "", "Money", "has-lines")
	if err != nil || view.Owner != "Money" {
		t.Fatalf("owner narrows the key = %+v err=%v", view, err)
	}
	_, err = uc.Execute("", "", "", "entity")
	if !errors.Is(err, application.ErrDomainUsage) || !strings.Contains(err.Error(), "arclint domain show entity <name>") {
		t.Fatalf("concept word: %v", err)
	}
	_, err = uc.Execute("", "", "", "Basket")
	if !errors.Is(err, vocab.ErrDefinitionNotFound) {
		t.Fatalf("unrecorded name: %v", err)
	}
	_, err = uc.Execute("", "elsewhere", "", "Order")
	if !errors.Is(err, vocab.ErrDefinitionNotFound) {
		t.Fatalf("unrecorded context: %v", err)
	}
}

// Every entry that can record rules lists them, or says it records
// none with the command recording one; an entity points at its
// aggregate. Without a check every rule says no check was run.
func TestShowDomainDefinitionListsTheRulesAnEntryRecords(t *testing.T) {
	t.Parallel()
	ctx := orderContext(orderAggregate())[0]
	ctx.ValueObjects = []vocab.ValueObject{{Name: "Money", Definition: "An amount."}}
	ctx.Services = []vocab.DomainService{{Name: "Pricing", Definition: "Prices an Order."}}
	ctx.Events = []vocab.DomainEvent{{Name: "OrderShipped", Definition: "An Order left the warehouse."}}
	uc, err := application.NewShowDomainDefinition(recorded(t, ctx))
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	view, err := uc.Execute("", "", "", "Order")
	if err != nil || view.Rules == nil || len(view.Rules.Checks) != 2 || view.Rules.Checked {
		t.Fatalf("Order rules = %+v err=%v", view.Rules, err)
	}
	for _, c := range view.Rules.Checks {
		if c.Status != application.RuleNotChecked || len(c.Reasons) != 1 || c.Reasons[0] != "no check was run" {
			t.Errorf("unchecked rule %s = %+v", c.Key, c)
		}
	}
	if k := view.Rules.Checks[1]; k.Kind != vocab.ConceptAssertion || k.On != "Ship" || k.OwnerConcept != vocab.ConceptAggregate {
		t.Errorf("the assertion = %+v", k)
	}
	for name, want := range map[string]string{
		"Money":   `arclint domain define invariant <key> --owner Money --statement "<statement>"`,
		"Pricing": `arclint domain define assertion <key> --owner Pricing --on <operation> --statement "<statement>"`,
	} {
		view, err := uc.Execute("", "", "", name)
		if err != nil || view.Rules == nil || len(view.Rules.Checks) != 0 || view.Rules.Record != want {
			t.Errorf("%s rules = %+v err=%v, want record %q", name, view.Rules, err, want)
		}
	}
	view, err = uc.Execute("", "", "", "OrderLine")
	if err != nil || view.Rules == nil || view.Rules.Aggregate != "Order" {
		t.Errorf("entity rules = %+v err=%v", view.Rules, err)
	}
	view, err = uc.Execute("", "", "", "OrderShipped")
	if err != nil || view.Rules != nil {
		t.Errorf("an event records no rules = %+v err=%v", view.Rules, err)
	}
}
