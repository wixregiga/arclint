package capacity

import "errors"

// ErrCapacityUnknown is the repository's refusal for an event it
// keeps no count for.
var ErrCapacityUnknown = errors.New("capacity: no count for that event")

// Repository stores and finds Capacities by their Event. It hands
// out clones; implementations live outside the entities layer.
type Repository interface {
	Save(Capacity) error
	Find(eventID string) (Capacity, error)
}
