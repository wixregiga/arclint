// Package placeorder is the use case where ordering and capacity
// meet: commit the Holds, capture Prices as struck, place the Order.
// The contexts touch here through plain calls, never through
// messaging machinery.
package placeorder

import (
	"boxoffice/internal/entities/capacity"
	"boxoffice/internal/entities/event"
	"boxoffice/internal/entities/order"
	"errors"
	"time"
)

// Use-case refusals.
var (
	ErrEventNotOnSale = errors.New("placeorder: event is not on sale")
	ErrTierNotSold    = errors.New("placeorder: a held tier is not sold by this event")
)

// Feature wires the use case.
type Feature struct {
	Events     event.Repository
	Orders     order.Repository
	Capacities capacity.Repository
}

// Do strikes the deal: every named Hold becomes seats spoken for and
// one Order captures the Prices as they stand this moment. Nothing
// is persisted unless everything holds, so a refusal changes
// nothing.
func (f Feature) Do(orderID, eventID string, holdIDs []string, attendee order.Attendee, now time.Time) (order.Order, error) {
	ev, err := f.Events.Find(eventID)
	if err != nil {
		return order.Order{}, err
	}
	if !ev.OnSale() {
		return order.Order{}, ErrEventNotOnSale
	}
	c, err := f.Capacities.Find(eventID)
	if err != nil {
		return order.Order{}, err
	}
	committed, err := c.CommitAll(holdIDs, now)
	if err != nil {
		return order.Order{}, err
	}

	quantities := map[string]int{}
	names := make([]string, 0, len(committed))
	for _, h := range committed {
		if _, seen := quantities[h.TierName]; !seen {
			names = append(names, h.TierName)
		}
		quantities[h.TierName] += h.Seats
	}
	lines := make([]order.Line, 0, len(names))
	for _, name := range names {
		t, ok := ev.Tier(name)
		if !ok {
			return order.Order{}, ErrTierNotSold
		}
		lines = append(lines, order.Line{TierName: name, Quantity: quantities[name], UnitCents: int64(t.Price)})
	}

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
