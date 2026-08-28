// Package event holds the catalog aggregate: one show an Organizer
// puts on sale, its TicketTiers with their Prices, and the publish
// and cancel rules the box office enforces around what sells.
package event

import "errors"

// Price is what one ticket of a TicketTier costs, in whole cents of
// one currency. Zero means the tier has no Price yet.
type Price int64

// TicketTier is one named offer within an Event, like general or
// front-row: what it buys and its Price.
type TicketTier struct {
	Name  string
	Price Price
}

// Status is where an Event stands: a draft nobody but its Organizer
// sees, published and on sale, or cancelled and off sale for good.
type Status string

// The three places an Event can stand. A draft goes forward only to
// published, and a published Event only to cancelled; nothing leads
// back.
const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
	StatusCancelled Status = "cancelled"
)

// Event is the catalog aggregate. It stays a draft until published,
// only a published Event sells, and cancelling takes it off sale for
// good.
type Event struct {
	id     string
	title  string
	story  string
	when   string
	where  string
	status Status
	tiers  []TicketTier
}

// Domain refusals, named for their subject.
var (
	ErrIdentityMissing   = errors.New("event: an event needs an id and a title")
	ErrEventPublished    = errors.New("event: a published event cannot be edited")
	ErrEventCancelled    = errors.New("event: a cancelled event is off sale for good")
	ErrEventNotPublished = errors.New("event: only a published event can be cancelled")
	ErrNothingToSell     = errors.New("event: publishing needs at least one ticket tier")
	ErrTierUnpriced      = errors.New("event: every ticket tier needs a price before publishing")
	ErrTierNameMissing   = errors.New("event: a ticket tier needs a name")
	ErrTierDuplicate     = errors.New("event: a ticket tier with that name already exists")
	ErrPriceNegative     = errors.New("event: a price cannot be negative")
	ErrAlreadyPublished  = errors.New("event: already published")
	ErrAlreadyCancelled  = errors.New("event: already cancelled")
)

// New starts a draft Event.
func New(id, title string) (Event, error) {
	if id == "" || title == "" {
		return Event{}, ErrIdentityMissing
	}
	return Event{id: id, title: title, status: StatusDraft}, nil
}

// refuseUnlessDraft says why the Event can no longer be reshaped, or
// nil while it is still a draft. Editing ends at publication and
// never resumes.
func (e Event) refuseUnlessDraft() error {
	switch e.status {
	case StatusPublished:
		return ErrEventPublished
	case StatusCancelled:
		return ErrEventCancelled
	default:
		return nil
	}
}

// Tell sets the story and the when and where of the Event. Drafts
// only: a published Event is a promise already made.
func (e *Event) Tell(story, when, where string) error {
	if err := e.refuseUnlessDraft(); err != nil {
		return err
	}
	e.story, e.when, e.where = story, when, where
	return nil
}

// AddTier adds one TicketTier to a draft Event.
func (e *Event) AddTier(name string, price Price) error {
	if err := e.refuseUnlessDraft(); err != nil {
		return err
	}
	if name == "" {
		return ErrTierNameMissing
	}
	if price < 0 {
		return ErrPriceNegative
	}
	for _, t := range e.tiers {
		if t.Name == name {
			return ErrTierDuplicate
		}
	}
	tiers := make([]TicketTier, len(e.tiers), len(e.tiers)+1)
	copy(tiers, e.tiers)
	e.tiers = append(tiers, TicketTier{Name: name, Price: price})
	return nil
}

// ReplaceTiers sets a draft's TicketTiers wholesale, the way the
// organizer's editor saves the whole list: added, renamed, repriced,
// and removed tiers all land at once. Drafts only; every tier needs
// a name, names stay unique, and a Price is never negative.
func (e *Event) ReplaceTiers(tiers []TicketTier) error {
	if err := e.refuseUnlessDraft(); err != nil {
		return err
	}
	seen := make(map[string]bool, len(tiers))
	for _, t := range tiers {
		if t.Name == "" {
			return ErrTierNameMissing
		}
		if t.Price < 0 {
			return ErrPriceNegative
		}
		if seen[t.Name] {
			return ErrTierDuplicate
		}
		seen[t.Name] = true
	}
	next := make([]TicketTier, len(tiers))
	copy(next, tiers)
	e.tiers = next
	return nil
}

// Publish puts the Event on sale: it needs at least one TicketTier,
// and every TicketTier must carry a Price. A cancelled Event never
// goes back on sale.
func (e *Event) Publish() error {
	switch e.status {
	case StatusPublished:
		return ErrAlreadyPublished
	case StatusCancelled:
		return ErrEventCancelled
	}
	if len(e.tiers) == 0 {
		return ErrNothingToSell
	}
	for _, t := range e.tiers {
		if t.Price <= 0 {
			return ErrTierUnpriced
		}
	}
	e.status = StatusPublished
	return nil
}

// Cancel calls a published show off: the Event goes off sale for
// good. A draft has nothing to call off, and cancelling happens
// once. The tickets sold for the Event are given back by the use
// case that drives this; the aggregate decides only whether the
// Event may be cancelled at all.
func (e *Event) Cancel() error {
	switch e.status {
	case StatusCancelled:
		return ErrAlreadyCancelled
	case StatusDraft:
		return ErrEventNotPublished
	}
	e.status = StatusCancelled
	return nil
}

// ID names the Event.
func (e Event) ID() string { return e.id }

// Title is the Event's name on the page.
func (e Event) Title() string { return e.title }

// Story is the page's telling of the Event.
func (e Event) Story() string { return e.story }

// When says when the Event happens, as told on the page.
func (e Event) When() string { return e.when }

// Where says where the Event happens, as told on the page.
func (e Event) Where() string { return e.where }

// Status is where the Event stands. An Event that never went
// through New still reads as the draft it effectively is.
func (e Event) Status() Status {
	if e.status == "" {
		return StatusDraft
	}
	return e.status
}

// Draft reports whether the Event is still the Organizer's alone:
// unpublished, and so invisible to everyone else.
func (e Event) Draft() bool { return e.Status() == StatusDraft }

// OnSale reports whether the Event sells tickets right now. A draft
// has not started selling and a cancelled Event has stopped.
func (e Event) OnSale() bool { return e.status == StatusPublished }

// Cancelled reports whether the show was called off.
func (e Event) Cancelled() bool { return e.status == StatusCancelled }

// Tiers returns the TicketTiers as a copy; the aggregate's own list
// changes only through its methods.
func (e Event) Tiers() []TicketTier {
	out := make([]TicketTier, len(e.tiers))
	copy(out, e.tiers)
	return out
}

// Tier finds one TicketTier by name.
func (e Event) Tier(name string) (TicketTier, bool) {
	for _, t := range e.tiers {
		if t.Name == name {
			return t, true
		}
	}
	return TicketTier{}, false
}
