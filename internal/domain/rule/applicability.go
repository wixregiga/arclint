package rule

import "fmt"

// Applicability is the composable, inspectable selection of the Files,
// Folders, and Zones a Rule evaluates. Selector dimensions intersect;
// multiple values within one dimension form a union. Rule Exclusions
// remove only their selected subjects.
type Applicability struct {
	entireRepository bool
	zones            []ZoneName
	files            []Glob
	exclusions       []Exclusion
}

// RepositoryApplicability selects the entire repository, optionally
// narrowed by file globs.
func RepositoryApplicability(files ...Glob) (Applicability, error) {
	if err := validFileGlobs(files); err != nil {
		return Applicability{}, err
	}
	return Applicability{
		entireRepository: true,
		files:            append([]Glob(nil), files...),
	}, nil
}

// ZoneApplicability selects the members of the named Zones (union),
// optionally intersected with file globs.
func ZoneApplicability(zones []ZoneName, files ...Glob) (Applicability, error) {
	if len(zones) == 0 {
		return Applicability{}, fmt.Errorf("applicability: no zone selected")
	}
	if err := uniqueValidZones("applicability", zones); err != nil {
		return Applicability{}, err
	}
	if err := validFileGlobs(files); err != nil {
		return Applicability{}, err
	}
	return Applicability{
		zones: append([]ZoneName(nil), zones...),
		files: append([]Glob(nil), files...),
	}, nil
}

func validFileGlobs(files []Glob) error {
	for _, g := range files {
		if g.IsZero() {
			return fmt.Errorf("applicability: unconstructed file glob")
		}
	}
	return nil
}

// IsZero reports an unconstructed Applicability, which selects nothing.
func (a Applicability) IsZero() bool {
	return !a.entireRepository && len(a.zones) == 0
}

// EntireRepository reports repository-wide selection.
func (a Applicability) EntireRepository() bool { return a.entireRepository }

// Zones returns the selected Zone names.
func (a Applicability) Zones() []ZoneName {
	return append([]ZoneName(nil), a.zones...)
}

// Files returns the file-glob dimension.
func (a Applicability) Files() []Glob { return append([]Glob(nil), a.files...) }

// Exclusions returns the applied Rule Exclusions.
func (a Applicability) Exclusions() []Exclusion {
	return append([]Exclusion(nil), a.exclusions...)
}

// Excluding returns Applicability with the Exclusion's subjects
// removed.
func (a Applicability) Excluding(e Exclusion) Applicability {
	a.exclusions = append(append([]Exclusion(nil), a.exclusions...), e)
	return a
}

// WouldSelectFile decides selection by the zone and file dimensions
// alone, ignoring Exclusions. memberOf is the file's resolved Zone
// membership.
func (a Applicability) WouldSelectFile(path string, memberOf []ZoneName) bool {
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
func (a Applicability) ExcludedFile(path string) bool {
	for _, e := range a.exclusions {
		if e.ExcludesFile(path) {
			return true
		}
	}
	return false
}

// SelectsFile decides whether the file is a Rule Subject: selected by
// the dimensions and not excluded.
func (a Applicability) SelectsFile(path string, memberOf []ZoneName) bool {
	return a.WouldSelectFile(path, memberOf) && !a.ExcludedFile(path)
}

// WouldSelectZone decides Zone selection ignoring Exclusions.
func (a Applicability) WouldSelectZone(name ZoneName) bool {
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
func (a Applicability) ExcludedZone(name ZoneName) bool {
	for _, e := range a.exclusions {
		if e.ExcludesZone(name) {
			return true
		}
	}
	return false
}

// SelectsZone decides whether the Zone is a Rule Subject.
func (a Applicability) SelectsZone(name ZoneName) bool {
	return a.WouldSelectZone(name) && !a.ExcludedZone(name)
}
