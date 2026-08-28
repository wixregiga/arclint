// Package cancelevent is the use case where the Organizer calls a
// published show off: the Event goes off sale for good, every ticket
// sold for it is given back, and the seats return to the room. All
// three contexts meet here through plain calls.
package cancelevent

import (
	"boxoffice/internal/entities/capacity"
	"boxoffice/internal/entities/event"
	"boxoffice/internal/entities/order"
)

// Feature wires the use case.
type Feature struct {
	Events     event.Repository
	Orders     order.Repository
	Capacities capacity.Repository
}

// Do cancels the Event and refunds every ticket still outstanding on
// every Order placed for it. The Event decides whether it may be
// cancelled at all; each Order decides what it still owes back; the
// ledger decides whether those seats were really spoken for. Nothing
// is persisted until all of them agree, so a refusal leaves the box
// office exactly as it was.
func (f Feature) Do(eventID string) (event.Event, error) {
	ev, err := f.Events.Find(eventID)
	if err != nil {
		return event.Event{}, err
	}
	if err := ev.Cancel(); err != nil {
		return event.Event{}, err
	}
	orders, err := f.Orders.ForEvent(eventID)
	if err != nil {
		return event.Event{}, err
	}
	c, err := f.Capacities.Find(eventID)
	if err != nil {
		return event.Event{}, err
	}
	refunded := make([]order.Order, 0, len(orders))
	for _, o := range orders {
		given := o.RefundAll()
		if len(given) == 0 {
			continue
		}
		for _, r := range given {
			if err := c.Refund(r.TierName, r.Quantity); err != nil {
				return event.Event{}, err
			}
		}
		refunded = append(refunded, o)
	}
	// Nobody is still deciding about a show that is off.
	c.ReleaseHolds()

	if err := f.Events.Save(ev); err != nil {
		return event.Event{}, err
	}
	if err := f.Capacities.Save(c); err != nil {
		return event.Event{}, err
	}
	for _, o := range refunded {
		if err := f.Orders.Save(o); err != nil {
			return event.Event{}, err
		}
	}
	return ev, nil
}
