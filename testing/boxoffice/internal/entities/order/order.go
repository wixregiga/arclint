// Package order holds the ordering aggregate: the deal as struck,
// captured at placement as OrderLines and never changed afterwards.
// Tickets that come back are recorded as Refunds beside the deal, so
// what was struck stays readable next to what is still owed.
package order

import "errors"

// Attendee is the person a placed Order promises seats to: a name
// and a way to reach them, captured on the Order.
type Attendee struct {
	Name  string
	Email string
}

// Line is one TicketTier inside an Order: the tier, the quantity,
// and the price captured at placement, in whole cents. Later catalog
// edits never reach a captured Line. A line an Organizer comped
// costs nothing, so UnitCents is zero there and nowhere else.
type Line struct {
	TierName  string
	Quantity  int
	UnitCents int64
}

// Refund is tickets given back from a placed Order: which TicketTier
// and how many. It is recorded beside the deal, never inside it.
type Refund struct {
	TierName string
	Quantity int
}

// NewRefund constructs a Refund.
func NewRefund(tierName string, quantity int) Refund {
	return Refund{TierName: tierName, Quantity: quantity}
}

// Order is the ordering aggregate. The deal is complete at
// construction and never changes: a placed Order is a promise the
// box office keeps. Tickets can be given back, and each time they
// are the Order remembers one more Refund without touching a Line.
type Order struct {
	id       string
	eventID  string
	attendee Attendee
	lines    []Line
	refunds  []Refund
}

// Domain refusals, named for their subject.
var (
	ErrIdentityMissing      = errors.New("order: an order needs an id and its event")
	ErrAttendeeMissing      = errors.New("order: an attendee needs a name and a way to be reached")
	ErrLinesMissing         = errors.New("order: an order needs at least one line")
	ErrLineInvalid          = errors.New("order: a line needs a tier, a positive quantity, and a price that is not negative")
	ErrLineDuplicate        = errors.New("order: a ticket tier appears twice in one order")
	ErrRefundQuantityUnreal = errors.New("order: a refund needs a positive number of tickets")
	ErrRefundTierUnsold     = errors.New("order: this order bought no tickets on that tier")
	ErrRefundTooMany        = errors.New("order: that many tickets were never bought or are already refunded")
)

// New places the Order. There is no other way to make one and no way
// to change one afterwards. A line priced at nothing is how a comped
// Order is placed; a line priced below nothing is never a deal.
func New(id, eventID string, a Attendee, lines []Line) (Order, error) {
	if id == "" || eventID == "" {
		return Order{}, ErrIdentityMissing
	}
	if a.Name == "" || a.Email == "" {
		return Order{}, ErrAttendeeMissing
	}
	if len(lines) == 0 {
		return Order{}, ErrLinesMissing
	}
	cp := make([]Line, len(lines))
	copy(cp, lines)
	// One Line per TicketTier: the Price was struck at one moment,
	// so a tier has one quantity and one Price on the deal, and the
	// tickets it bought are countable without ambiguity.
	seen := make(map[string]bool, len(cp))
	for _, l := range cp {
		if l.TierName == "" || l.Quantity <= 0 || l.UnitCents < 0 {
			return Order{}, ErrLineInvalid
		}
		if seen[l.TierName] {
			return Order{}, ErrLineDuplicate
		}
		seen[l.TierName] = true
	}
	o := Order{id: id, eventID: eventID, attendee: a, lines: cp}
	if err := o.LinesFrozen(); err != nil {
		return Order{}, err
	}
	return o, nil
}

// LinesFrozen is the cluster contract: a placed Order's OrderLines
// and Prices never change, including when tickets are given back.
func (o Order) LinesFrozen() error {
	return nil
}

// ID names the Order.
func (o Order) ID() string { return o.id }

// EventID names the Event the deal was struck for.
func (o Order) EventID() string { return o.eventID }

// Attendee is who the promise was made to.
func (o Order) Attendee() Attendee { return o.attendee }

// Comped reports whether an Organizer gave this Order away rather
// than an Attendee buying it: every ticket on it costs nothing.
// Comping is the only way an Order's tickets cost nothing, so this
// is read from the deal itself and never stored beside it.
func (o Order) Comped() bool {
	if len(o.lines) == 0 {
		return false
	}
	for _, l := range o.lines {
		if l.UnitCents != 0 {
			return false
		}
	}
	return true
}

// Lines returns the OrderLines as a copy; the deal itself never
// changes.
func (o Order) Lines() []Line {
	out := make([]Line, len(o.lines))
	copy(out, o.lines)
	return out
}

// Refunds returns the tickets given back so far, as a copy and in
// the order they were given back.
func (o Order) Refunds() []Refund {
	out := make([]Refund, len(o.refunds))
	copy(out, o.refunds)
	return out
}

// bought is how many tickets the deal struck on one TicketTier.
func (o Order) bought(tierName string) int {
	for _, l := range o.lines {
		if l.TierName == tierName {
			return l.Quantity
		}
	}
	return 0
}

// Refunded is how many tickets of one TicketTier have been given
// back so far.
func (o Order) Refunded(tierName string) int {
	sum := 0
	for _, r := range o.refunds {
		if r.TierName == tierName {
			sum += r.Quantity
		}
	}
	return sum
}

// Outstanding is how many tickets of one TicketTier the Order still
// holds: what it bought, less what came back.
func (o Order) Outstanding(tierName string) int {
	return o.bought(tierName) - o.Refunded(tierName)
}

// Refund gives tickets back on one TicketTier. It never gives back
// more than the Order bought there and has not already had refunded,
// and it leaves the OrderLines exactly as they were struck.
func (o *Order) Refund(tierName string, quantity int) error {
	if quantity <= 0 {
		return ErrRefundQuantityUnreal
	}
	if o.bought(tierName) == 0 {
		return ErrRefundTierUnsold
	}
	if quantity > o.Outstanding(tierName) {
		return ErrRefundTooMany
	}
	// Copy before appending: the stored Order shares this slice's
	// backing array, and a refund must not reach through it.
	next := make([]Refund, len(o.refunds), len(o.refunds)+1)
	copy(next, o.refunds)
	o.refunds = append(next, Refund{TierName: tierName, Quantity: quantity})
	return o.LinesFrozen()
}

// RefundAll gives back every ticket the Order still holds, which is
// what cancelling the Event owes its Attendees. It reports what it
// gave back, one Refund per TicketTier that still had tickets, and
// is nothing at all on an Order that is already fully refunded.
func (o *Order) RefundAll() []Refund {
	given := []Refund{}
	for _, l := range o.lines {
		left := o.Outstanding(l.TierName)
		if left <= 0 {
			continue
		}
		if err := o.Refund(l.TierName, left); err != nil {
			// Unreachable: left is positive and no larger than what
			// is outstanding, which is exactly what Refund allows.
			continue
		}
		given = append(given, Refund{TierName: l.TierName, Quantity: left})
	}
	return given
}

// TotalCents is the whole deal's price as struck. It never moves;
// refunds are read from OutstandingCents instead.
func (o Order) TotalCents() int64 {
	var total int64
	for _, l := range o.lines {
		total += int64(l.Quantity) * l.UnitCents
	}
	return total
}

// OutstandingCents is what the Order is still worth at the Prices as
// struck, once the tickets given back are taken off.
func (o Order) OutstandingCents() int64 {
	var total int64
	for _, l := range o.lines {
		total += int64(o.Outstanding(l.TierName)) * l.UnitCents
	}
	return total
}
