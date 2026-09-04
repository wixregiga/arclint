package rule

import "fmt"

// Installation is the decision to extend one Pattern: the extends entry
// rules.yaml records, naming the PatternReference and carrying a
// Binding for every PatternModule that has paths here. A Module left
// unbound is listed so the adopter is told what still needs a path.
type Installation struct {
	ref      PatternReference
	modules  []PatternModule
	bindings map[ModuleName]Binding
}

// NewInstallation drafts the Installation of a Pattern from the paths
// it suggests: every Module with suggested paths is bound to exactly
// those paths, every other Module is reported unbound.
func NewInstallation(p Pattern) (Installation, error) {
	if p.Reference().IsZero() {
		return Installation{}, fmt.Errorf("installation: unconstructed pattern")
	}
	inst := Installation{ref: p.Reference(), modules: p.Modules(), bindings: map[ModuleName]Binding{}}
	for _, m := range inst.modules {
		paths := m.SuggestedPaths()
		if len(paths) == 0 {
			continue
		}
		b, err := NewBinding(m.Name(), paths)
		if err != nil {
			return Installation{}, fmt.Errorf("installation of %s: %v", p.Reference(), err)
		}
		inst.bindings[m.Name()] = b
	}
	return inst, nil
}

// Rebind binds one Pattern Module to the given paths, replacing any
// drafted Binding: the adopter's own paths for a Module win over the
// Pattern's suggestion.
func (i Installation) Rebind(name ModuleName, paths []Glob) (Installation, error) {
	if i.IsZero() {
		return Installation{}, fmt.Errorf("installation: unconstructed")
	}
	if _, ok := i.module(name); !ok {
		return Installation{}, fmt.Errorf("installation of %s: the pattern lists no module %q", i.ref, name)
	}
	b, err := NewBinding(name, paths)
	if err != nil {
		return Installation{}, fmt.Errorf("installation of %s: %v", i.ref, err)
	}
	out := Installation{ref: i.ref, modules: i.modules, bindings: make(map[ModuleName]Binding, len(i.bindings)+1)}
	for k, v := range i.bindings {
		out.bindings[k] = v
	}
	out.bindings[name] = b
	return out, nil
}

// Reference is the Pattern the Installation extends.
func (i Installation) Reference() PatternReference { return i.ref }

// Modules lists every Pattern Module in Pattern order, bound or not.
func (i Installation) Modules() []PatternModule {
	return append([]PatternModule(nil), i.modules...)
}

// Binding returns the Binding of one Pattern Module, if it has one.
func (i Installation) Binding(name ModuleName) (Binding, bool) {
	b, ok := i.bindings[name]
	return b, ok
}

// Bindings are the Module bindings, in Pattern Module order.
func (i Installation) Bindings() []Binding {
	out := make([]Binding, 0, len(i.bindings))
	for _, m := range i.modules {
		if b, ok := i.bindings[m.Name()]; ok {
			out = append(out, b)
		}
	}
	return out
}

// Unbound lists the Pattern Modules that still have no paths, in
// Pattern Module order.
func (i Installation) Unbound() []PatternModule {
	var out []PatternModule
	for _, m := range i.modules {
		if _, ok := i.bindings[m.Name()]; !ok {
			out = append(out, m)
		}
	}
	return out
}

// IsZero reports an unconstructed value.
func (i Installation) IsZero() bool { return i.ref.IsZero() }

func (i Installation) module(name ModuleName) (PatternModule, bool) {
	for _, m := range i.modules {
		if m.Name() == name {
			return m, true
		}
	}
	return PatternModule{}, false
}
