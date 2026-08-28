package event_test

import (
	"boxoffice/internal/entities/event"
	"errors"
	"testing"
)

func draftWithTier(t *testing.T, price event.Price) event.Event {
	t.Helper()
	ev, err := event.New("open-mic-night", "Open Mic Night")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := ev.AddTier("general", price); err != nil {
		t.Fatalf("AddTier: %v", err)
	}
	return ev
}

func TestPublishRefusesEventWithoutTiers(t *testing.T) {
	ev, err := event.New("open-mic-night", "Open Mic Night")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := ev.Publish(); !errors.Is(err, event.ErrNothingToSell) {
		t.Fatalf("Publish on tierless draft = %v, want ErrNothingToSell", err)
	}
}

func TestPublishRefusesUnpricedTier(t *testing.T) {
	ev := draftWithTier(t, 0)
	if err := ev.Publish(); !errors.Is(err, event.ErrTierUnpriced) {
		t.Fatalf("Publish with unpriced tier = %v, want ErrTierUnpriced", err)
	}
	if !ev.Draft() {
		t.Fatal("a refused publish must leave the event a draft")
	}
}

func TestPublishedEventRefusesEdits(t *testing.T) {
	ev := draftWithTier(t, 1500)
	if err := ev.Publish(); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if err := ev.AddTier("front-row", 3000); !errors.Is(err, event.ErrEventPublished) {
		t.Fatalf("AddTier after publish = %v, want ErrEventPublished", err)
	}
	if err := ev.Tell("new story", "later", "elsewhere"); !errors.Is(err, event.ErrEventPublished) {
		t.Fatalf("Tell after publish = %v, want ErrEventPublished", err)
	}
	if err := ev.Publish(); !errors.Is(err, event.ErrAlreadyPublished) {
		t.Fatalf("second Publish = %v, want ErrAlreadyPublished", err)
	}
}

func TestDuplicateTierRefused(t *testing.T) {
	ev := draftWithTier(t, 1500)
	if err := ev.AddTier("general", 2200); !errors.Is(err, event.ErrTierDuplicate) {
		t.Fatalf("duplicate AddTier = %v, want ErrTierDuplicate", err)
	}
}

func TestNegativePriceRefused(t *testing.T) {
	ev := draftWithTier(t, 1500)
	if err := ev.AddTier("front-row", -1); !errors.Is(err, event.ErrPriceNegative) {
		t.Fatalf("negative AddTier = %v, want ErrPriceNegative", err)
	}
	err := ev.ReplaceTiers([]event.TicketTier{{Name: "general", Price: -100}})
	if !errors.Is(err, event.ErrPriceNegative) {
		t.Fatalf("negative ReplaceTiers = %v, want ErrPriceNegative", err)
	}
}

func TestReplaceTiersReshapesTheDraft(t *testing.T) {
	ev := draftWithTier(t, 0)
	next := []event.TicketTier{
		{Name: "general", Price: 2200},
		{Name: "front-row", Price: 3000},
	}
	if err := ev.ReplaceTiers(next); err != nil {
		t.Fatalf("ReplaceTiers: %v", err)
	}
	if got := ev.Tiers(); len(got) != 2 || got[0].Price != 2200 || got[1].Name != "front-row" {
		t.Fatalf("tiers after replace = %v", got)
	}
	// The aggregate keeps its own copy of the list it was given.
	next[0].Price = 1
	if got, _ := ev.Tier("general"); got.Price != 2200 {
		t.Fatalf("mutating the given slice reached the aggregate: %d", got.Price)
	}
	// Removing every tier is a valid draft; it just cannot publish.
	if err := ev.ReplaceTiers(nil); err != nil {
		t.Fatalf("ReplaceTiers to none: %v", err)
	}
	if err := ev.Publish(); !errors.Is(err, event.ErrNothingToSell) {
		t.Fatalf("Publish with no tiers = %v, want ErrNothingToSell", err)
	}
}

