package refundticket_test

import (
	"boxoffice/internal/entities/capacity"
	"boxoffice/internal/entities/order"
	"boxoffice/internal/features/refundticket"
	"errors"
	"testing"
	"time"
)

var (
	t0       = time.Date(2026, 8, 28, 19, 0, 0, 0, time.UTC)
	attendee = order.Attendee{Name: "Sam", Email: "sam@example.com"}
)

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

// sold is one placed deal, three general tickets out of four seats,
// with the ledger already counting them as spoken for.
func sold(t *testing.T) (orders, capacities) {
	t.Helper()
	o, err := order.New("ord-1", "jazz-trio", attendee, []order.Line{
		{TierName: "general", Quantity: 3, UnitCents: 2200},
	})
	if err != nil {
		t.Fatalf("order.New: %v", err)
	}
	c, err := capacity.New("jazz-trio")
	if err != nil {
		t.Fatalf("capacity.New: %v", err)
	}
	if err := c.OpenTier("general", 4); err != nil {
		t.Fatalf("OpenTier: %v", err)
	}
	if err := c.PlaceHold("h1", "general", 3, t0.Add(time.Minute), t0); err != nil {
		t.Fatalf("PlaceHold: %v", err)
	}
	if _, err := c.CommitAll([]string{"h1"}, t0); err != nil {
		t.Fatalf("CommitAll: %v", err)
	}
	return orders{"ord-1": o}, capacities{"jazz-trio": c}
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

func TestRefundRecordsTheRefundAndFreesTheSeats(t *testing.T) {
	ords, caps := sold(t)
	f := refundticket.Feature{Orders: ords, Capacities: caps}

	o, err := f.Do("ord-1", "general", 1)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if o.Outstanding("general") != 2 {
		t.Fatalf("outstanding = %d, want 2", o.Outstanding("general"))
	}

	stored, err := ords.Find("ord-1")
	if err != nil {
		t.Fatalf("Find order: %v", err)
	}
	if stored.Refunded("general") != 1 {
		t.Fatalf("the refund was not persisted: refunded = %d", stored.Refunded("general"))
	}
	if stored.TotalCents() != 3*2200 {
		t.Fatalf("TotalCents = %d; the deal as struck never moves", stored.TotalCents())
	}
	if stored.OutstandingCents() != 2*2200 {
		t.Fatalf("OutstandingCents = %d, want %d", stored.OutstandingCents(), 2*2200)
	}
	if left := remaining(t, caps, "general"); left != 2 {
		t.Fatalf("remaining = %d, want 2 (one seat back on top of the one never sold)", left)
	}
}

func TestRefundedSeatsCanBeSoldAgain(t *testing.T) {
	ords, caps := sold(t)
	f := refundticket.Feature{Orders: ords, Capacities: caps}
	if _, err := f.Do("ord-1", "general", 3); err != nil {
		t.Fatalf("Do: %v", err)
	}

	c, err := caps.Find("jazz-trio")
	if err != nil {
		t.Fatalf("Find capacity: %v", err)
	}
	if err := c.PlaceHold("h2", "general", 4, t0.Add(time.Minute), t0); err != nil {
		t.Fatalf("holding every seat after a full refund: %v", err)
	}
}

func TestRefundRefusalsChangeNothing(t *testing.T) {
	cases := []struct {
		name     string
		orderID  string
		tier     string
		quantity int
		want     error
	}{
		{"unknown order", "nope", "general", 1, order.ErrOrderUnknown},
		{"no tickets asked for", "ord-1", "general", 0, order.ErrRefundQuantityUnreal},
		{"a tier the order never bought", "ord-1", "balcony", 1, order.ErrRefundTierUnsold},
		{"more than the deal struck", "ord-1", "general", 4, order.ErrRefundTooMany},
	}
	for _, c := range cases {
		ords, caps := sold(t)
		f := refundticket.Feature{Orders: ords, Capacities: caps}
		if _, err := f.Do(c.orderID, c.tier, c.quantity); !errors.Is(err, c.want) {
			t.Errorf("%s: Do = %v, want %v", c.name, err, c.want)
			continue
		}
		stored, err := ords.Find("ord-1")
		if err != nil {
			t.Fatalf("%s: Find order: %v", c.name, err)
		}
		if len(stored.Refunds()) != 0 {
			t.Errorf("%s: a refused refund was recorded: %+v", c.name, stored.Refunds())
		}
		if left := remaining(t, caps, "general"); left != 1 {
			t.Errorf("%s: a refused refund freed seats: remaining = %d, want 1", c.name, left)
		}
	}
}

func TestRefundNeedsTheLedgerToKnowTheEvent(t *testing.T) {
	ords, _ := sold(t)
	f := refundticket.Feature{Orders: ords, Capacities: capacities{}}
	if _, err := f.Do("ord-1", "general", 1); !errors.Is(err, capacity.ErrCapacityUnknown) {
		t.Fatalf("Do without a count = %v, want ErrCapacityUnknown", err)
	}
	stored, err := ords.Find("ord-1")
	if err != nil {
		t.Fatalf("Find order: %v", err)
	}
	if len(stored.Refunds()) != 0 {
		t.Fatalf("the order was refunded without the ledger agreeing: %+v", stored.Refunds())
	}
}
