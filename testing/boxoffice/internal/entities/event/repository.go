package event

import "errors"

// ErrEventUnknown is the repository's refusal for an id it has never
// seen.
var ErrEventUnknown = errors.New("event: unknown event")

// Repository stores and finds Events. Implementations live outside
// the entities layer; the interface belongs to the domain.
type Repository interface {
	Save(Event) error
	Find(id string) (Event, error)
	All() ([]Event, error)
}
