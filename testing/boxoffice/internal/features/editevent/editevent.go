// Package editevent is the catalog use case that reshapes a draft
// Event: its story, its when and where, and its TicketTiers with
// the seats the room gives each. The aggregate refuses published
// Events; the count is rebuilt from the saved list, which is safe
// because a draft has no Holds or Orders to honor.
package editevent

import (
	"boxoffice/internal/entities/capacity"
	"boxoffice/internal/entities/event"
)

// Tier is one TicketTier as the editor saves it: the name, the
// Price, and how many seats the room gives it.
type Tier struct {
	Name  string
	Price event.Price
	Seats int
}

// Feature wires the use case.
type Feature struct {
	Events     event.Repository
	Capacities capacity.Repository
}

// Do reshapes the draft and recounts its room, then persists both.
// Any refusal along the way leaves the stored Event and Capacity
// untouched.
func (f Feature) Do(eventID, story, when, where string, tiers []Tier) (event.Event, error) {
	ev, err := f.Events.Find(eventID)
	if err != nil {
		return event.Event{}, err
	}
	if err := ev.Tell(story, when, where); err != nil {
		return event.Event{}, err
	}
	list := make([]event.TicketTier, 0, len(tiers))
	for _, t := range tiers {
		list = append(list, event.TicketTier{Name: t.Name, Price: t.Price})
	}
	if err := ev.ReplaceTiers(list); err != nil {
		return event.Event{}, err
	}
	c, err := capacity.New(eventID)
	if err != nil {
		return event.Event{}, err
	}
	for _, t := range tiers {
		if err := c.OpenTier(t.Name, t.Seats); err != nil {
			return event.Event{}, err
		}
	}
	if err := f.Events.Save(ev); err != nil {
		return event.Event{}, err
	}
	if err := f.Capacities.Save(c); err != nil {
		return event.Event{}, err
	}
	return ev, nil
}
