package rule

import (
	"errors"
	"fmt"
)

// ZoneName is the non-empty repository-local name of a Zone.
type ZoneName string

// NewZoneName validates a Zone name.
func NewZoneName(s string) (ZoneName, error) {
	m := ZoneName(s)
	if err := m.validate(); err != nil {
		return "", err
	}
	return m, nil
}

func (m ZoneName) validate() error {
	if m == "" {
		return errors.New("zone name: empty")
	}
	for _, r := range string(m) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return fmt.Errorf("zone name %q: contains %q (allowed: a-z 0-9 _ -)", string(m), r)
		}
	}
	return nil
}

func (m ZoneName) String() string { return string(m) }

// Zone is a named logical grouping of Files and Folders, defined by
// the Pattern Consumer through membership globs. Zones may overlap;
// membership does not imply exclusive ownership.
type Zone struct {
	name        ZoneName
	description string
	paths       []Glob
}

// NewZone requires a valid name and at least one membership selector.
func NewZone(name ZoneName, description string, paths []Glob) (Zone, error) {
	if err := name.validate(); err != nil {
		return Zone{}, err
	}
	if len(paths) == 0 {
		return Zone{}, fmt.Errorf("zone %q: no membership selector", name)
	}
	for _, g := range paths {
		if g.IsZero() {
			return Zone{}, fmt.Errorf("zone %q: unconstructed membership glob", name)
		}
	}
	return Zone{
		name: name, description: description,
		paths: append([]Glob(nil), paths...),
	}, nil
}

// Name returns the repository-local Zone name.
func (m Zone) Name() ZoneName { return m.name }

// Description returns the authoring description, possibly empty.
func (m Zone) Description() string { return m.description }

// Paths returns the membership selectors.
func (m Zone) Paths() []Glob { return append([]Glob(nil), m.paths...) }

// Contains determines membership of a repo-relative file path: a
// selector matches the path directly, or names a directory whose whole
// subtree belongs to the Zone.
func (m Zone) Contains(path string) bool {
	for _, g := range m.paths {
		if g.MatchesSubtree(path) {
			return true
		}
	}
	return false
}
