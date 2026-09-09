package rule

import "fmt"

// Scope is the composable, inspectable selection of the Files,
// Folders, and Zones a Rule evaluates. Selector dimensions intersect;
// multiple values within one dimension form a union. Rule Exclusions
// remove only their selected subjects.
type Scope struct {
	entireRepository bool
	zones            []ZoneName
	files            []Glob
	exclusions       []Exclusion
}

// RepositoryScope selects the entire repository, optionally
// narrowed by file globs.
func RepositoryScope(files ...Glob) (Scope, error) {
	if err := validFileGlobs(files); err != nil {
		return Scope{}, err
	}
	return Scope{
		entireRepository: true,
		files:            append([]Glob(nil), files...),
	}, nil
}

// ZoneScope selects the members of the named Zones (union),
// optionally intersected with file globs.
func ZoneScope(zones []ZoneName, files ...Glob) (Scope, error) {
	if len(zones) == 0 {
		return Scope{}, fmt.Errorf("scope: no zone selected")
	}
	if err := uniqueValidZones("scope", zones); err != nil {
		return Scope{}, err
	}
	if err := validFileGlobs(files); err != nil {
		return Scope{}, err
	}
	return Scope{
		zones: append([]ZoneName(nil), zones...),
		files: append([]Glob(nil), files...),
	}, nil
}

func validFileGlobs(files []Glob) error {
	for _, g := range files {
		if g.IsZero() {
			return fmt.Errorf("scope: unconstructed file glob")
		}
	}
	return nil
}

// IsZero reports an unconstructed Scope, which selects nothing.
func (a Scope) IsZero() bool {
	return !a.entireRepository && len(a.zones) == 0
}

// EntireRepository reports repository-wide selection.
func (a Scope) EntireRepository() bool { return a.entireRepository }

// Zones returns the selected Zone names.
func (a Scope) Zones() []ZoneName {
	return append([]ZoneName(nil), a.zones...)
}

// Files returns the file-glob dimension.
func (a Scope) Files() []Glob { return append([]Glob(nil), a.files...) }

// Exclusions returns the applied Rule Exclusions.
func (a Scope) Exclusions() []Exclusion {
	return append([]Exclusion(nil), a.exclusions...)
}

// Excluding returns Scope with the Exclusion's subjects
// removed.
func (a Scope) Excluding(e Exclusion) Scope {
	a.exclusions = append(append([]Exclusion(nil), a.exclusions...), e)
	return a
}

// WouldSelectFile decides selection by the zone and file dimensions
// alone, ignoring Exclusions. memberOf is the file's resolved Zone
// membership.
func (a Scope) WouldSelectFile(path string, memberOf []ZoneName) bool {
	if a.IsZero() {
		return false
	}
	if !a.entireRepository {
		member := false
		for _, m := range a.zones {
			for _, of := range memberOf {
				if m == of {
					member = true
					break
				}
			}
		}
		if !member {
			return false
		}
	}
	if len(a.files) == 0 {
		return true
	}
	for _, g := range a.files {
		if g.Match(path) {
			return true
		}
	}
	return false
}

// ExcludedFile reports whether an Exclusion removes the path.
func (a Scope) ExcludedFile(path string) bool {
	for _, e := range a.exclusions {
		if e.ExcludesFile(path) {
			return true
		}
	}
	return false
}

// SelectsFile decides whether the file is a Rule Subject: selected by
// the dimensions and not excluded.
func (a Scope) SelectsFile(path string, memberOf []ZoneName) bool {
	return a.WouldSelectFile(path, memberOf) && !a.ExcludedFile(path)
}

// WouldSelectZone decides Zone selection ignoring Exclusions.
func (a Scope) WouldSelectZone(name ZoneName) bool {
	if a.entireRepository {
		return true
	}
	for _, m := range a.zones {
		if m == name {
			return true
		}
	}
	return false
}

// ExcludedZone reports whether an Exclusion removes the Zone.
func (a Scope) ExcludedZone(name ZoneName) bool {
	for _, e := range a.exclusions {
		if e.ExcludesZone(name) {
			return true
		}
	}
	return false
}

// SelectsZone decides whether the Zone is a Rule Subject.
func (a Scope) SelectsZone(name ZoneName) bool {
	return a.WouldSelectZone(name) && !a.ExcludedZone(name)
}
