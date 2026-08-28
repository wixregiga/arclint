package placeorder_test

import (
	"boxoffice/internal/entities/capacity"
	"boxoffice/internal/entities/event"
	"boxoffice/internal/entities/order"
	"boxoffice/internal/features/placeorder"
	"errors"
	"testing"
	"time"
)

var (
	t0       = time.Date(2026, 8, 28, 19, 0, 0, 0, time.UTC)
	attendee = order.Attendee{Name: "Sam", Email: "sam@example.com"}
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

type orders map[string]order.Order

func (o orders) Save(ord order.Order) error { o[ord.ID()] = ord; return nil }

func (o orders) Find(id string) (order.Order, error) {
	ord, ok := o[id]
	if !ok {
		return order.Order{}, order.ErrOrderUnknown
	}
	return ord, nil
}

func (o orders) ForEvent(eventID string) ([]order.Order, error) {
	out := []order.Order{}
	for _, ord := range o {
		if ord.EventID() == eventID {
			out = append(out, ord)
		}
	}
	return out, nil
}

type capacities map[string]capacity.Capacity

func (c capacities) Save(cp capacity.Capacity) error { c[cp.EventID()] = cp.Clone(); return nil }

func (c capacities) Find(eventID string) (capacity.Capacity, error) {
	cp, ok := c[eventID]
	if !ok {
		return capacity.Capacity{}, capacity.ErrCapacityUnknown
	}
	return cp.Clone(), nil
}

func sale(t *testing.T) (events, orders, capacities) {
	t.Helper()
	ev, err := event.New("jazz-trio", "Jazz Trio")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := ev.AddTier("general", 2200); err != nil {
		t.Fatalf("AddTier general: %v", err)
	}
	if err := ev.AddTier("front-row", 3000); err != nil {
		t.Fatalf("AddTier front-row: %v", err)
	}
	if err := ev.Publish(); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	c, err := capacity.New("jazz-trio")
	if err != nil {
		t.Fatalf("capacity.New: %v", err)
	}
	if err := c.OpenTier("general", 4); err != nil {
		t.Fatalf("OpenTier: %v", err)
	}
	if err := c.OpenTier("front-row", 1); err != nil {
		t.Fatalf("OpenTier: %v", err)
	}
	return events{"jazz-trio": ev}, orders{}, capacities{"jazz-trio": c}
}

func hold(t *testing.T, caps capacities, id, tier string, seats int) {
	t.Helper()
	c, err := caps.Find("jazz-trio")
	if err != nil {
		t.Fatalf("Find capacity: %v", err)
	}
	if err := c.PlaceHold(id, tier, seats, t0.Add(5*time.Minute), t0); err != nil {
		t.Fatalf("PlaceHold %s: %v", id, err)
	}
	if err := caps.Save(c); err != nil {
		t.Fatalf("Save capacity: %v", err)
	}
}

func TestPlaceOrderCapturesPricesAndSpeaksForSeats(t *testing.T) {
	evs, ords, caps := sale(t)
	hold(t, caps, "h1", "general", 2)
	hold(t, caps, "h2", "general", 1)
	hold(t, caps, "h3", "front-row", 1)

	f := placeorder.Feature{Events: evs, Orders: ords, Capacities: caps}
	o, err := f.Do("ord-1", "jazz-trio", []string{"h1", "h2", "h3"}, attendee, t0)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	lines := o.Lines()
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2 (same-tier holds merge)", len(lines))
	}
	if lines[0].TierName != "general" || lines[0].Quantity != 3 || lines[0].UnitCents != 2200 {
		t.Fatalf("general line = %+v", lines[0])
	}
	if o.TotalCents() != 3*2200+3000 {
		t.Fatalf("TotalCents = %d", o.TotalCents())
	}
	if _, err := ords.Find("ord-1"); err != nil {
		t.Fatalf("order not persisted: %v", err)
	}

	stored, err := caps.Find("jazz-trio")
	if err != nil {
		t.Fatalf("Find capacity: %v", err)
	}
	farFuture := t0.Add(time.Hour)
	left, err := stored.Remaining("general", farFuture)
	if err != nil || left != 1 {
		t.Fatalf("general Remaining = %d, %v; want 1 spoken-for-forever", left, err)
	}
}

func TestExpiredHoldRefusesTheWholeOrder(t *testing.T) {
	evs, ords, caps := sale(t)
	hold(t, caps, "h1", "general", 1)

	later := t0.Add(10 * time.Minute)
	f := placeorder.Feature{Events: evs, Orders: ords, Capacities: caps}
	if _, err := f.Do("ord-1", "jazz-trio", []string{"h1"}, attendee, later); !errors.Is(err, capacity.ErrHoldExpiredOrUnknown) {
		t.Fatalf("Do with expired hold = %v, want ErrHoldExpiredOrUnknown", err)
	}
	if len(ords) != 0 {
		t.Fatal("a refused order must not be persisted")
	}
	stored, err := caps.Find("jazz-trio")
	if err != nil {
		t.Fatalf("Find capacity: %v", err)
	}
	left, err := stored.Remaining("general", later)
	if err != nil || left != 4 {
		t.Fatalf("Remaining = %d, %v; the expired hold's seats must be free", left, err)
	}
}

func TestIncompleteAttendeeRefusedWithoutSideEffects(t *testing.T) {
	evs, ords, caps := sale(t)
	hold(t, caps, "h1", "general", 1)

	f := placeorder.Feature{Events: evs, Orders: ords, Capacities: caps}
	if _, err := f.Do("ord-1", "jazz-trio", []string{"h1"}, order.Attendee{Name: "Sam"}, t0); !errors.Is(err, order.ErrAttendeeMissing) {
		t.Fatalf("Do = %v, want ErrAttendeeMissing", err)
	}
	stored, err := caps.Find("jazz-trio")
	if err != nil {
		t.Fatalf("Find capacity: %v", err)
	}
	left, err := stored.Remaining("general", t0)
	if err != nil || left != 3 {
		t.Fatalf("Remaining = %d, %v; the hold must still stand, nothing spoken for", left, err)
	}
}
