// Package capacity holds the capacity aggregate: the seat count for
// one Event, seats spoken for, and the Holds still pending. It has
// no opinion on stories or Prices, only on whether one more promise
// fits the room.
package capacity

import (
	"errors"
	"time"
)

// Hold is seats set aside for someone still deciding, before any
// Order exists. It carries a deadline from birth; an expired Hold
// frees its seats.
type Hold struct {
	ID       string
	TierName string
	Seats    int
	Deadline time.Time
}

// NewHold constructs a Hold.
func NewHold(id, tier string, seats int, deadline time.Time) Hold {
	return Hold{ID: id, TierName: tier, Seats: seats, Deadline: deadline}
}

type tierSeats struct {
	total     int
	spokenFor int
}

// Capacity is the capacity aggregate for one Event. Expiry is lazy:
// no timers, deadlines are checked against the caller's clock
// whenever the count is consulted.
type Capacity struct {
	eventID string
	tiers   map[string]*tierSeats
	holds   map[string]Hold
}

// Domain refusals, named for their subject.
var (
	ErrEventMissing         = errors.New("capacity: a capacity needs its event")
	ErrTierNotOpen          = errors.New("capacity: that ticket tier is not open")
	ErrTierOpenTwice        = errors.New("capacity: that ticket tier is already open")
	ErrSeatsInvalid         = errors.New("capacity: seats must be a positive count")
	ErrSeatsExhausted       = errors.New("capacity: not enough seats left")
	ErrHoldIDMissing        = errors.New("capacity: a hold needs an id")
	ErrHoldDuplicate        = errors.New("capacity: a hold with that id already exists")
	ErrHoldExpiredOrUnknown = errors.New("capacity: hold expired or unknown")
	ErrDeadlinePassed       = errors.New("capacity: a hold's deadline must lie ahead")
	ErrSeatsNotSpokenFor    = errors.New("capacity: that many seats are not spoken for")
)

// Count is the ledger's answer about one TicketTier: the seats the
// tier was given when it opened, how many are spoken for, how many
// are held by someone still deciding, and how many are left.
type Count struct {
	Seats     int
	SpokenFor int
	Held      int
	Remaining int
}

// New opens the count for one Event.
func New(eventID string) (Capacity, error) {
	if eventID == "" {
		return Capacity{}, ErrEventMissing
	}
	c := Capacity{
		eventID: eventID,
		tiers:   map[string]*tierSeats{},
		holds:   map[string]Hold{},
	}
	if err := c.SeatBudget(); err != nil {
		return Capacity{}, err
	}
	return c, nil
}

// SeatBudget is the cluster contract: seats spoken for plus seats held
// never exceed the seats a TicketTier has.
func (c Capacity) SeatBudget() error {
	for name, ts := range c.tiers {
		if ts.spokenFor+c.held(name) > ts.total {
			return ErrSeatsExhausted
		}
	}
	return nil
}

// EventID names the Event this count belongs to.
func (c Capacity) EventID() string { return c.eventID }

// OpenTier counts a TicketTier's seats in.
func (c Capacity) OpenTier(name string, seats int) error {
	if name == "" {
		return ErrTierNotOpen
	}
	if seats <= 0 {
		return ErrSeatsInvalid
	}
	if _, ok := c.tiers[name]; ok {
		return ErrTierOpenTwice
	}
	c.tiers[name] = &tierSeats{total: seats}
	return c.SeatBudget()
}

// prune drops expired Holds, freeing their seats.
func (c Capacity) prune(now time.Time) {
	for id, h := range c.holds {
		if !h.Deadline.After(now) {
			delete(c.holds, id)
		}
	}
}

func (c Capacity) held(tier string) int {
	sum := 0
	for _, h := range c.holds {
		if h.TierName == tier {
			sum += h.Seats
		}
	}
	return sum
}

// Count answers for one tier at once: the seats it was given, the
// seats spoken for, the seats held right now, and the seats left.
// Expired Holds are gone before the count is taken.
func (c Capacity) Count(tier string, now time.Time) (Count, error) {
	c.prune(now)
	ts, ok := c.tiers[tier]
	if !ok {
		return Count{}, ErrTierNotOpen
	}
	if err := c.SeatBudget(); err != nil {
		return Count{}, err
	}
	held := c.held(tier)
	return Count{
		Seats:     ts.total,
		SpokenFor: ts.spokenFor,
		Held:      held,
		Remaining: ts.total - ts.spokenFor - held,
	}, nil
}

