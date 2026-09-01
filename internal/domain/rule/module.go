package rule

import (
	"errors"
	"fmt"
)

// ModuleName is the non-empty repository-local name of a Module.
type ModuleName string

// NewModuleName validates a Module name.
func NewModuleName(s string) (ModuleName, error) {
	m := ModuleName(s)
	if err := m.validate(); err != nil {
		return "", err
	}
	return m, nil
}

func (m ModuleName) validate() error {
	if m == "" {
		return errors.New("module name: empty")
	}
	for _, r := range string(m) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return fmt.Errorf("module name %q: contains %q (allowed: a-z 0-9 _ -)", string(m), r)
		}
	}
	return nil
}

func (m ModuleName) String() string { return string(m) }

// Module is a named logical grouping of files and directories selected by
// membership globs. Modules may overlap; membership does not imply exclusive
// ownership.
type Module struct {
	name        ModuleName
	description string
	paths       []Glob
}

// NewModule requires a valid name and at least one membership selector.
func NewModule(name ModuleName, description string, paths []Glob) (Module, error) {
	if err := name.validate(); err != nil {
		return Module{}, err
	}
	if len(paths) == 0 {
		return Module{}, fmt.Errorf("module %q: no membership selector", name)
	}
	for _, g := range paths {
		if g.IsZero() {
			return Module{}, fmt.Errorf("module %q: unconstructed membership glob", name)
		}
	}
	return Module{
		name: name, description: description,
		paths: append([]Glob(nil), paths...),
	}, nil
}

// Name returns the repository-local Module name.
func (m Module) Name() ModuleName { return m.name }

// Description returns the authoring description, possibly empty.
func (m Module) Description() string { return m.description }

// Paths returns the membership selectors.
func (m Module) Paths() []Glob { return append([]Glob(nil), m.paths...) }

// IsZero reports an unconstructed Module.
func (m Module) IsZero() bool { return m.name == "" }

// Contains determines membership of a repo-relative file path: a
// selector matches the path directly, or names a directory whose whole
// subtree belongs to the Module.
func (m Module) Contains(path string) bool {
	for _, g := range m.paths {
		if g.MatchesSubtree(path) {
			return true
		}
	}
	return false
}
