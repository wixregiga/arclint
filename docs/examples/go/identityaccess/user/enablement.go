package user

import (
	"errors"
	"time"
)

// Enablement says whether a user may sign in, and optionally during which
// period.
type Enablement struct {
	enabled   bool
	startDate time.Time
	endDate   time.Time
}

// ErrEnablementWindowHalfOpen is returned when only one of start and end is set.
var ErrEnablementWindowHalfOpen = errors.New("enablement window needs both a start and an end")

// ErrEnablementWindowInverted is returned when the window ends before it starts.
var ErrEnablementWindowInverted = errors.New("enablement window must not end before it starts")

// IndefiniteEnablement enables a user with no period.
func IndefiniteEnablement() Enablement {
	return Enablement{enabled: true}
}

// NewEnablement is the one door through which a flag and a period become
// an Enablement. Invariant window-is-whole: a period has both ends, and
// the end is not before the start.
func NewEnablement(enabled bool, startDate, endDate time.Time) (Enablement, error) {
	if startDate.IsZero() != endDate.IsZero() {
		return Enablement{}, ErrEnablementWindowHalfOpen
	}
	if !startDate.IsZero() && endDate.Before(startDate) {
		return Enablement{}, ErrEnablementWindowInverted
	}
	return Enablement{enabled: enabled, startDate: startDate, endDate: endDate}, nil
}

// IsEnabled reports whether the flag is on, regardless of the period.
func (e Enablement) IsEnabled() bool { return e.enabled }

// IsEnablementEnabled is a side-effect-free function: the flag is on and,
// when a period is set, the instant lies inside it.
func (e Enablement) IsEnablementEnabled(at time.Time) bool {
	if !e.enabled {
		return false
	}
	if e.startDate.IsZero() {
		return true
	}
	return !at.Before(e.startDate) && !at.After(e.endDate)
}
