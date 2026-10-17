package conformance

import (
	"fmt"
	"strings"
	"time"
)

// ExtensionFailure is the reason an Extension Rule was not evaluated:
// its Extension ran past its time limit, or its code failed while
// running. An ExtensionEvaluator returns it as the error of Evaluate;
// the check then reports that one Rule as not evaluated and keeps
// evaluating every other Rule. Any other error from Evaluate is a
// configuration error that aborts the check.
type ExtensionFailure struct {
	origin string
	limit  time.Duration
	cause  string
}

// NewExtensionTimeout reports an Extension from source that ran past
// its time limit.
func NewExtensionTimeout(source string, limit time.Duration) (ExtensionFailure, error) {
	if strings.TrimSpace(source) == "" {
		return ExtensionFailure{}, fmt.Errorf("extension failure: missing source")
	}
	if limit <= 0 {
		return ExtensionFailure{}, fmt.Errorf("extension failure: time limit %s is not positive", limit)
	}
	return ExtensionFailure{origin: source, limit: limit}, nil
}

// NewExtensionFault reports an Extension from source whose code raised
// cause while running.
func NewExtensionFault(source, cause string) (ExtensionFailure, error) {
	if strings.TrimSpace(source) == "" {
		return ExtensionFailure{}, fmt.Errorf("extension failure: missing source")
	}
	if strings.TrimSpace(cause) == "" {
		return ExtensionFailure{}, fmt.Errorf("extension failure: missing cause")
	}
	return ExtensionFailure{origin: source, cause: cause}, nil
}

// Source names where the code that did not complete came from: a file
// under .arclint/extensions or a Pattern's extension. It is the Rule's
// own Extension, or another one whose registration failed and left no
// Extension able to run.
func (f ExtensionFailure) Source() string { return f.origin }

// TimeLimit returns the limit the Extension exceeded, and whether it
// timed out at all.
func (f ExtensionFailure) TimeLimit() (time.Duration, bool) { return f.limit, f.limit > 0 }

// Cause states why the Extension did not complete.
func (f ExtensionFailure) Cause() string {
	if f.limit > 0 {
		return fmt.Sprintf("exceeded its %s time limit", f.limit)
	}
	return "failed: " + f.cause
}

// Error lets an ExtensionEvaluator return the failure through Evaluate.
func (f ExtensionFailure) Error() string {
	return fmt.Sprintf("extension %s %s", f.origin, f.Cause())
}
