package rule

import (
	"fmt"
	"sort"
)

// FixtureFile is one materialized file of a deterministic Rule Test
// scenario.
type FixtureFile struct {
	Path    string
	Content string
}

// Finding is the neutral shape a Rule Test asserts against: one
// expected or produced Diagnostic identified by anchor path and
// message.
type Finding struct {
	Path    string
	Message string
}

// Test is a deterministic fixture scenario and the complete expected
// Diagnostics for its Rule. It asserts the complete result, never one
// convenient Diagnostic.
type Test struct {
	name     string
	fixture  []FixtureFile
	expected []Finding
}

// NewTest requires a name and at least one fixture file. An empty
// expected set is meaningful: the scenario must produce no
// Diagnostics.
func NewTest(name string, fixture []FixtureFile, expected []Finding) (Test, error) {
	if name == "" {
		return Test{}, fmt.Errorf("rule test: missing name")
	}
	if len(fixture) == 0 {
		return Test{}, fmt.Errorf("rule test %q: no fixture files", name)
	}
	seen := map[string]bool{}
	for _, f := range fixture {
		if f.Path == "" {
			return Test{}, fmt.Errorf("rule test %q: fixture file with empty path", name)
		}
		if seen[f.Path] {
			return Test{}, fmt.Errorf("rule test %q: duplicate fixture path %q", name, f.Path)
		}
		seen[f.Path] = true
	}
	return Test{
		name:     name,
		fixture:  append([]FixtureFile(nil), fixture...),
		expected: append([]Finding(nil), expected...),
	}, nil
}

// Name identifies the scenario.
func (t Test) Name() string { return t.name }

// Fixture returns the scenario files.
func (t Test) Fixture() []FixtureFile { return append([]FixtureFile(nil), t.fixture...) }

// Expected returns the complete expected Diagnostics.
func (t Test) Expected() []Finding { return append([]Finding(nil), t.expected...) }

// IsZero reports an unconstructed Test.
func (t Test) IsZero() bool { return t.name == "" }

// Compare detects missing, unexpected, and duplicate Diagnostics by
// multiset comparison of the complete result.
func (t Test) Compare(actual []Finding) (missing, unexpected []Finding) {
	key := func(f Finding) string { return f.Path + "\x00" + f.Message }
	counts := map[string]int{}
	byKey := map[string]Finding{}
	for _, f := range t.expected {
		counts[key(f)]++
		byKey[key(f)] = f
	}
	for _, f := range actual {
		k := key(f)
		if counts[k] > 0 {
			counts[k]--
			continue
		}
		unexpected = append(unexpected, f)
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for range counts[k] {
			missing = append(missing, byKey[k])
		}
	}
	return missing, unexpected
}
