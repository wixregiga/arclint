package order_test

import (
	"boxoffice/internal/entities/order"
	"errors"
	"testing"
)

var attendee = order.Attendee{Name: "Sam", Email: "sam@example.com"}

func TestNewRefusesIncompleteDeals(t *testing.T) {
	lines := []order.Line{{TierName: "general", Quantity: 2, UnitCents: 1500}}
	cases := []struct {
		name    string
		id      string
		eventID string
		a       order.Attendee
		lines   []order.Line
		want    error
	}{
		{"missing id", "", "open-mic-night", attendee, lines, order.ErrIdentityMissing},
		{"missing event", "ord-1", "", attendee, lines, order.ErrIdentityMissing},
		{"nameless attendee", "ord-1", "open-mic-night", order.Attendee{Email: "x@example.com"}, lines, order.ErrAttendeeMissing},
		{"unreachable attendee", "ord-1", "open-mic-night", order.Attendee{Name: "Sam"}, lines, order.ErrAttendeeMissing},
		{"no lines", "ord-1", "open-mic-night", attendee, nil, order.ErrLinesMissing},
		{"zero quantity", "ord-1", "open-mic-night", attendee, []order.Line{{TierName: "general", Quantity: 0, UnitCents: 1500}}, order.ErrLineInvalid},
		{"price below nothing", "ord-1", "open-mic-night", attendee, []order.Line{{TierName: "general", Quantity: 1, UnitCents: -1}}, order.ErrLineInvalid},
	}
	for _, c := range cases {
		if _, err := order.New(c.id, c.eventID, c.a, c.lines); !errors.Is(err, c.want) {
			t.Errorf("%s: New = %v, want %v", c.name, err, c.want)
		}
	}
}

func TestNewRefusesTheSameTierTwice(t *testing.T) {
	lines := []order.Line{
		{TierName: "general", Quantity: 2, UnitCents: 1500},
		{TierName: "general", Quantity: 1, UnitCents: 1500},
	}
	if _, err := order.New("ord-1", "jazz-trio", attendee, lines); !errors.Is(err, order.ErrLineDuplicate) {
		t.Fatalf("New with a repeated tier = %v, want ErrLineDuplicate", err)
	}
}

