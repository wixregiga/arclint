package conformance

import (
	"fmt"
	"strings"
	"time"
)

// NearLimitSpec is the input to NewNearLimit: the Extension that ran a
// finished check, its source file, the TimeLimit in force, and how long
// the check ran.
type NearLimitSpec struct {
	Extension string
	Source    string
	Limit     time.Duration
	Elapsed   time.Duration
}

// NearLimit is an Extension Rule's check that finished after more than
// half its TimeLimit, so a run twice as slow would stop it. Two
// NearLimits with the same values are the same note.
type NearLimit struct {
	extension string
	origin    string
	limit     time.Duration
	elapsed   time.Duration
}

// NewNearLimit validates one check near its limit: it ran longer than
// half its TimeLimit, and it names the Extension and its source beside
// the TimeLimit in force and how long the check ran.
func NewNearLimit(spec NearLimitSpec) (NearLimit, error) {
	if strings.TrimSpace(spec.Extension) == "" || strings.TrimSpace(spec.Source) == "" {
		return NearLimit{}, fmt.Errorf("near limit: missing extension or source")
	}
	if spec.Limit <= 0 {
		return NearLimit{}, fmt.Errorf("near limit: time limit %s is not positive", spec.Limit)
	}
	if 2*spec.Elapsed <= spec.Limit {
		return NearLimit{}, fmt.Errorf("near limit: %s is not past half of %s", spec.Elapsed, spec.Limit)
	}
	return NearLimit{extension: spec.Extension, origin: spec.Source, limit: spec.Limit, elapsed: spec.Elapsed}, nil
}

// Extension names the Extension that ran the check.
func (n NearLimit) Extension() string { return n.extension }

// Source names the Extension's source file.
func (n NearLimit) Source() string { return n.origin }

// Limit is the TimeLimit in force.
func (n NearLimit) Limit() time.Duration { return n.limit }

// Elapsed is how long the check ran.
func (n NearLimit) Elapsed() time.Duration { return n.elapsed }
