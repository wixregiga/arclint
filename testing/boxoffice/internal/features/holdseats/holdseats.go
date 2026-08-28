// Package holdseats is the capacity use case that sets seats aside
// while someone is still deciding, before any Order exists.
package holdseats

import (
	"boxoffice/internal/entities/capacity"
	"boxoffice/internal/entities/event"
	"errors"
	"time"
)

// ErrEventNotOnSale refuses holds on anything that is not selling: a
// draft Event is invisible to ordering and capacity alike, and a
// cancelled one has stopped selling for good.
var ErrEventNotOnSale = errors.New("holdseats: event is not on sale")

// Feature wires the use case; HoldFor is how long someone may keep
// deciding before the Hold expires.
type Feature struct {
	Events     event.Repository
	Capacities capacity.Repository
	HoldFor    time.Duration
}

// Held is the receipt: which Hold, on what, and until when.
type Held struct {
	HoldID   string
	TierName string
	Seats    int
	Deadline time.Time
}

// Do sets seats aside on one tier of a published Event.
func (f Feature) Do(holdID, eventID, tier string, seats int, now time.Time) (Held, error) {
	ev, err := f.Events.Find(eventID)
	if err != nil {
		return Held{}, err
	}
	if !ev.OnSale() {
		return Held{}, ErrEventNotOnSale
	}
	c, err := f.Capacities.Find(eventID)
	if err != nil {
		return Held{}, err
	}
	deadline := now.Add(f.HoldFor)
	if err := c.PlaceHold(holdID, tier, seats, deadline, now); err != nil {
		return Held{}, err
	}
	if err := f.Capacities.Save(c); err != nil {
		return Held{}, err
	}
	return Held{HoldID: holdID, TierName: tier, Seats: seats, Deadline: deadline}, nil
}
