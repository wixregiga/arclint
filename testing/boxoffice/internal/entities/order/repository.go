package order

import "errors"

// ErrOrderUnknown is the repository's refusal for an id it has never
// seen.
var ErrOrderUnknown = errors.New("order: unknown order")

// Repository stores and finds Orders. Implementations live outside
// the entities layer; the interface belongs to the domain.
type Repository interface {
	Save(Order) error
	Find(id string) (Order, error)
	// ForEvent returns every Order placed for one Event, which is
	// how the Organizer sees who bought tickets and how cancelling
	// reaches every deal it owes a refund.
	ForEvent(eventID string) ([]Order, error)
}
