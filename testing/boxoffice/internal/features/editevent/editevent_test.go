package editevent_test

import (
	"boxoffice/internal/entities/capacity"
	"boxoffice/internal/entities/event"
	"boxoffice/internal/features/editevent"
	"errors"
	"testing"
	"time"
)

type events map[string]event.Event

func (e events) Save(ev event.Event) error { e[ev.ID()] = ev; return nil }

func (e events) Find(id string) (event.Event, error) {
	ev, ok := e[id]
	if !ok {
		return event.Event{}, event.ErrEventUnknown
	}
	return ev, nil
}

func (e events) All() ([]event.Event, error) { return nil, nil }

type capacities map[string]capacity.Capacity

func (c capacities) Save(count capacity.Capacity) error {
	c[count.EventID()] = count.Clone()
	return nil
}

func (c capacities) Find(eventID string) (capacity.Capacity, error) {
	stored, ok := c[eventID]
	if !ok {
		return capacity.Capacity{}, capacity.ErrCapacityUnknown
	}
	return stored.Clone(), nil
}

// seeded builds one stored draft with a single unpriced general tier
// of 50 seats, the shape a fresh draft has.
func seeded(t *testing.T) (events, capacities) {
	t.Helper()
	ev, err := event.New("winter-recital", "Winter Recital")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := ev.AddTier("general", 0); err != nil {
		t.Fatalf("AddTier: %v", err)
	}
	c, err := capacity.New("winter-recital")
	if err != nil {
		t.Fatalf("capacity.New: %v", err)
	}
	if err := c.OpenTier("general", 50); err != nil {
		t.Fatalf("OpenTier: %v", err)
	}
	evs, caps := events{}, capacities{}
	if err := evs.Save(ev); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := caps.Save(c); err != nil {
		t.Fatalf("Save capacity: %v", err)
	}
	return evs, caps
}

func TestEditReshapesDraftAndRecountsSeats(t *testing.T) {
	evs, caps := seeded(t)
	f := editevent.Feature{Events: evs, Capacities: caps}

	edited, err := f.Do("winter-recital", "The students' season finale.", "December 12, 19:00", "Main Hall",
		[]editevent.Tier{
			{Name: "general", Price: 1200, Seats: 80},
			{Name: "front-row", Price: 2000, Seats: 10},
		})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if edited.Story() != "The students' season finale." || edited.Where() != "Main Hall" {
		t.Fatalf("details not applied: %q, %q", edited.Story(), edited.Where())
	}

	stored, err := evs.Find("winter-recital")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got := stored.Tiers(); len(got) != 2 || got[0].Price != 1200 || got[1].Name != "front-row" {
		t.Fatalf("stored tiers = %v", got)
	}
	now := time.Date(2026, 8, 28, 19, 0, 0, 0, time.UTC)
	c, err := caps.Find("winter-recital")
	if err != nil {
		t.Fatalf("Find capacity: %v", err)
	}
	if left, err := c.Remaining("general", now); err != nil || left != 80 {
		t.Fatalf("general remaining = %d, %v, want 80", left, err)
	}
	if left, err := c.Remaining("front-row", now); err != nil || left != 10 {
		t.Fatalf("front-row remaining = %d, %v, want 10", left, err)
	}
}

func TestEditRemovesATier(t *testing.T) {
	evs, caps := seeded(t)
	f := editevent.Feature{Events: evs, Capacities: caps}

	if _, err := f.Do("winter-recital", "", "", "", nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	stored, err := evs.Find("winter-recital")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got := stored.Tiers(); len(got) != 0 {
		t.Fatalf("tiers after removing all = %v", got)
	}
	now := time.Date(2026, 8, 28, 19, 0, 0, 0, time.UTC)
	c, err := caps.Find("winter-recital")
	if err != nil {
		t.Fatalf("Find capacity: %v", err)
	}
	if _, err := c.Remaining("general", now); !errors.Is(err, capacity.ErrTierNotOpen) {
		t.Fatalf("removed tier still counted: %v", err)
	}
}

func TestEditRefusesPublishedEvent(t *testing.T) {
	evs, caps := seeded(t)
	ev, _ := evs.Find("winter-recital")
	if err := ev.ReplaceTiers([]event.TicketTier{{Name: "general", Price: 1200}}); err != nil {
		t.Fatalf("ReplaceTiers: %v", err)
	}
	if err := ev.Publish(); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if err := evs.Save(ev); err != nil {
		t.Fatalf("Save: %v", err)
	}

	f := editevent.Feature{Events: evs, Capacities: caps}
	_, err := f.Do("winter-recital", "rewritten", "later", "elsewhere",
		[]editevent.Tier{{Name: "general", Price: 9900, Seats: 5}})
	if !errors.Is(err, event.ErrEventPublished) {
		t.Fatalf("Do on published = %v, want ErrEventPublished", err)
	}
	stored, _ := evs.Find("winter-recital")
	if tier, _ := stored.Tier("general"); stored.Story() == "rewritten" || tier.Price != 1200 {
		t.Fatalf("a refused edit reached the store: %q, %d", stored.Story(), tier.Price)
	}
}

func TestEditRefusesBadSeatCountAndKeepsTheStore(t *testing.T) {
	evs, caps := seeded(t)
	f := editevent.Feature{Events: evs, Capacities: caps}

	_, err := f.Do("winter-recital", "rewritten", "", "",
		[]editevent.Tier{{Name: "general", Price: 1200, Seats: 0}})
	if !errors.Is(err, capacity.ErrSeatsInvalid) {
		t.Fatalf("Do with zero seats = %v, want ErrSeatsInvalid", err)
	}
	stored, _ := evs.Find("winter-recital")
	if stored.Story() == "rewritten" {
		t.Fatal("a refused edit reached the event store")
	}
	now := time.Date(2026, 8, 28, 19, 0, 0, 0, time.UTC)
	c, _ := caps.Find("winter-recital")
	if left, err := c.Remaining("general", now); err != nil || left != 50 {
		t.Fatalf("a refused edit reached the count: %d, %v", left, err)
	}
}

func TestEditUnknownEvent(t *testing.T) {
	f := editevent.Feature{Events: events{}, Capacities: capacities{}}
	if _, err := f.Do("nope", "", "", "", nil); !errors.Is(err, event.ErrEventUnknown) {
		t.Fatalf("Do = %v, want ErrEventUnknown", err)
	}
}
