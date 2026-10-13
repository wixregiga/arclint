package vocab

import (
	"fmt"
	"reflect"
)

// Difference says where two recordings of a language part ways, or
// returns "" when they record the same language. Where a language is
// written down is no part of it: the DomainRecording and every Line are
// ignored, so a language translated from one DomainRecording into
// another and read back is the same language exactly when Difference
// returns "". Everything else counts, the order of every section
// included.
func (l UbiquitousLanguage) Difference(other UbiquitousLanguage) string {
	if l.Project != other.Project {
		return fmt.Sprintf("the project is %q in one and %q in the other", l.Project, other.Project)
	}
	if l.Description != other.Description {
		return "the description differs"
	}
	if len(l.Contexts) != len(other.Contexts) {
		return fmt.Sprintf("%d bounded contexts against %d", len(l.Contexts), len(other.Contexts))
	}
	for i := range l.Contexts {
		if d := contextDifference(l.Contexts[i], other.Contexts[i]); d != "" {
			return d
		}
	}
	if len(l.Relations) != len(other.Relations) {
		return fmt.Sprintf("%d relations against %d", len(l.Relations), len(other.Relations))
	}
	for i := range l.Relations {
		a, b := l.Relations[i], other.Relations[i]
		a.Line, b.Line = 0, 0
		if a != b {
			return fmt.Sprintf("relation %d differs: %s %s %s against %s %s %s",
				i+1, l.Relations[i].From, l.Relations[i].Kind, l.Relations[i].To,
				other.Relations[i].From, other.Relations[i].Kind, other.Relations[i].To)
		}
	}
	return ""
}

// contextDifference compares two bounded contexts section by section,
// naming the first entry that differs.
func contextDifference(a, b BoundedContext) string {
	if a.Name != b.Name {
		return fmt.Sprintf("bounded context %q stands where %q does", a.Name, b.Name)
	}
	where := fmt.Sprintf("bounded context %q", a.Name)
	if a.Definition != b.Definition {
		return where + ": the definition differs"
	}
	sections := []struct {
		name string
		a, b []namedEntry
	}{
		{"aggregates", entries(a.Aggregates, func(x Aggregate) string { return x.Name }), entries(b.Aggregates, func(x Aggregate) string { return x.Name })},
		{"value objects", entries(a.ValueObjects, func(x ValueObject) string { return x.Name }), entries(b.ValueObjects, func(x ValueObject) string { return x.Name })},
		{"events", entries(a.Events, func(x DomainEvent) string { return x.Name }), entries(b.Events, func(x DomainEvent) string { return x.Name })},
		{"services", entries(a.Services, func(x DomainService) string { return x.Name }), entries(b.Services, func(x DomainService) string { return x.Name })},
		{"specifications", entries(a.Specifications, func(x Specification) string { return x.Name }), entries(b.Specifications, func(x Specification) string { return x.Name })},
		{"questions", entries(a.Questions, func(x Question) string { return x.Key }), entries(b.Questions, func(x Question) string { return x.Key })},
	}
	for _, s := range sections {
		if d := sectionDifference(s.name, s.a, s.b); d != "" {
			return where + ": " + d
		}
	}
	return ""
}

// namedEntry is one recorded entry with its name and its value, every
// Line in it set to zero.
type namedEntry struct {
	name  string
	value any
}

func entries[T any](in []T, name func(T) string) []namedEntry {
	out := make([]namedEntry, 0, len(in))
	for _, x := range in {
		out = append(out, namedEntry{name: name(x), value: withoutLines(x)})
	}
	return out
}

func sectionDifference(section string, a, b []namedEntry) string {
	if len(a) != len(b) {
		return fmt.Sprintf("%d %s against %d", len(a), section, len(b))
	}
	for i := range a {
		if a[i].name != b[i].name {
			return fmt.Sprintf("%s: %q stands where %q does", section, a[i].name, b[i].name)
		}
		if !reflect.DeepEqual(a[i].value, b[i].value) {
			return fmt.Sprintf("%s: %q differs", section, a[i].name)
		}
	}
	return ""
}

// withoutLines returns a copy of one recorded entry with every Line it
// holds, its members' included, set to zero.
func withoutLines(v any) any {
	switch x := v.(type) {
	case Aggregate:
		x = x.clone()
		x.Line = 0
		x.Aliases = nilWhenEmpty(x.Aliases)
		x.Entities = nilWhenEmpty(x.Entities)
		for i := range x.Entities {
			x.Entities[i].Line = 0
			x.Entities[i].Aliases = nilWhenEmpty(x.Entities[i].Aliases)
		}
		x.Invariants = nilWhenEmpty(x.Invariants)
		x.Assertions = nilWhenEmpty(x.Assertions)
		zeroInvariantLines(x.Invariants)
		zeroAssertionLines(x.Assertions)
		return x
	case ValueObject:
		x = x.clone()
		x.Line = 0
		x.Aliases = nilWhenEmpty(x.Aliases)
		x.Invariants = nilWhenEmpty(x.Invariants)
		zeroInvariantLines(x.Invariants)
		return x
	case DomainEvent:
		x.Line = 0
		return x
	case DomainService:
		x = x.clone()
		x.Line = 0
		x.Assertions = nilWhenEmpty(x.Assertions)
		zeroAssertionLines(x.Assertions)
		return x
	case Specification:
		x.Line = 0
		return x
	case Question:
		x.Line = 0
		return x
	default:
		return v
	}
}

func zeroInvariantLines(in []Invariant) {
	for i := range in {
		in[i].Line = 0
	}
}

func zeroAssertionLines(in []Assertion) {
	for i := range in {
		in[i].Line = 0
	}
}

// nilWhenEmpty treats a section that records nothing alike whether it
// is held as nil or as an empty slice.
func nilWhenEmpty[T any](in []T) []T {
	if len(in) == 0 {
		return nil
	}
	return in
}
