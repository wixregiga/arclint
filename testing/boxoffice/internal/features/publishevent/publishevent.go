// Package publishevent is the catalog use case that flips a draft
// Event to published; the aggregate itself refuses drafts that are
// not ready to sell.
package publishevent

import "boxoffice/internal/entities/event"

// Feature wires the use case.
type Feature struct {
	Events event.Repository
}

// Do publishes the Event and persists the flip.
func (f Feature) Do(eventID string) (event.Event, error) {
	ev, err := f.Events.Find(eventID)
	if err != nil {
		return event.Event{}, err
	}
	if err := ev.Publish(); err != nil {
		return event.Event{}, err
	}
	if err := f.Events.Save(ev); err != nil {
		return event.Event{}, err
	}
	return ev, nil
}
