// The JSON the box office speaks over HTTP. Transport shapes live
// here so entities stay technology-free.

package app

import (
	"boxoffice/internal/entities/capacity"
	"boxoffice/internal/entities/event"
	"boxoffice/internal/entities/order"
	"time"
)

type tierInput struct {
	Name       string `json:"name"`
	PriceCents int64  `json:"priceCents"`
	Seats      int    `json:"seats"`
}

type createEventInput struct {
	Title string      `json:"title"`
	Story string      `json:"story"`
	When  string      `json:"when"`
	Where string      `json:"where"`
	Tiers []tierInput `json:"tiers"`
}

type editEventInput struct {
	Story string      `json:"story"`
	When  string      `json:"when"`
	Where string      `json:"where"`
	Tiers []tierInput `json:"tiers"`
}

type tierView struct {
	Name       string `json:"name"`
	PriceCents int64  `json:"priceCents"`
	Remaining  int    `json:"remaining"`
}

type eventView struct {
	ID     string     `json:"id"`
	Title  string     `json:"title"`
	Story  string     `json:"story"`
	When   string     `json:"when"`
	Where  string     `json:"where"`
	Status string     `json:"status"`
	Tiers  []tierView `json:"tiers"`
}

// organizerTierView is a tier as its Organizer needs to see it: the
// seats published alongside what has become of them.
type organizerTierView struct {
	Name       string `json:"name"`
	PriceCents int64  `json:"priceCents"`
	Seats      int    `json:"seats"`
	SpokenFor  int    `json:"spokenFor"`
	Held       int    `json:"held"`
	Remaining  int    `json:"remaining"`
}

// organizerEventView is one Event from behind the counter: the page
// as it stands, the seat counts per tier, and who bought tickets.
type organizerEventView struct {
	ID     string              `json:"id"`
	Title  string              `json:"title"`
	Story  string              `json:"story"`
	When   string              `json:"when"`
	Where  string              `json:"where"`
	Status string              `json:"status"`
	Tiers  []organizerTierView `json:"tiers"`
	Orders []orderView         `json:"orders"`
}

type holdInput struct {
	Tier  string `json:"tier"`
	Seats int    `json:"seats"`
}

type holdView struct {
	HoldID   string    `json:"holdId"`
	Tier     string    `json:"tier"`
	Seats    int       `json:"seats"`
	Deadline time.Time `json:"deadline"`
}

type attendeeShape struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type placeOrderInput struct {
	EventID  string        `json:"eventId"`
	HoldIDs  []string      `json:"holdIds"`
	Attendee attendeeShape `json:"attendee"`
}

type refundInput struct {
	Tier     string `json:"tier"`
	Quantity int    `json:"quantity"`
}

// compTicketInput is how many tickets of one tier the Organizer is
// giving away. No price is asked for: a comped ticket is free.
type compTicketInput struct {
	Tier     string `json:"tier"`
	Quantity int    `json:"quantity"`
}

type compInput struct {
	Attendee attendeeShape     `json:"attendee"`
	Tickets  []compTicketInput `json:"tickets"`
}

// lineView is one line of the deal as struck, with the tickets of it
// that have since come back. Quantity is what was bought and never
// moves; Refunded is what was given back.
type lineView struct {
	Tier      string `json:"tier"`
	Quantity  int    `json:"quantity"`
	Refunded  int    `json:"refunded"`
	UnitCents int64  `json:"unitCents"`
}

// orderView is one Order as JSON. Comped says the Organizer gave
// this one away instead of an Attendee buying it, which is the
// organizer's answer to "what tickets did I comp?".
type orderView struct {
	ID               string        `json:"id"`
	EventID          string        `json:"eventId"`
	Attendee         attendeeShape `json:"attendee"`
	Lines            []lineView    `json:"lines"`
	TotalCents       int64         `json:"totalCents"`
	OutstandingCents int64         `json:"outstandingCents"`
	Comped           bool          `json:"comped"`
}

// viewEvent renders one Event with the seats remaining per tier.
func (s *server) viewEvent(ev event.Event, now time.Time) eventView {
	view := eventView{
		ID:     ev.ID(),
		Title:  ev.Title(),
		Story:  ev.Story(),
		When:   ev.When(),
		Where:  ev.Where(),
		Status: string(ev.Status()),
		Tiers:  []tierView{},
	}
	counts := s.tierCounts(ev.ID(), now)
	for _, t := range ev.Tiers() {
		view.Tiers = append(view.Tiers, tierView{
			Name:       t.Name,
			PriceCents: int64(t.Price),
			Remaining:  counts(t.Name).Remaining,
		})
	}
	return view
}

// viewOrganizerEvent renders one Event from behind the counter: the
// seats published per tier next to what has become of them, and the
// Orders placed for the Event.
func (s *server) viewOrganizerEvent(ev event.Event, orders []order.Order, now time.Time) organizerEventView {
	view := organizerEventView{
		ID:     ev.ID(),
		Title:  ev.Title(),
		Story:  ev.Story(),
		When:   ev.When(),
		Where:  ev.Where(),
		Status: string(ev.Status()),
		Tiers:  []organizerTierView{},
		Orders: []orderView{},
	}
	counts := s.tierCounts(ev.ID(), now)
	for _, t := range ev.Tiers() {
		count := counts(t.Name)
		view.Tiers = append(view.Tiers, organizerTierView{
			Name:       t.Name,
			PriceCents: int64(t.Price),
			Seats:      count.Seats,
			SpokenFor:  count.SpokenFor,
			Held:       count.Held,
			Remaining:  count.Remaining,
		})
	}
	for _, o := range orders {
		view.Orders = append(view.Orders, viewOrder(o))
	}
	return view
}

// tierCounts asks the ledger about one Event once and then answers
// per tier. A tier the ledger has never heard of counts as nothing
// rather than failing the page: the catalog and the ledger are
// separate contexts, and the page tells what it knows.
func (s *server) tierCounts(eventID string, now time.Time) func(tier string) capacity.Count {
	c, err := s.deps.Capacities.Find(eventID)
	return func(tier string) capacity.Count {
		if err != nil {
			return capacity.Count{}
		}
		count, cerr := c.Count(tier, now)
		if cerr != nil {
			return capacity.Count{}
		}
		return count
	}
}

func viewOrder(o order.Order) orderView {
	view := orderView{
		ID:               o.ID(),
		EventID:          o.EventID(),
		Attendee:         attendeeShape{Name: o.Attendee().Name, Email: o.Attendee().Email},
		Lines:            []lineView{},
		TotalCents:       o.TotalCents(),
		OutstandingCents: o.OutstandingCents(),
		Comped:           o.Comped(),
	}
	for _, l := range o.Lines() {
		view.Lines = append(view.Lines, lineView{
			Tier:      l.TierName,
			Quantity:  l.Quantity,
			Refunded:  o.Refunded(l.TierName),
			UnitCents: l.UnitCents,
		})
	}
	return view
}
