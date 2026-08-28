package cancelevent_test

import (
	"boxoffice/internal/entities/capacity"
	"boxoffice/internal/entities/event"
	"boxoffice/internal/entities/order"
	"boxoffice/internal/features/cancelevent"
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

// show is a published event with two tiers, two buyers holding
// tickets on it, and one attendee still deciding.
func show(t *testing.T) (events, orders, capacities) {
	t.Helper()
	ev, err := event.New("jazz-trio", "Jazz Trio")
	if err != nil {
		t.Fatalf("event.New: %v", err)
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
	if err := c.OpenTier("general", 10); err != nil {
		t.Fatalf("OpenTier general: %v", err)
	}
	if err := c.OpenTier("front-row", 4); err != nil {
		t.Fatalf("OpenTier front-row: %v", err)
	}
	// Two deals struck: Sam takes three general and one front-row,
	// Ada takes two general.
	deadline := t0.Add(5 * time.Minute)
	for _, h := range []struct {
		id, tier string
		seats    int
	}{
		{"h1", "general", 3},
		{"h2", "front-row", 1},
		{"h3", "general", 2},
	} {
		if err := c.PlaceHold(h.id, h.tier, h.seats, deadline, t0); err != nil {
			t.Fatalf("PlaceHold %s: %v", h.id, err)
		}
	}
	if _, err := c.CommitAll([]string{"h1", "h2", "h3"}, t0); err != nil {
		t.Fatalf("CommitAll: %v", err)
	}
	// A fourth attendee is still deciding when the show is called off.
	if err := c.PlaceHold("h4", "general", 1, deadline, t0); err != nil {
		t.Fatalf("PlaceHold h4: %v", err)
	}

	sam, err := order.New("ord-1", "jazz-trio", order.Attendee{Name: "Sam", Email: "sam@example.com"}, []order.Line{
		{TierName: "general", Quantity: 3, UnitCents: 2200},
		{TierName: "front-row", Quantity: 1, UnitCents: 3000},
	})
	if err != nil {
		t.Fatalf("order.New sam: %v", err)
	}
	ada, err := order.New("ord-2", "jazz-trio", order.Attendee{Name: "Ada", Email: "ada@example.com"}, []order.Line{
		{TierName: "general", Quantity: 2, UnitCents: 2200},
	})
	if err != nil {
		t.Fatalf("order.New ada: %v", err)
	}
	return events{"jazz-trio": ev},
		orders{"ord-1": sam, "ord-2": ada},
		capacities{"jazz-trio": c}
}

func count(t *testing.T, caps capacities, tier string) capacity.Count {
	t.Helper()
	c, err := caps.Find("jazz-trio")
	if err != nil {
		t.Fatalf("Find capacity: %v", err)
	}
	got, err := c.Count(tier, t0)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	return got
}

func TestCancelRefundsEveryTicketAndTakesTheEventOffSale(t *testing.T) {
	evs, ords, caps := show(t)
	f := cancelevent.Feature{Events: evs, Orders: ords, Capacities: caps}

	ev, err := f.Do("jazz-trio")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !ev.Cancelled() || ev.OnSale() {
		t.Fatalf("returned event = %v, want cancelled and off sale", ev.Status())
	}
	stored, err := evs.Find("jazz-trio")
	if err != nil || !stored.Cancelled() {
		t.Fatalf("the cancellation was not persisted: %v, status=%v", err, stored.Status())
	}

	// Every ticket on every order came back, and the deals as struck
	// stayed as struck.
	for _, id := range []string{"ord-1", "ord-2"} {
		o, err := ords.Find(id)
		if err != nil {
			t.Fatalf("Find %s: %v", id, err)
		}
		if o.OutstandingCents() != 0 {
			t.Errorf("%s still owes %d cents after cancelling", id, o.OutstandingCents())
		}
		for _, l := range o.Lines() {
			if o.Outstanding(l.TierName) != 0 {
				t.Errorf("%s still holds %d %s tickets", id, o.Outstanding(l.TierName), l.TierName)
			}
		}
	}
	if o, _ := ords.Find("ord-1"); o.TotalCents() != 3*2200+3000 {
		t.Errorf("TotalCents = %d; the deal as struck never moves", o.TotalCents())
	}

	// The room is whole again: nothing spoken for, nothing held.
	general := count(t, caps, "general")
	if (general != capacity.Count{Seats: 10, Remaining: 10}) {
		t.Errorf("general after cancelling = %+v, want all 10 free", general)
	}
	front := count(t, caps, "front-row")
	if (front != capacity.Count{Seats: 4, Remaining: 4}) {
		t.Errorf("front-row after cancelling = %+v, want all 4 free", front)
	}
}

func TestCancelIsIdempotentlyRefused(t *testing.T) {
	evs, ords, caps := show(t)
	f := cancelevent.Feature{Events: evs, Orders: ords, Capacities: caps}
	if _, err := f.Do("jazz-trio"); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if _, err := f.Do("jazz-trio"); !errors.Is(err, event.ErrAlreadyCancelled) {
		t.Fatalf("second Do = %v, want ErrAlreadyCancelled", err)
	}
	// The second, refused cancel gave nothing back twice.
	general := count(t, caps, "general")
	if general.SpokenFor != 0 || general.Remaining != 10 {
		t.Fatalf("general after a refused second cancel = %+v", general)
	}
}

func TestCancelRefusesADraftAndChangesNothing(t *testing.T) {
	evs, ords, caps := show(t)
	draft, err := event.New("winter-recital", "Winter Recital")
	if err != nil {
		t.Fatalf("event.New: %v", err)
	}
	if err := evs.Save(draft); err != nil {
		t.Fatalf("Save: %v", err)
	}

	f := cancelevent.Feature{Events: evs, Orders: ords, Capacities: caps}
	if _, err := f.Do("winter-recital"); !errors.Is(err, event.ErrEventNotPublished) {
		t.Fatalf("cancelling a draft = %v, want ErrEventNotPublished", err)
	}
	stored, err := evs.Find("winter-recital")
	if err != nil || !stored.Draft() {
		t.Fatalf("the draft did not survive the refusal: %v, status=%v", err, stored.Status())
	}
}

func TestCancelUnknownEvent(t *testing.T) {
	evs, ords, caps := show(t)
	f := cancelevent.Feature{Events: evs, Orders: ords, Capacities: caps}
	if _, err := f.Do("nope"); !errors.Is(err, event.ErrEventUnknown) {
		t.Fatalf("Do = %v, want ErrEventUnknown", err)
	}
}

func TestCancelWithAlreadyRefundedAndUnsoldEvents(t *testing.T) {
	evs, ords, caps := show(t)
	// Ada already had one ticket back before the show was called off.
	ada, err := ords.Find("ord-2")
	if err != nil {
		t.Fatalf("Find ord-2: %v", err)
	}
	if err := ada.Refund("general", 1); err != nil {
		t.Fatalf("Refund: %v", err)
	}
	if err := ords.Save(ada); err != nil {
		t.Fatalf("Save: %v", err)
	}
	c, err := caps.Find("jazz-trio")
	if err != nil {
		t.Fatalf("Find capacity: %v", err)
	}
	if err := c.Refund("general", 1); err != nil {
		t.Fatalf("capacity Refund: %v", err)
	}
	if err := caps.Save(c); err != nil {
		t.Fatalf("Save capacity: %v", err)
	}

	f := cancelevent.Feature{Events: evs, Orders: ords, Capacities: caps}
	if _, err := f.Do("jazz-trio"); err != nil {
		t.Fatalf("Do: %v", err)
	}

	// The ticket already given back was not given back twice.
	stored, err := ords.Find("ord-2")
	if err != nil {
		t.Fatalf("Find ord-2: %v", err)
	}
	if stored.Refunded("general") != 2 {
		t.Fatalf("refunded = %d, want the 2 tickets bought, no more", stored.Refunded("general"))
	}
	general := count(t, caps, "general")
	if (general != capacity.Count{Seats: 10, Remaining: 10}) {
		t.Fatalf("general after cancelling = %+v, want all 10 free exactly once", general)
	}
}

func TestCancelNeedsTheLedgerToKnowTheEvent(t *testing.T) {
	evs, ords, _ := show(t)
	f := cancelevent.Feature{Events: evs, Orders: ords, Capacities: capacities{}}
	if _, err := f.Do("jazz-trio"); !errors.Is(err, capacity.ErrCapacityUnknown) {
		t.Fatalf("Do without a count = %v, want ErrCapacityUnknown", err)
	}
	stored, err := evs.Find("jazz-trio")
	if err != nil || !stored.OnSale() {
		t.Fatalf("a refused cancel took the event off sale: %v, status=%v", err, stored.Status())
	}
	o, err := ords.Find("ord-1")
	if err != nil {
		t.Fatalf("Find ord-1: %v", err)
	}
	if len(o.Refunds()) != 0 {
		t.Fatalf("a refused cancel refunded tickets: %+v", o.Refunds())
	}
}
