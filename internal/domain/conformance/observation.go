package conformance

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// ImportClass is the classification of one import specifier.
type ImportClass string

const (
	// ImportStdlib is a toolchain standard-library import.
	ImportStdlib ImportClass = "stdlib"
	// ImportInternal resolves to a file or package inside the
	// repository.
	ImportInternal ImportClass = "internal"
	// ImportExternal resolves to a declared third-party dependency.
	ImportExternal ImportClass = "external"
	// ImportUnknown classifies neither stdlib, internal, nor a declared
	// dependency; the repository's unknown-import policy decides its
	// effect.
	ImportUnknown ImportClass = "unknown"
	// ImportCgo is Go's "C" pseudo-import: neither a package nor a
	// resolvable module path.
	ImportCgo ImportClass = "cgo"
)

// Valid reports whether the value is a defined enum member.
func (c ImportClass) Valid() bool {
	switch c {
	case ImportStdlib, ImportInternal, ImportExternal, ImportUnknown, ImportCgo:
		return true
	}
	return false
}

// Import is one classified import occurrence in one file.
type Import struct {
	// Path is the specifier as written.
	Path string
	Line int
	// Class is the exact classification.
	Class ImportClass
	// TargetDir is the repo-relative directory an internal import
	// resolves to for package-granular languages ("." = repo root), else
	// "".
	TargetDir string
	// TargetFile is the repo-relative file for file-granular languages
	// when the specifier resolves to one file, else "".
	TargetFile string
}

// DeclarationParam is one parameter of a func or method declaration
// at the syntactic tier.
type DeclarationParam struct {
	// Name is the parameter identifier, "" when unnamed.
	Name string
	// Type is the source-text annotation, never resolved.
	Type string
	// Optional marks a marker or default the language expresses.
	Optional bool
	// Variadic marks rest or splat parameters.
	Variadic bool
}

// declarationKinds is the closed cross-language declaration
// vocabulary. Every language emits only these kinds; a producer
// inventing its own kind fails at observation construction, so the
// SDK's facts stay one reliable interface across languages.
var declarationKinds = map[string]bool{
	"struct": true, "interface": true, "type": true, "class": true,
	"enum": true, "func": true, "method": true, "field": true,
	"const": true, "var": true,
}

// Declaration is one source declaration in the shared cross-language
// vocabulary: the same shape for every supported language, with each
// language emitting only the kinds it honestly has.
type Declaration struct {
	// Kind is one of: struct, interface, type, class, enum, func,
	// method, field, const, var.
	Kind string
	Name string
	// Owner names the enclosing declaration for members (receiver
	// type, interface, class), else "".
	Owner string
	// Exported follows the language's own visibility convention.
	Exported  bool
	StartLine int
	EndLine   int
	// Params and Results carry the syntactic signature of func and
	// method declarations; absent for every other kind.
	Params  []DeclarationParam
	Results []string
}

// LanguageFacts is the immutable normalized description of one file's
// code facts available to Rule enforcement. Facts describe observed
// code, never intended architecture, and a parse failure is
// distinguishable from a genuine empty result.
type LanguageFacts struct {
	// Language that produced the facts.
	Language rule.Language
	// Package is the language's own grouping clause (the Go package
	// name), "" where the language has none.
	Package string
	// ImportsAvailable records whether the language can honestly supply
	// imports for this file.
	ImportsAvailable bool
	// DeclarationsAvailable records whether the language can honestly
	// supply declarations for this file, and whether they were
	// requested: absence of the fact is never an empty result.
	DeclarationsAvailable bool
	// ParseFailure is non-empty when analysis failed; the file's facts
	// are then unusable, which is not the same as empty.
	ParseFailure string
	// Imports is the classified import view, valid when
	// ImportsAvailable and no ParseFailure.
	Imports []Import
	// Declarations is the normalized declaration view, valid when
	// DeclarationsAvailable and no ParseFailure.
	Declarations []Declaration
}