// Remaining is the seats neither spoken for nor held on one tier.
func (c Capacity) Remaining(tier string, now time.Time) (int, error) {
	if err := c.SeatBudget(); err != nil {
		return 0, err
	}
	count, err := c.Count(tier, now)
	if err != nil {
		return 0, err
	}
	return count.Remaining, nil
}

// Refund hands seats back to the room: they stop being spoken for
// and are free to be promised again. A tier never gives back more
// seats than it has spoken for, so the count can never run past the
// seats the tier was given.
func (c Capacity) Refund(tier string, seats int) error {
	if seats <= 0 {
		return ErrSeatsInvalid
	}
	ts, ok := c.tiers[tier]
	if !ok {
		return ErrTierNotOpen
	}
	if seats > ts.spokenFor {
		return ErrSeatsNotSpokenFor
	}
	ts.spokenFor -= seats
	return c.SeatBudget()
}

// ReleaseHolds lets every pending Hold go at once. Cancelling an
// Event calls it: nobody is still deciding about a show that is off.
func (c Capacity) ReleaseHolds() {
	for id := range c.holds {
		delete(c.holds, id)
	}
}

// PlaceHold sets seats aside until the deadline. Seats spoken for
// plus seats held never exceed the seats a tier has; a hold that
// would break that is refused.
func (c Capacity) PlaceHold(id, tier string, seats int, deadline, now time.Time) error {
	c.prune(now)
	if id == "" {
		return ErrHoldIDMissing
	}
	if seats <= 0 {
		return ErrSeatsInvalid
	}
	if !deadline.After(now) {
		return ErrDeadlinePassed
	}
	ts, ok := c.tiers[tier]
	if !ok {
		return ErrTierNotOpen
	}
	if _, ok := c.holds[id]; ok {
		return ErrHoldDuplicate
	}
	if ts.total-ts.spokenFor-c.held(tier) < seats {
		return ErrSeatsExhausted
	}
	c.holds[id] = Hold{ID: id, TierName: tier, Seats: seats, Deadline: deadline}
	return c.SeatBudget()
}

// CommitAll turns the held seats of every named Hold into seats
// spoken for, all or nothing: one expired, unknown, or repeated hold
// refuses the whole commit and changes nothing.
func (c Capacity) CommitAll(ids []string, now time.Time) ([]Hold, error) {
	c.prune(now)
	if len(ids) == 0 {
		return nil, ErrHoldExpiredOrUnknown
	}
	committed := make([]Hold, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			return nil, ErrHoldDuplicate
		}
		seen[id] = true
		h, ok := c.holds[id]
		if !ok {
			return nil, ErrHoldExpiredOrUnknown
		}
		committed = append(committed, h)
	}
	for _, h := range committed {
		c.tiers[h.TierName].spokenFor += h.Seats
		delete(c.holds, h.ID)
	}
	if err := c.SeatBudget(); err != nil {
		return nil, err
	}
	return committed, nil
}

// Commit speaks seats for right away, without anyone having decided
// first. Comping takes this door: the Organizer is not deciding, so
// there is no Hold to make and none to commit. The room is counted
// the same either way, so seats that do not fit are refused. Expired
// Holds are gone before the room is measured.
func (c Capacity) Commit(tier string, seats int, now time.Time) error {
	c.prune(now)
	if seats <= 0 {
		return ErrSeatsInvalid
	}
	ts, ok := c.tiers[tier]
	if !ok {
		return ErrTierNotOpen
	}
	if ts.total-ts.spokenFor-c.held(tier) < seats {
		return ErrSeatsExhausted
	}
	ts.spokenFor += seats
	return c.SeatBudget()
}

// Release lets one Hold go early.
func (c Capacity) Release(id string) {
	delete(c.holds, id)
}

// Clone deep-copies the count; repositories store and hand out
// clones so no two callers share the same maps.
func (c Capacity) Clone() Capacity {
	out := Capacity{
		eventID: c.eventID,
		tiers:   make(map[string]*tierSeats, len(c.tiers)),
		holds:   make(map[string]Hold, len(c.holds)),
	}
	for name, ts := range c.tiers {
		cp := *ts
		out.tiers[name] = &cp
	}
	for id, h := range c.holds {
		out.holds[id] = h
	}
	return out
}
