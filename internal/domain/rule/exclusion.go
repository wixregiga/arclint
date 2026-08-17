package rule

import "fmt"

// Exclusion is a Pattern Consumer decision removing selected Files,
// Folders, or Modules from one Rule's Applicability. It applies to
// exactly one Rule (through attachment to the aggregate) and produces
// not-applicable rather than a Violation.
type Exclusion struct {
	paths   []Glob
	modules []ModuleName
	reason  string
}

// NewExclusion requires at least one concrete subject selector and a
// reason: an exclusion is policy, not configuration.
func NewExclusion(paths []Glob, modules []ModuleName, reason string) (Exclusion, error) {
	if len(paths)+len(modules) == 0 {
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
	if err := uniqueValidModules("exclusion", modules); err != nil {
		return Exclusion{}, err
	}
	return Exclusion{
		paths:   append([]Glob(nil), paths...),
		modules: append([]ModuleName(nil), modules...),
		reason:  reason,
	}, nil
}

// ExcludesFile decides whether a candidate path is outside Rule
// Applicability.
func (e Exclusion) ExcludesFile(path string) bool {
	for _, g := range e.paths {
		if g.Match(path) {
			return true
		}
	}
	return false
}

// ExcludesModule decides whether a candidate Module is outside Rule
// Applicability.
func (e Exclusion) ExcludesModule(name ModuleName) bool {
	for _, m := range e.modules {
		if m == name {
			return true
		}
	}
	return false
}

// Paths returns the path selectors.
func (e Exclusion) Paths() []Glob { return append([]Glob(nil), e.paths...) }

// Modules returns the Module selectors.
func (e Exclusion) Modules() []ModuleName { return append([]ModuleName(nil), e.modules...) }

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
