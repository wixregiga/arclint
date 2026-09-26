// Package event holds the domain event contract of the identityaccess
// context and the recorder an aggregate root embeds to raise events.
package event

import "time"

// Event is something that happened in the identityaccess domain that
// domain experts care about. Every event carries the instant it occurred.
type Event interface {
	OccurredOn() time.Time
}

// Recorder collects the events an aggregate raised during one operation.
// A root embeds it; the use case that ran the operation drains it and
// publishes what it holds after the aggregate is saved.
type Recorder struct {
	pending []Event
}

// Raise records one event.
func (r *Recorder) Raise(e Event) {
	r.pending = append(r.pending, e)
}

// Events returns and forgets every recorded event, in raising order.
func (r *Recorder) Events() []Event {
	out := r.pending
	r.pending = nil
	return out
}
