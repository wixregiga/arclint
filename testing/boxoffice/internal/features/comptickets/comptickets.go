// Package comptickets is the use case where an Organizer gives
// tickets away: the seats are spoken for like any other promise, and
// the Order that comes out costs its Attendee nothing. Ordering and
// capacity meet here through plain calls, the way they do when an
// Order is bought.
package comptickets

import (
	"boxoffice/internal/entities/capacity"
	"boxoffice/internal/entities/event"
	"boxoffice/internal/entities/order"
	"errors"
	"time"
)

// Use-case refusals.
var (
	ErrEventNotOnSale = errors.New("comptickets: event is not on sale")
	ErrTierNotSold    = errors.New("comptickets: a comped tier is not sold by this event")
	ErrNothingToComp  = errors.New("comptickets: comping needs at least one ticket")
)

// Ticket is what the Organizer is giving away on one TicketTier: the
// tier and how many. No price is named, because a comped ticket
// costs its Attendee nothing.
type Ticket struct {
	TierName string
	Quantity int
}

// Feature wires the use case.
type Feature struct {
	Events     event.Repository
	Orders     order.Repository
	Capacities capacity.Repository
}

// Do comps tickets for one named person on a published Event. The
// seats are spoken for the moment they are given, so the room can
// refuse a comp it cannot fit, exactly as it refuses a hold. Nothing
// is persisted unless everything holds, so a refusal changes
// nothing.
func (f Feature) Do(orderID, eventID string, attendee order.Attendee, tickets []Ticket, now time.Time) (order.Order, error) {
	if len(tickets) == 0 {
		return order.Order{}, ErrNothingToComp
	}
	ev, err := f.Events.Find(eventID)
	if err != nil {
		return order.Order{}, err
	}
	// A draft sells nothing and a cancelled show has stopped, so
	// there is no room to give seats out of either way.
	if !ev.OnSale() {
		return order.Order{}, ErrEventNotOnSale
	}
	c, err := f.Capacities.Find(eventID)
	if err != nil {
		return order.Order{}, err
	}

	lines := make([]order.Line, 0, len(tickets))
	for _, t := range tickets {
		if _, ok := ev.Tier(t.TierName); !ok {
			return order.Order{}, ErrTierNotSold
		}
		if err := c.Commit(t.TierName, t.Quantity, now); err != nil {
			return order.Order{}, err
		}
		lines = append(lines, order.Line{TierName: t.TierName, Quantity: t.Quantity})
	}

	// order.New has the last word on the deal: one line per tier, a
	// real quantity on each, and an Attendee to promise them to.
	o, err := order.New(orderID, eventID, attendee, lines)
	if err != nil {
		return order.Order{}, err
	}
	if err := f.Capacities.Save(c); err != nil {
		return order.Order{}, err
	}
	if err := f.Orders.Save(o); err != nil {
		return order.Order{}, err
	}
	return o, nil
}
