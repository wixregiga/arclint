package rule

import "fmt"

// Exclusion is a Pattern Consumer decision removing selected Files,
// Folders, or Zones from one Rule's Scope. It applies to
// exactly one Rule (through attachment to the aggregate) and produces
// not-applicable rather than a Violation.
type Exclusion struct {
	paths  []Glob
	zones  []ZoneName
	reason string
}

// NewExclusion requires at least one concrete subject selector and a
// reason: an exclusion is policy, not configuration.
func NewExclusion(paths []Glob, zones []ZoneName, reason string) (Exclusion, error) {
	if len(paths)+len(zones) == 0 {
		return Exclusion{}, fmt.Errorf("exclusion: no subject selector")
	}
	if reason == "" {
		return Exclusion{}, fmt.Errorf("exclusion: missing reason")
	}
	for _, g := range paths {
		if g.IsZero() {
			return Exclusion{}, fmt.Errorf("exclusion: unconstructed path glob")
		}
	}
	if err := uniqueValidZones("exclusion", zones); err != nil {
		return Exclusion{}, err
	}
	return Exclusion{
		paths:  append([]Glob(nil), paths...),
		zones:  append([]ZoneName(nil), zones...),
		reason: reason,
	}, nil
}

// ExcludesFile decides whether a candidate path is outside Rule
// Scope.
func (e Exclusion) ExcludesFile(path string) bool {
	for _, g := range e.paths {
		if g.Match(path) {
			return true
		}
	}
	return false
}

// ExcludesZone decides whether a candidate Zone is outside Rule
// Scope.
func (e Exclusion) ExcludesZone(name ZoneName) bool {
	for _, m := range e.zones {
		if m == name {
			return true
		}
	}
	return false
}

// Paths returns the path selectors.
func (e Exclusion) Paths() []Glob { return append([]Glob(nil), e.paths...) }

// Zones returns the Zone selectors.
func (e Exclusion) Zones() []ZoneName { return append([]ZoneName(nil), e.zones...) }

// Reason returns why the subjects were excluded.
func (e Exclusion) Reason() string { return e.reason }

// Disablement is a Pattern Consumer decision preventing one Rule from
// being evaluated for the repository. The Rule and its provenance stay
// inspectable.
type Disablement struct {
	reason string
}

// NewDisablement requires a reason so the decision stays inspectable.
func NewDisablement(reason string) (Disablement, error) {
	if reason == "" {
		return Disablement{}, fmt.Errorf("disablement: missing reason")
	}
	return Disablement{reason: reason}, nil
}

// Reason returns why the Rule is not evaluated.
func (d Disablement) Reason() string { return d.reason }
