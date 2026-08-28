// Package refundticket is the use case that gives one buyer's
// tickets back: the Order records the Refund beside the deal it
// struck, and the capacity ledger stops counting those seats as
// spoken for. Ordering and capacity meet here through plain calls,
// the way they do when an Order is placed.
package refundticket

import (
	"boxoffice/internal/entities/capacity"
	"boxoffice/internal/entities/order"
)

// Feature wires the use case.
type Feature struct {
	Orders     order.Repository
	Capacities capacity.Repository
}

// Do gives quantity tickets of one TicketTier back on one Order.
// Both aggregates decide for themselves whether they can: the Order
// refuses tickets it never sold or already refunded, the ledger
// refuses seats that are not spoken for. Nothing is persisted until
// both agree, so a refusal changes nothing.
func (f Feature) Do(orderID, tier string, quantity int) (order.Order, error) {
	o, err := f.Orders.Find(orderID)
	if err != nil {
		return order.Order{}, err
	}
	if err := o.Refund(tier, quantity); err != nil {
		return order.Order{}, err
	}
	c, err := f.Capacities.Find(o.EventID())
	if err != nil {
		return order.Order{}, err
	}
	if err := c.Refund(tier, quantity); err != nil {
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
