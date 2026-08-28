package holdseats_test

import (
	"boxoffice/internal/entities/capacity"
	"boxoffice/internal/entities/event"
	"boxoffice/internal/features/holdseats"
	"errors"
	"testing"
	"time"
)

var t0 = time.Date(2026, 8, 28, 19, 0, 0, 0, time.UTC)

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

func (c capacities) Save(cp capacity.Capacity) error { c[cp.EventID()] = cp.Clone(); return nil }

func (c capacities) Find(eventID string) (capacity.Capacity, error) {
	cp, ok := c[eventID]
	if !ok {
		return capacity.Capacity{}, capacity.ErrCapacityUnknown
	}
	return cp.Clone(), nil
}

func room(t *testing.T, published bool) (events, capacities) {
	t.Helper()
	ev, err := event.New("jazz-trio", "Jazz Trio")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := ev.AddTier("general", 2200); err != nil {
		t.Fatalf("AddTier: %v", err)
	}
	if published {
		if err := ev.Publish(); err != nil {
			t.Fatalf("Publish: %v", err)
		}
	}
	c, err := capacity.New("jazz-trio")
	if err != nil {
		t.Fatalf("capacity.New: %v", err)
	}
	if err := c.OpenTier("general", 2); err != nil {
		t.Fatalf("OpenTier: %v", err)
	}
	return events{"jazz-trio": ev}, capacities{"jazz-trio": c}
}

func TestHoldOnDraftRefused(t *testing.T) {
	evs, caps := room(t, false)
	f := holdseats.Feature{Events: evs, Capacities: caps, HoldFor: 5 * time.Minute}
	if _, err := f.Do("h1", "jazz-trio", "general", 1, t0); !errors.Is(err, holdseats.ErrEventNotOnSale) {
		t.Fatalf("Do on draft = %v, want ErrEventNotOnSale", err)
	}
}

func TestHoldPersistsAndCarriesItsDeadline(t *testing.T) {
	evs, caps := room(t, true)
	f := holdseats.Feature{Events: evs, Capacities: caps, HoldFor: 5 * time.Minute}

	held, err := f.Do("h1", "jazz-trio", "general", 2, t0)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if want := t0.Add(5 * time.Minute); !held.Deadline.Equal(want) {
		t.Fatalf("Deadline = %v, want %v", held.Deadline, want)
	}
	stored, err := caps.Find("jazz-trio")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	left, err := stored.Remaining("general", t0)
	if err != nil || left != 0 {
		t.Fatalf("Remaining after persisted hold = %d, %v; want 0", left, err)
	}
}

func TestRefusedHoldChangesNothing(t *testing.T) {
	evs, caps := room(t, true)
	f := holdseats.Feature{Events: evs, Capacities: caps, HoldFor: 5 * time.Minute}

	if _, err := f.Do("h1", "jazz-trio", "general", 3, t0); !errors.Is(err, capacity.ErrSeatsExhausted) {
		t.Fatalf("overhold = %v, want ErrSeatsExhausted", err)
	}
	stored, err := caps.Find("jazz-trio")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	left, err := stored.Remaining("general", t0)
	if err != nil || left != 2 {
		t.Fatalf("Remaining after refused hold = %d, %v; want the full 2", left, err)
	}
}
