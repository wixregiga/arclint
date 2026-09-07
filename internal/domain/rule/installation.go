package rule

import "fmt"

// Installation is the decision to extend one Pattern: the extends entry
// rules.arclint.yaml records, naming the PatternReference and carrying a
// Binding for every PatternZone that has paths here. A Zone left
// unbound is listed so the adopter is told what still needs a path.
type Installation struct {
	ref      PatternReference
	zones    []PatternZone
	bindings map[ZoneName]Binding
}

// NewInstallation drafts the Installation of a Pattern from the paths
// it suggests: every Zone with suggested paths is bound to exactly
// those paths, every other Zone is reported unbound.
func NewInstallation(p Pattern) (Installation, error) {
	if p.Reference().IsZero() {
		return Installation{}, fmt.Errorf("installation: unconstructed pattern")
	}
	inst := Installation{ref: p.Reference(), zones: p.Zones(), bindings: map[ZoneName]Binding{}}
	for _, m := range inst.zones {
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

// Rebind binds one Pattern Zone to the given paths, replacing any
// drafted Binding: the adopter's own paths for a Zone win over the
// Pattern's suggestion.
func (i Installation) Rebind(name ZoneName, paths []Glob) (Installation, error) {
	if i.IsZero() {
		return Installation{}, fmt.Errorf("installation: unconstructed")
	}
	if _, ok := i.zone(name); !ok {
		return Installation{}, fmt.Errorf("installation of %s: the pattern lists no zone %q", i.ref, name)
	}
	b, err := NewBinding(name, paths)
	if err != nil {
		return Installation{}, fmt.Errorf("installation of %s: %v", i.ref, err)
	}
	out := Installation{ref: i.ref, zones: i.zones, bindings: make(map[ZoneName]Binding, len(i.bindings)+1)}
	for k, v := range i.bindings {
		out.bindings[k] = v
	}
	out.bindings[name] = b
	return out, nil
}

// Reference is the Pattern the Installation extends.
func (i Installation) Reference() PatternReference { return i.ref }

// Zones lists every Pattern Zone in Pattern order, bound or not.
func (i Installation) Zones() []PatternZone {
	return append([]PatternZone(nil), i.zones...)
}

// Binding returns the Binding of one Pattern Zone, if it has one.
func (i Installation) Binding(name ZoneName) (Binding, bool) {
	b, ok := i.bindings[name]
	return b, ok
}

// Bindings are the Zone bindings, in Pattern Zone order.
func (i Installation) Bindings() []Binding {
	out := make([]Binding, 0, len(i.bindings))
	for _, m := range i.zones {
		if b, ok := i.bindings[m.Name()]; ok {
			out = append(out, b)
		}
	}
	return out
}

// Unbound lists the Pattern Zones that still have no paths, in
// Pattern Zone order.
func (i Installation) Unbound() []PatternZone {
	var out []PatternZone
	for _, m := range i.zones {
		if _, ok := i.bindings[m.Name()]; !ok {
			out = append(out, m)
		}
	}
	return out
}

// IsZero reports an unconstructed value.
func (i Installation) IsZero() bool { return i.ref.IsZero() }

func (i Installation) zone(name ZoneName) (PatternZone, bool) {
	for _, m := range i.zones {
		if m.Name() == name {
			return m, true
		}
	}
	return PatternZone{}, false
}