// Supports states whether a requested fact class is available.
func (f LanguageFacts) Supports(fact rule.Fact) bool {
	switch fact {
	case rule.FactImports:
		return f.ImportsAvailable && f.ParseFailure == ""
	case rule.FactDeclarations:
		return f.DeclarationsAvailable && f.ParseFailure == ""
	default:
		return false
	}
}

// ObservedFile is one regular file of the walked repository.
type ObservedFile struct {
	// Path is repo-root-relative with forward slashes.
	Path string
	Size int64
}

// Content is the deterministic capability to read one observed file's
// bytes for Extension evaluation (ctx.read). Production supplies a
// lazy repository reader; Rule Tests supply fixture content directly
// so temporary fixture materialization cannot invalidate later reads.
type Content interface {
	Read(path string) (string, error)
}

// MapContent is Content backed by an immutable in-memory path-to-bytes map.
type MapContent struct {
	files map[string]string
}

// Read returns the content recorded for path.
func (m MapContent) Read(path string) (string, error) {
	s, ok := m.files[path]
	if !ok {
		return "", fmt.Errorf("file not found")
	}
	return s, nil
}

// NewMapContent returns MapContent from an in-memory path-to-bytes map.
// The returned capability holds an independent copy of the map.
func NewMapContent(files map[string]string) MapContent {
	cp := make(map[string]string, len(files))
	for k, v := range files {
		cp[k] = v
	}
	return MapContent{files: cp}
}

// Observations is the immutable normalized input to one Conformance
// Check: the deterministic file list and per-file Language Facts.
type Observations struct {
	files   []ObservedFile
	facts   map[string]LanguageFacts
	content Content
}

// NewObservations validates and orders the observed repository. Facts
// must describe observed files.
func NewObservations(files []ObservedFile, facts map[string]LanguageFacts) (Observations, error) {
	sorted := append([]ObservedFile(nil), files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	seen := map[string]bool{}
	for _, f := range sorted {
		if err := validObservedPath(f.Path); err != nil {
			return Observations{}, err
		}
		if seen[f.Path] {
			return Observations{}, fmt.Errorf("observations: duplicate file %q", f.Path)
		}
		seen[f.Path] = true
	}
	copied := make(map[string]LanguageFacts, len(facts))
	for path, f := range facts {
		if !seen[path] {
			return Observations{}, fmt.Errorf("observations: facts for unobserved file %q", path)
		}
		if !f.Language.Valid() {
			return Observations{}, fmt.Errorf("observations: %s: language %q invalid", path, f.Language)
		}
		for _, imp := range f.Imports {
			if !imp.Class.Valid() {
				return Observations{}, fmt.Errorf("observations: %s: import class %q invalid", path, imp.Class)
			}
		}
		for _, d := range f.Declarations {
			if !declarationKinds[d.Kind] {
				return Observations{}, fmt.Errorf("observations: %s: declaration kind %q is outside the closed cross-language vocabulary", path, d.Kind)
			}
		}
		f.Imports = append([]Import(nil), f.Imports...)
		f.Declarations = append([]Declaration(nil), f.Declarations...)
		copied[path] = f
	}
	return Observations{files: sorted, facts: copied}, nil
}

func validObservedPath(p string) error {
	if p == "" {
		return fmt.Errorf("observations: empty file path")
	}
	if strings.HasPrefix(p, "/") || strings.Contains(p, "\\") {
		return fmt.Errorf("observations: %q is not repo-relative with forward slashes", p)
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("observations: %q contains an empty, dot, or dot-dot segment", p)
		}
	}
	return nil
}

// Files returns the deterministic file list.
func (o Observations) Files() []ObservedFile { return append([]ObservedFile(nil), o.files...) }

// FactsFor returns the Language Facts for one file, when a language
// adapter produced them.
func (o Observations) FactsFor(path string) (LanguageFacts, bool) {
	f, ok := o.facts[path]
	return f, ok
}

// WithContent returns Observations that lend c for Extension content
// reads. The file list and Language Facts are unchanged.
func (o Observations) WithContent(c Content) Observations {
	o.content = c
	return o
}

// Content returns the content capability, or nil when none was
// supplied.
func (o Observations) Content() Content {
	return o.content
}
