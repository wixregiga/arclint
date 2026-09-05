package lipgloss

import (
	"bytes"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

func TestLipglossContextScopedDomainGrammarAndAnchorColors(t *testing.T) {
	dk := &application.DomainKnowledge{
		Source:  "domain.arclint.yaml",
		Counts:  vocab.Counts{Contexts: 2, Entities: 1, Aggregates: 1, ValueObjects: 2, Invariants: 3},
		Scoped:  true,
		Shown:   vocab.Counts{Contexts: 1, Entities: 1, Aggregates: 1, ValueObjects: 1, Invariants: 3},
		Located: true,
		Contexts: []application.DomainContextKnowledge{{
			Name:         "catalog",
			Entities:     []application.DomainEntityRef{{Name: "Event", Aggregate: true}},
			ValueObjects: []string{"Price"},
			Invariants: []application.DomainInvariantRef{
				{Statement: "A published Event never changes.", Owner: "Event", ID: "published-frozen", Source: "event/event.go:90", Anchor: application.AnchorFound},
				{Statement: "An Event has one Venue.", Owner: "Event", Anchor: application.AnchorUnanchorable, Reason: "owner Event is an aggregate and the invariant has no id, so no method is named to carry it"},
				{Statement: "A Price is never negative.", Owner: "Price", Anchor: application.AnchorMissing},
			},
		}},
		Unanchored: []application.UnanchoredContract{
			{Kind: application.ContractInvariant, Context: "catalog", Owner: "Event", Statement: "An Event has one Venue.", Anchor: application.AnchorUnanchorable, Reason: "owner Event is an aggregate and the invariant has no id, so no method is named to carry it"},
			{Kind: application.ContractInvariant, Context: "catalog", Owner: "Price", Statement: "A Price is never negative.", Anchor: application.AnchorMissing},
		},
	}
	var buf bytes.Buffer
	err := ansiRenderer().Render(&buf, cli.ContextReport{Context: application.ArchitecturalContext{Scope: "m/event.go", Domain: dk}})
	if err != nil {
		t.Fatal(err)
	}
	raw := buf.String()
	if !strings.Contains(raw, "\x1b[31munanchorable\x1b[0m") {
		t.Errorf("unanchorable is not rendered in the error color: %q", raw)
	}
	if !strings.Contains(raw, "\x1b[33mmissing\x1b[0m") {
		t.Errorf("missing is not rendered in the warning color: %q", raw)
	}
	if !strings.Contains(raw, "\x1b[36;2mevent/event.go:90\x1b[0m") && !strings.Contains(raw, "\x1b[2;36mevent/event.go:90\x1b[0m") {
		t.Errorf("a found source is not rendered as a path: %q", raw)
	}
	out := stripANSI(raw)
	for _, want := range []string{
		"project domain (domain.arclint.yaml): 1 of 2 contexts, 1 of 1 entity, 1 of 2 value objects, 3 of 3 invariants anchor into this scope; --full shows the whole model\n",
		"      A published Event never changes. (owner: Event, id: published-frozen) event/event.go:90\n",
		"      An Event has one Venue. (owner: Event) unanchorable\n",
		"      A Price is never negative. (owner: Price) missing\n",
		"  unanchored contracts: 1 unanchorable, 1 missing\n",
		"    unanchorable: 1 invariant owned by Event (context catalog)\n",
		"      owner Event is an aggregate and the invariant has no id, so no method is named to carry it\n",
		"    missing: 1 invariant owned by Price (context catalog)\n",
		"      no constructor declared for Price\n",
		"    an unanchorable contract needs its recording changed before any source can carry it\n",
		"    an invariants Rule on the owning Module reports each missing contract as a Violation\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("grammar lacks %q:\n%s", want, out)
		}
	}
}
