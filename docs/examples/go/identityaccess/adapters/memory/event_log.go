package memory

import (
	"context"
	"sync"

	"example.com/saasovation/identityaccess/event"
)

// EventLog implements the use cases' EventPublisher port by appending to
// an in-memory log that tests and local tooling read back.
type EventLog struct {
	mu        sync.Mutex
	published []event.Event
}

// Publish appends the events to the log.
func (l *EventLog) Publish(_ context.Context, events []event.Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.published = append(l.published, events...)
	return nil
}

// Published returns a copy of everything published so far.
func (l *EventLog) Published() []event.Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]event.Event, len(l.published))
	copy(out, l.published)
	return out
}
