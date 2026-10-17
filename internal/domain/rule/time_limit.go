package rule

import (
	"fmt"
	"time"
)

// TimeLimit is how long the check of one Extension Rule may run before
// it is stopped and the Rule is reported as not evaluated. Only an
// Extension Rule carries one; registration always runs under the
// default.
type TimeLimit struct {
	duration time.Duration
}

// DefaultTimeLimit applies when neither the Rule nor an Override sets
// one, and to every Extension's registration.
var DefaultTimeLimit = TimeLimit{duration: 5 * time.Second}

// NewTimeLimit accepts a positive duration.
func NewTimeLimit(d time.Duration) (TimeLimit, error) {
	if d <= 0 {
		return TimeLimit{}, fmt.Errorf("time limit %s: must be a positive duration", d)
	}
	return TimeLimit{duration: d}, nil
}

// ParseTimeLimit reads a duration such as 10s or 1m30s.
func ParseTimeLimit(s string) (TimeLimit, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return TimeLimit{}, fmt.Errorf("time limit %q: not a duration such as 10s or 1m30s", s)
	}
	return NewTimeLimit(d)
}

// Duration returns the limit.
func (t TimeLimit) Duration() time.Duration { return t.duration }

// IsZero reports an unset limit.
func (t TimeLimit) IsZero() bool { return t.duration == 0 }

// String spells the limit as a duration.
func (t TimeLimit) String() string { return t.duration.String() }

// Near reports whether a check that ran for elapsed came within half of
// this limit: a run twice as slow would stop.
func (t TimeLimit) Near(elapsed time.Duration) bool { return 2*elapsed > t.duration }
