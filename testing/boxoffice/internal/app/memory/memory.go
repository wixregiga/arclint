// Package memory implements the entity repositories on plain maps:
// the whole persistence story of the self-contained box office.
package memory

import (
	"boxoffice/internal/entities/capacity"
	"boxoffice/internal/entities/event"
	"boxoffice/internal/entities/order"
	"sort"
	"sync"
)

// Events stores event aggregates in memory.
type Events struct {
	mu   sync.RWMutex
	byID map[string]event.Event
}

// NewEvents opens an empty event store.
func NewEvents() *Events { return &Events{byID: map[string]event.Event{}} }

// Save keeps the Event.
func (s *Events) Save(ev event.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[ev.ID()] = ev
	return nil
}

// Find returns one Event by id.
func (s *Events) Find(id string) (event.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ev, ok := s.byID[id]
	if !ok {
		return event.Event{}, event.ErrEventUnknown
	}
	return ev, nil
}

// All returns every Event, ordered by id for determinism.
func (s *Events) All() ([]event.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]event.Event, 0, len(s.byID))
	for _, ev := range s.byID {
		out = append(out, ev)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out, nil
}

// Orders stores order aggregates in memory.
type Orders struct {
	mu   sync.RWMutex
	byID map[string]order.Order
}

// NewOrders opens an empty order store.
func NewOrders() *Orders { return &Orders{byID: map[string]order.Order{}} }

// Save keeps the Order.
func (s *Orders) Save(o order.Order) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[o.ID()] = o
	return nil
}

// Find returns one Order by id.
func (s *Orders) Find(id string) (order.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.byID[id]
	if !ok {
		return order.Order{}, order.ErrOrderUnknown
	}
	return o, nil
}

// ForEvent returns every Order placed for one Event, ordered by id
// for determinism.
func (s *Orders) ForEvent(eventID string) ([]order.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]order.Order, 0, len(s.byID))
	for _, o := range s.byID {
		if o.EventID() == eventID {
			out = append(out, o)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out, nil
}

// Capacities stores capacity aggregates in memory. It stores and
// hands out clones so no two callers share the same maps.
type Capacities struct {
	mu      sync.RWMutex
	byEvent map[string]capacity.Capacity
}

// NewCapacities opens an empty capacity store.
func NewCapacities() *Capacities {
	return &Capacities{byEvent: map[string]capacity.Capacity{}}
}

// Save keeps a clone of the Capacity.
func (s *Capacities) Save(c capacity.Capacity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byEvent[c.EventID()] = c.Clone()
	return nil
}

// Find returns a clone of one Capacity by its event.
func (s *Capacities) Find(eventID string) (capacity.Capacity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.byEvent[eventID]
	if !ok {
		return capacity.Capacity{}, capacity.ErrCapacityUnknown
	}
	return c.Clone(), nil
}

var (
	_ event.Repository    = (*Events)(nil)
	_ order.Repository    = (*Orders)(nil)
	_ capacity.Repository = (*Capacities)(nil)
)