func TestCompedReadsTheDealNotAMarking(t *testing.T) {
	comped, err := order.New("comp-1", "jazz-trio", attendee, []order.Line{
		{TierName: "general", Quantity: 2},
		{TierName: "front-row", Quantity: 1},
	})
	if err != nil {
		t.Fatalf("New with free lines: %v", err)
	}
	if !comped.Comped() {
		t.Errorf("an order whose tickets all cost nothing does not read as comped")
	}
	if comped.TotalCents() != 0 || comped.OutstandingCents() != 0 {
		t.Errorf("a comped order costs %d and owes %d, want nothing either way",
			comped.TotalCents(), comped.OutstandingCents())
	}

	if placed(t).Comped() {
		t.Errorf("a bought order reads as comped")
	}
	// Half free is not a giveaway: the attendee still owes something.
	part, err := order.New("ord-2", "jazz-trio", attendee, []order.Line{
		{TierName: "general", Quantity: 1},
		{TierName: "front-row", Quantity: 1, UnitCents: 3000},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if part.Comped() {
		t.Errorf("an order with a priced line reads as comped")
	}
	if part.OutstandingCents() != 3000 {
		t.Errorf("outstanding = %d, want the 3000 still owed", part.OutstandingCents())
	}
}

func TestZeroValueOrderIsNotComped(t *testing.T) {
	var none order.Order
	if none.Comped() {
		t.Errorf("an order that was never placed reads as comped")
	}
}

// placed is a two-tier deal: three general and one front-row.
func placed(t *testing.T) order.Order {
	t.Helper()
	o, err := order.New("ord-1", "jazz-trio", attendee, []order.Line{
		{TierName: "general", Quantity: 3, UnitCents: 1500},
		{TierName: "front-row", Quantity: 1, UnitCents: 3000},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return o
}

func TestRefundLeavesTheDealAsStruck(t *testing.T) {
	o := placed(t)
	if err := o.Refund("general", 1); err != nil {
		t.Fatalf("Refund: %v", err)
	}

	// The lines and their prices are exactly what was struck.
	lines := o.Lines()
	if len(lines) != 2 || lines[0].Quantity != 3 || lines[0].UnitCents != 1500 {
		t.Fatalf("a refund reached the order lines: %+v", lines)
	}
	if o.TotalCents() != 3*1500+3000 {
		t.Fatalf("TotalCents = %d; the deal as struck never moves", o.TotalCents())
	}

	// The refund is recorded beside it.
	if got := o.Refunded("general"); got != 1 {
		t.Fatalf("Refunded(general) = %d, want 1", got)
	}
	if got := o.Outstanding("general"); got != 2 {
		t.Fatalf("Outstanding(general) = %d, want 2", got)
	}
	if got := o.OutstandingCents(); got != 2*1500+3000 {
		t.Fatalf("OutstandingCents = %d, want %d", got, 2*1500+3000)
	}
	refunds := o.Refunds()
	if len(refunds) != 1 || refunds[0].TierName != "general" || refunds[0].Quantity != 1 {
		t.Fatalf("Refunds = %+v", refunds)
	}
	// The returned slice is a copy.
	refunds[0].Quantity = 99
	if o.Refunded("general") != 1 {
		t.Fatal("mutating the returned refunds reached the aggregate")
	}
}

func TestRefundNeverGivesBackMoreThanWasBought(t *testing.T) {
	o := placed(t)
	cases := []struct {
		name     string
		tier     string
		quantity int
		want     error
	}{
		{"no tickets asked for", "general", 0, order.ErrRefundQuantityUnreal},
		{"a negative refund", "general", -1, order.ErrRefundQuantityUnreal},
		{"a tier the order never bought", "balcony", 1, order.ErrRefundTierUnsold},
		{"more than the deal struck", "general", 4, order.ErrRefundTooMany},
	}
	for _, c := range cases {
		if err := o.Refund(c.tier, c.quantity); !errors.Is(err, c.want) {
			t.Errorf("%s: Refund = %v, want %v", c.name, err, c.want)
		}
	}
	if len(o.Refunds()) != 0 {
		t.Fatalf("a refused refund was recorded: %+v", o.Refunds())
	}

	// Refunds accumulate against the same bound, one ticket at a time.
	for i := range 3 {
		if err := o.Refund("general", 1); err != nil {
			t.Fatalf("Refund %d: %v", i+1, err)
		}
	}
	if err := o.Refund("general", 1); !errors.Is(err, order.ErrRefundTooMany) {
		t.Fatalf("refunding a fourth general ticket = %v, want ErrRefundTooMany", err)
	}
	if got := o.Outstanding("general"); got != 0 {
		t.Fatalf("Outstanding(general) = %d, want 0", got)
	}
}

func TestRefundAllGivesBackWhatIsLeft(t *testing.T) {
	o := placed(t)
	if err := o.Refund("general", 1); err != nil {
		t.Fatalf("Refund: %v", err)
	}

	given := o.RefundAll()
	if len(given) != 2 {
		t.Fatalf("RefundAll gave back %+v, want both tiers", given)
	}
	if given[0].TierName != "general" || given[0].Quantity != 2 {
		t.Fatalf("general given back = %+v, want the 2 still outstanding", given[0])
	}
	if given[1].TierName != "front-row" || given[1].Quantity != 1 {
		t.Fatalf("front-row given back = %+v", given[1])
	}
	if o.OutstandingCents() != 0 {
		t.Fatalf("OutstandingCents = %d, want 0 after refunding everything", o.OutstandingCents())
	}
	if o.TotalCents() != 3*1500+3000 {
		t.Fatalf("TotalCents = %d; the deal as struck never moves", o.TotalCents())
	}

	// A fully refunded order owes nothing more.
	if again := o.RefundAll(); len(again) != 0 {
		t.Fatalf("RefundAll on a refunded order gave back %+v, want nothing", again)
	}
}

func TestRefundsOnACopyDoNotReachTheOriginal(t *testing.T) {
	original := placed(t)
	if err := original.Refund("general", 1); err != nil {
		t.Fatalf("Refund: %v", err)
	}

	// Repositories hand out copies of the stored value; a refund on
	// one must not reach through the shared backing array.
	copied := original
	if err := copied.Refund("general", 2); err != nil {
		t.Fatalf("Refund on the copy: %v", err)
	}
	if got := original.Refunded("general"); got != 1 {
		t.Fatalf("the copy's refund reached the original: Refunded = %d, want 1", got)
	}
}

func TestPlacedOrderNeverChanges(t *testing.T) {
	lines := []order.Line{
		{TierName: "general", Quantity: 2, UnitCents: 1500},
		{TierName: "front-row", Quantity: 1, UnitCents: 3000},
	}
	o, err := order.New("ord-1", "jazz-trio", attendee, lines)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	lines[0].UnitCents = 1 // the caller's slice is not the order's
	got := o.Lines()
	got[1].Quantity = 99 // neither is the returned copy

	fresh := o.Lines()
	if fresh[0].UnitCents != 1500 || fresh[1].Quantity != 1 {
		t.Fatalf("order lines changed after placement: %+v", fresh)
	}
	if o.TotalCents() != 2*1500+3000 {
		t.Fatalf("TotalCents = %d, want %d", o.TotalCents(), 2*1500+3000)
	}
}
