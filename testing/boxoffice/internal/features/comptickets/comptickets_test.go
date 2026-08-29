package comptickets_test

import (
	"boxoffice/internal/entities/capacity"
	"boxoffice/internal/entities/event"
	"boxoffice/internal/entities/order"
	"boxoffice/internal/features/comptickets"
	"errors"
	"testing"
	"time"
)

var (
	t0    = time.Date(2026, 8, 28, 19, 0, 0, 0, time.UTC)
	guest = order.Attendee{Name: "Rae Mensah", Email: "rae@example.com"}
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

func (e events) All() ([]event.Event, error) {
	out := []event.Event{}
	for _, ev := range e {
		out = append(out, ev)
	}
	return out, nil
}

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

// onSale is the jazz trio published with four general seats and two
// front-row, and an empty order book.
func onSale(t *testing.T) (events, orders, capacities) {
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
	if err := c.OpenTier("general", 4); err != nil {
		t.Fatalf("OpenTier general: %v", err)
	}
	if err := c.OpenTier("front-row", 2); err != nil {
		t.Fatalf("OpenTier front-row: %v", err)
	}
	return events{"jazz-trio": ev}, orders{}, capacities{"jazz-trio": c}
}

func remaining(t *testing.T, caps capacities, tier string) int {
	t.Helper()
	c, err := caps.Find("jazz-trio")
	if err != nil {
		t.Fatalf("Find capacity: %v", err)
	}
	left, err := c.Remaining(tier, t0)
	if err != nil {
		t.Fatalf("Remaining: %v", err)
	}
	return left
}

func TestCompPlacesAFreeOrderAndSpeaksForTheSeats(t *testing.T) {
	evs, ords, caps := onSale(t)
	f := comptickets.Feature{Events: evs, Orders: ords, Capacities: caps}

	o, err := f.Do("comp-1", "jazz-trio", guest, []comptickets.Ticket{
		{TierName: "general", Quantity: 2},
		{TierName: "front-row", Quantity: 1},
	}, t0)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !o.Comped() {
		t.Errorf("a comped order does not read as comped: %+v", o.Lines())
	}
	if o.TotalCents() != 0 || o.OutstandingCents() != 0 {
		t.Errorf("a comped order costs %d (owes %d), want nothing", o.TotalCents(), o.OutstandingCents())
	}
	if o.Attendee().Name != "Rae Mensah" {
		t.Errorf("attendee = %+v, want the person comped", o.Attendee())
	}

	stored, err := ords.Find("comp-1")
	if err != nil {
		t.Fatalf("the comp was not persisted: %v", err)
	}
	if stored.Outstanding("general") != 2 || stored.Outstanding("front-row") != 1 {
		t.Errorf("stored comp holds %+v", stored.Lines())
	}
	// Promised like any other: the seats are spoken for, not held.
	if left := remaining(t, caps, "general"); left != 2 {
		t.Errorf("general remaining = %d, want 2", left)
	}
	if left := remaining(t, caps, "front-row"); left != 1 {
		t.Errorf("front-row remaining = %d, want 1", left)
	}
}

func TestCompCannotOutrunTheRoom(t *testing.T) {
	evs, ords, caps := onSale(t)
	f := comptickets.Feature{Events: evs, Orders: ords, Capacities: caps}

	if _, err := f.Do("comp-1", "jazz-trio", guest,
		[]comptickets.Ticket{{TierName: "general", Quantity: 5}}, t0); !errors.Is(err, capacity.ErrSeatsExhausted) {
		t.Fatalf("comping 5 of 4 seats = %v, want ErrSeatsExhausted", err)
	}
	if left := remaining(t, caps, "general"); left != 4 {
		t.Errorf("a refused comp took seats: remaining = %d, want 4", left)
	}
	if _, err := ords.Find("comp-1"); !errors.Is(err, order.ErrOrderUnknown) {
		t.Errorf("a refused comp left an order behind")
	}
}

func TestCompCompetesWithHoldsAndSales(t *testing.T) {
	evs, ords, caps := onSale(t)
	f := comptickets.Feature{Events: evs, Orders: ords, Capacities: caps}

	// Someone is deciding on three of the four general seats.
	c, err := caps.Find("jazz-trio")
	if err != nil {
		t.Fatalf("Find capacity: %v", err)
	}
	if err := c.PlaceHold("h1", "general", 3, t0.Add(time.Minute), t0); err != nil {
		t.Fatalf("PlaceHold: %v", err)
	}
	if err := caps.Save(c); err != nil {
		t.Fatalf("Save capacity: %v", err)
	}

	// Only one seat is really free, so two cannot be comped.
	if _, err := f.Do("comp-1", "jazz-trio", guest,
		[]comptickets.Ticket{{TierName: "general", Quantity: 2}}, t0); !errors.Is(err, capacity.ErrSeatsExhausted) {
		t.Fatalf("comping into held seats = %v, want ErrSeatsExhausted", err)
	}
	if _, err := f.Do("comp-1", "jazz-trio", guest,
		[]comptickets.Ticket{{TierName: "general", Quantity: 1}}, t0); err != nil {
		t.Fatalf("comping the one free seat: %v", err)
	}
	if left := remaining(t, caps, "general"); left != 0 {
		t.Errorf("remaining = %d, want the room full", left)
	}
}

func TestCompRefusalsChangeNothing(t *testing.T) {
	cases := []struct {
		name    string
		eventID string
		guest   order.Attendee
		tickets []comptickets.Ticket
		want    error
	}{
		{
			"an event nobody created", "nope", guest,
			[]comptickets.Ticket{{TierName: "general", Quantity: 1}},
			event.ErrEventUnknown,
		},
		{
			"no tickets at all", "jazz-trio", guest,
			[]comptickets.Ticket{},
			comptickets.ErrNothingToComp,
		},
		{
			"a tier the event never sold", "jazz-trio", guest,
			[]comptickets.Ticket{{TierName: "balcony", Quantity: 1}},
			comptickets.ErrTierNotSold,
		},
		{
			"no tickets on the tier", "jazz-trio", guest,
			[]comptickets.Ticket{{TierName: "general", Quantity: 0}},
			capacity.ErrSeatsInvalid,
		},
		{
			"nobody to comp them for", "jazz-trio",
			order.Attendee{Name: "Rae"},
			[]comptickets.Ticket{{TierName: "general", Quantity: 1}},
			order.ErrAttendeeMissing,
		},
		{
			"the same tier twice", "jazz-trio", guest,
			[]comptickets.Ticket{{TierName: "general", Quantity: 1}, {TierName: "general", Quantity: 1}},
			order.ErrLineDuplicate,
		},
	}
	for _, c := range cases {
		evs, ords, caps := onSale(t)
		f := comptickets.Feature{Events: evs, Orders: ords, Capacities: caps}
		if _, err := f.Do("comp-1", c.eventID, c.guest, c.tickets, t0); !errors.Is(err, c.want) {
			t.Errorf("%s: Do = %v, want %v", c.name, err, c.want)
			continue
		}
		if left := remaining(t, caps, "general"); left != 4 {
			t.Errorf("%s: a refused comp took seats: remaining = %d, want 4", c.name, left)
		}
		if _, err := ords.Find("comp-1"); !errors.Is(err, order.ErrOrderUnknown) {
			t.Errorf("%s: a refused comp left an order behind", c.name)
		}
	}
}

func TestOnlyAShowOnSaleCanBeComped(t *testing.T) {
	// A draft has not started selling.
	draft, err := event.New("winter-recital", "Winter Recital")
	if err != nil {
		t.Fatalf("event.New: %v", err)
	}
	c, err := capacity.New("winter-recital")
	if err != nil {
		t.Fatalf("capacity.New: %v", err)
	}
	if err := c.OpenTier("general", 10); err != nil {
		t.Fatalf("OpenTier: %v", err)
	}
	ords := orders{}
	f := comptickets.Feature{
		Events:     events{"winter-recital": draft},
		Orders:     ords,
		Capacities: capacities{"winter-recital": c},
	}
	if _, err := f.Do("comp-1", "winter-recital", guest,
		[]comptickets.Ticket{{TierName: "general", Quantity: 1}}, t0); !errors.Is(err, comptickets.ErrEventNotOnSale) {
		t.Fatalf("comping a draft = %v, want ErrEventNotOnSale", err)
	}

	// A cancelled show has stopped selling for good.
	evs, ords, caps := onSale(t)
	cancelled, err := evs.Find("jazz-trio")
	if err != nil {
		t.Fatalf("Find event: %v", err)
	}
	if err := cancelled.Cancel(); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if err := evs.Save(cancelled); err != nil {
		t.Fatalf("Save event: %v", err)
	}
	f = comptickets.Feature{Events: evs, Orders: ords, Capacities: caps}
	if _, err := f.Do("comp-1", "jazz-trio", guest,
		[]comptickets.Ticket{{TierName: "general", Quantity: 1}}, t0); !errors.Is(err, comptickets.ErrEventNotOnSale) {
		t.Fatalf("comping a cancelled show = %v, want ErrEventNotOnSale", err)
	}
}

func TestCompedTicketsComeBackLikeAnyOther(t *testing.T) {
	evs, ords, caps := onSale(t)
	f := comptickets.Feature{Events: evs, Orders: ords, Capacities: caps}
	o, err := f.Do("comp-1", "jazz-trio", guest,
		[]comptickets.Ticket{{TierName: "general", Quantity: 2}}, t0)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	// The organizer changes their mind: the same Refund every other
	// ticket takes, with no comp-shaped door of its own.
	if err := o.Refund("general", 1); err != nil {
		t.Fatalf("refunding a comped ticket: %v", err)
	}
	if o.Outstanding("general") != 1 {
		t.Fatalf("outstanding = %d, want 1", o.Outstanding("general"))
	}
	if !o.Comped() {
		t.Errorf("a refunded comp stopped reading as comped")
	}
	c, err := caps.Find("jazz-trio")
	if err != nil {
		t.Fatalf("Find capacity: %v", err)
	}
	if err := c.Refund("general", 1); err != nil {
		t.Fatalf("the ledger refused a comped seat back: %v", err)
	}
	left, err := c.Remaining("general", t0)
	if err != nil {
		t.Fatalf("Remaining: %v", err)
	}
	if left != 3 {
		t.Fatalf("remaining after giving one comped seat back = %d, want 3", left)
	}
}
