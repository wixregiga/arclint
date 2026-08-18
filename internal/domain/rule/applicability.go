package rule

import "fmt"

// Applicability is the composable, inspectable selection of the Files,
// Folders, and Modules a Rule evaluates. Selector dimensions intersect;
// multiple values within one dimension form a union. Rule Exclusions
// remove only their selected subjects.
type Applicability struct {
	entireRepository bool
	modules          []ModuleName
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

// ModuleApplicability selects the members of the named Modules (union),
// optionally intersected with file globs.
func ModuleApplicability(modules []ModuleName, files ...Glob) (Applicability, error) {
	if len(modules) == 0 {
		return Applicability{}, fmt.Errorf("applicability: no module selected")
	}
	if err := uniqueValidModules("applicability", modules); err != nil {
		return Applicability{}, err
	}
	if err := validFileGlobs(files); err != nil {
		return Applicability{}, err
	}
	return Applicability{
		modules: append([]ModuleName(nil), modules...),
		files:   append([]Glob(nil), files...),
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
	return !a.entireRepository && len(a.modules) == 0
}

// EntireRepository reports repository-wide selection.
func (a Applicability) EntireRepository() bool { return a.entireRepository }

// Modules returns the selected Module names.
func (a Applicability) Modules() []ModuleName {
	return append([]ModuleName(nil), a.modules...)
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

// WouldSelectFile decides selection by the module and file dimensions
// alone, ignoring Exclusions. memberOf is the file's resolved Module
// membership.
func (a Applicability) WouldSelectFile(path string, memberOf []ModuleName) bool {
	if a.IsZero() {
		return false
	}
	if !a.entireRepository {
		member := false
		for _, m := range a.modules {
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
func (a Applicability) SelectsFile(path string, memberOf []ModuleName) bool {
	return a.WouldSelectFile(path, memberOf) && !a.ExcludedFile(path)
}

// WouldSelectModule decides Module selection ignoring Exclusions.
func (a Applicability) WouldSelectModule(name ModuleName) bool {
	if a.entireRepository {
		return true
	}
	for _, m := range a.modules {
		if m == name {
			return true
		}
	}
	return false
}

// ExcludedModule reports whether an Exclusion removes the Module.
func (a Applicability) ExcludedModule(name ModuleName) bool {
	for _, e := range a.exclusions {
		if e.ExcludesModule(name) {
			return true
		}
	}
	return false
}

// SelectsModule decides whether the Module is a Rule Subject.
func (a Applicability) SelectsModule(name ModuleName) bool {
	return a.WouldSelectModule(name) && !a.ExcludedModule(name)
}
