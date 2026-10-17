package conformance

import (
	"fmt"
	"strings"
	"time"
)

// ExtensionPhase names when Extension code ran: while registering its
// rule definitions, or while checking one Rule.
type ExtensionPhase string

const (
	// PhaseRegistration runs the Extension's module and collects its
	// rule definitions.
	PhaseRegistration ExtensionPhase = "registration"
	// PhaseCheck runs one Rule's check over its selected subjects.
	PhaseCheck ExtensionPhase = "check"
)

// ExtensionFailureSpec is the plain construction input of an
// ExtensionFailure.
type ExtensionFailureSpec struct {
	// Source names where the code came from: a file under
	// .arclint/extensions or a Pattern's extension.
	Source string
	Phase  ExtensionPhase
	// Limit is the time limit in force; Elapsed is how long the code
	// ran before it stopped.
	Limit   time.Duration
	Elapsed time.Duration
	// TimedOut reports that the code reached Limit; otherwise Raised is
	// the error it raised.
	TimedOut bool
	Raised   string
	// File is the subject the check last read, "" when none is known.
	File string
}

// ExtensionFailure is the reason an Extension Rule was not evaluated:
// its Extension ran past its time limit, or its code failed while
// running. An ExtensionEvaluator returns it as the error of Evaluate;
// the check then reports that one Rule as not evaluated and keeps
// evaluating every other Rule. Any other error from Evaluate is a
// configuration error that aborts the check.
type ExtensionFailure struct {
	origin   string
	phase    ExtensionPhase
	limit    time.Duration
	elapsed  time.Duration
	timedOut bool
	raised   string
	file     string
}

// NewExtensionFailure validates the circumstances of one stopped
// Extension run: it always states its cause, its phase, and how long
// the code ran against the limit in force.
func NewExtensionFailure(spec ExtensionFailureSpec) (ExtensionFailure, error) {
	if strings.TrimSpace(spec.Source) == "" {
		return ExtensionFailure{}, fmt.Errorf("extension failure: missing source")
	}
	switch spec.Phase {
	case PhaseRegistration, PhaseCheck:
	default:
		return ExtensionFailure{}, fmt.Errorf("extension failure: phase %q is neither registration nor check", spec.Phase)
	}
	if spec.Limit <= 0 {
		return ExtensionFailure{}, fmt.Errorf("extension failure: time limit %s is not positive", spec.Limit)
	}
	if spec.Elapsed < 0 {
		return ExtensionFailure{}, fmt.Errorf("extension failure: negative elapsed time %s", spec.Elapsed)
	}
	raised := strings.Join(strings.Fields(spec.Raised), " ")
	if spec.TimedOut == (raised != "") {
		return ExtensionFailure{}, fmt.Errorf("extension failure: needs exactly one cause, a timeout or a raised error")
	}
	if spec.File != "" && spec.Phase != PhaseCheck {
		return ExtensionFailure{}, fmt.Errorf("extension failure: only a check reads files")
	}
	return ExtensionFailure{
		origin: spec.Source, phase: spec.Phase, limit: spec.Limit, elapsed: spec.Elapsed,
		timedOut: spec.TimedOut, raised: raised, file: spec.File,
	}, nil
}

// Source names where the code that did not complete came from: a file
// under .arclint/extensions or a Pattern's extension. It is the Rule's
// own Extension, or another one whose registration failed and left no
// Extension able to run.
func (f ExtensionFailure) Source() string { return f.origin }

// Phase names when the code stopped.
func (f ExtensionFailure) Phase() ExtensionPhase { return f.phase }

// Limit is the time limit in force.
func (f ExtensionFailure) Limit() time.Duration { return f.limit }

// Elapsed is how long the code ran before it stopped.
func (f ExtensionFailure) Elapsed() time.Duration { return f.elapsed }

// TimedOut reports whether the code reached its time limit.
func (f ExtensionFailure) TimedOut() bool { return f.timedOut }

// Raised is the error the code raised, on one line; "" for a timeout.
func (f ExtensionFailure) Raised() string { return f.raised }

// File is the subject the check last read, "" when none is known.
func (f ExtensionFailure) File() string { return f.file }

// Cause states why the code stopped, in which phase, how long it ran,
// and the file it last read when one is known, on one line.
func (f ExtensionFailure) Cause() string {
	var b strings.Builder
	elapsed := f.elapsed.Round(time.Millisecond)
	if f.timedOut {
		fmt.Fprintf(&b, "reached its %s time limit in %s and was stopped after %s", f.limit, f.phase, elapsed)
	} else {
		fmt.Fprintf(&b, "failed in %s after %s of its %s time limit", f.phase, elapsed, f.limit)
	}
	if f.file != "" {
		fmt.Fprintf(&b, ", last reading %s", f.file)
	}
	if !f.timedOut {
		b.WriteString(": ")
		b.WriteString(f.raised)
	}
	return b.String()
}

// Error lets an ExtensionEvaluator return the failure through Evaluate.
func (f ExtensionFailure) Error() string {
	return fmt.Sprintf("extension %s %s", f.origin, f.Cause())
}