func TestReplaceTiersRefusesBadLists(t *testing.T) {
	ev := draftWithTier(t, 1500)
	err := ev.ReplaceTiers([]event.TicketTier{{Name: "", Price: 100}})
	if !errors.Is(err, event.ErrTierNameMissing) {
		t.Fatalf("nameless tier = %v, want ErrTierNameMissing", err)
	}
	err = ev.ReplaceTiers([]event.TicketTier{
		{Name: "general", Price: 100},
		{Name: "general", Price: 200},
	})
	if !errors.Is(err, event.ErrTierDuplicate) {
		t.Fatalf("duplicate tiers = %v, want ErrTierDuplicate", err)
	}
	// A refused replace changes nothing.
	if got, _ := ev.Tier("general"); got.Price != 1500 {
		t.Fatalf("a refused replace reached the aggregate: %d", got.Price)
	}
}

func TestReplaceTiersRefusesPublishedEvent(t *testing.T) {
	ev := draftWithTier(t, 1500)
	if err := ev.Publish(); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	err := ev.ReplaceTiers([]event.TicketTier{{Name: "general", Price: 9900}})
	if !errors.Is(err, event.ErrEventPublished) {
		t.Fatalf("ReplaceTiers after publish = %v, want ErrEventPublished", err)
	}
}

func TestOnlyAPublishedEventCanBeCancelled(t *testing.T) {
	draft := draftWithTier(t, 1500)
	if err := draft.Cancel(); !errors.Is(err, event.ErrEventNotPublished) {
		t.Fatalf("cancelling a draft = %v, want ErrEventNotPublished", err)
	}
	if !draft.Draft() {
		t.Fatalf("a refused cancel changed the status to %v", draft.Status())
	}

	ev := draftWithTier(t, 1500)
	if err := ev.Publish(); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if err := ev.Cancel(); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if ev.Status() != event.StatusCancelled || !ev.Cancelled() {
		t.Fatalf("status after cancel = %v", ev.Status())
	}
	if ev.OnSale() {
		t.Fatal("a cancelled event must not be on sale")
	}
	if err := ev.Cancel(); !errors.Is(err, event.ErrAlreadyCancelled) {
		t.Fatalf("second Cancel = %v, want ErrAlreadyCancelled", err)
	}
}

func TestCancellingIsFinal(t *testing.T) {
	ev := draftWithTier(t, 1500)
	if err := ev.Publish(); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if err := ev.Cancel(); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	// A cancelled event never goes back on sale and never reopens
	// for editing.
	if err := ev.Publish(); !errors.Is(err, event.ErrEventCancelled) {
		t.Fatalf("Publish after cancel = %v, want ErrEventCancelled", err)
	}
	if err := ev.Tell("new story", "later", "elsewhere"); !errors.Is(err, event.ErrEventCancelled) {
		t.Fatalf("Tell after cancel = %v, want ErrEventCancelled", err)
	}
	if err := ev.AddTier("front-row", 3000); !errors.Is(err, event.ErrEventCancelled) {
		t.Fatalf("AddTier after cancel = %v, want ErrEventCancelled", err)
	}
	err := ev.ReplaceTiers([]event.TicketTier{{Name: "general", Price: 9900}})
	if !errors.Is(err, event.ErrEventCancelled) {
		t.Fatalf("ReplaceTiers after cancel = %v, want ErrEventCancelled", err)
	}
	if got, _ := ev.Tier("general"); got.Price != 1500 {
		t.Fatalf("a refused edit reached the cancelled event: %d", got.Price)
	}
}

func TestANewEventStartsADraft(t *testing.T) {
	ev, err := event.New("open-mic-night", "Open Mic Night")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if ev.Status() != event.StatusDraft || !ev.Draft() || ev.OnSale() || ev.Cancelled() {
		t.Fatalf("a new event = %v, want a draft that is not on sale", ev.Status())
	}
}

func TestEventCopiesAreIndependent(t *testing.T) {
	original := draftWithTier(t, 1500)
	copied := original
	if err := copied.AddTier("front-row", 3000); err != nil {
		t.Fatalf("AddTier on copy: %v", err)
	}
	if len(original.Tiers()) != 1 {
		t.Fatalf("mutating a copy leaked into the original: %d tiers", len(original.Tiers()))
	}
	tiers := original.Tiers()
	tiers[0].Price = 1
	if got, _ := original.Tier("general"); got.Price != 1500 {
		t.Fatalf("mutating the returned slice reached the aggregate: %d", got.Price)
	}
}
