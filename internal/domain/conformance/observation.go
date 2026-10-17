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
	// resolvable zone path.
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

const (
	importTargetFile       = "file"
	importTargetDirectory  = "directory"
	importTargetUnresolved = "unresolved"
)

// Target returns the resolver's precision and repository-relative endpoint.
// Package resolution never implies a dependency on a particular member file.
func (i Import) Target() (kind, target string) {
	if i.Class == ImportInternal {
		if i.TargetFile != "" {
			return importTargetFile, i.TargetFile
		}
		if i.TargetDir != "" {
			return importTargetDirectory, i.TargetDir
		}
	}
	return importTargetUnresolved, ""
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
	Exported bool
	// Abstract marks a class the language declares abstract, which
	// cannot be instantiated and may leave its methods without a body:
	// a TypeScript abstract class, or a Python class deriving ABC or
	// Protocol, or naming ABCMeta as its metaclass, by the name the
	// class statement writes. False for every other kind; an interface
	// is abstract by its kind.
	Abstract  bool
	StartLine int
	EndLine   int
	// Params and Results carry the syntactic signature of func and
	// method declarations; absent for every other kind.
	Params  []DeclarationParam
	Results []string
	// Holds lists, for a field, the declarations whose values the field
	// holds: its type and, through pointers, collections, and type
	// arguments, the element types inside it, never the types inside a
	// function signature. Valid when FieldTypesAvailable; absent for
	// every other kind.
	Holds []Reference
	// Type is, for a field, the declaration its own type names, through
	// pointers, the repository's type aliases, and optional or nullable
	// wrappers: OrderID for *OrderID, an alias of OrderID, or
	// OrderID | undefined, the collection for []OrderID or
	// list[OrderID]. Valid when FieldTypesAvailable; zero for every
	// other kind.
	Type Reference
}

// Call is one source call: the callee identifier as written, the line
// of the call, and the enclosing function or method with the type that
// owns it. Target and Receiver are valid when ResolvedCallsAvailable.
type Call struct {
	Callee    string
	Line      int
	Enclosing string
	// EnclosingOwner names the type the enclosing method belongs to,
	// "" for a function.
	EnclosingOwner string
	// Target is the declaration the call reaches.
	Target Reference
	// Receiver is, for a method call, the declared type of the value
	// the method is called on, which may differ from the Target's
	// owner when the method is promoted from an embedded or inherited
	// type; the zero Reference for a call that is not a method call.
	Receiver Reference
}

// Resolution says how far the language's own resolution followed a
// reference.
type Resolution string

const (
	// ResolutionDeclared reaches a declaration in an observed file of
	// the repository.
	ResolutionDeclared Resolution = "declared"
	// ResolutionExternal reaches a declaration outside the repository:
	// the standard library, a dependency, or a language builtin.
	ResolutionExternal Resolution = "external"
	// ResolutionLocal reaches a type, or a member of one, that the code
	// declares without a file-level name: an interface or struct
	// literal, or a type declared inside a function. No recorded term
	// can be one.
	ResolutionLocal Resolution = "local"
	// ResolutionUndetermined means resolution could not follow the
	// reference; Reason says why.
	ResolutionUndetermined Resolution = "undetermined"
)

// Valid reports whether the value is a defined enum member.
func (r Resolution) Valid() bool {
	switch r {
	case ResolutionDeclared, ResolutionExternal, ResolutionLocal, ResolutionUndetermined:
		return true
	}
	return false
}

// Reference is the declaration a call or a field's type reaches.
type Reference struct {
	Resolution Resolution
	// File is the repository-relative file holding the declaration,
	// for a declared reference.
	File string
	// Package is the import path or module outside the repository, for
	// an external reference; "" for a language builtin, or when the
	// language resolves the reference outside the repository without
	// knowing which package declares it.
	Package string
	// Owner is the type that declares the member reached, "" when the
	// reference reaches a type or a function.
	Owner string
	// Name is the declared name reached.
	Name string
	// Reason says why resolution stopped, for an undetermined
	// reference.
	Reason string
}

// IsZero reports the absent Reference: a call that is not a method
// call has no Receiver.
func (r Reference) IsZero() bool { return r == Reference{} }

// Declares reports whether the reference reaches the type declared
// under name in file, as the reached type or as the owner of the
// member reached.
func (r Reference) Declares(file, name string) bool {
	if r.Resolution != ResolutionDeclared || r.File != file {
		return false
	}
	if r.Owner != "" {
		return r.Owner == name
	}
	return r.Name == name
}

// validate enforces the shape each resolution carries, so an
// unresolved reference can never pass for a resolved one.
func (r Reference) validate(observed map[string]bool) error {
	switch r.Resolution {
	case ResolutionDeclared:
		if r.Name == "" {
			return fmt.Errorf("declared reference without a name")
		}
		if !observed[r.File] {
			return fmt.Errorf("declared reference %s names unobserved file %q", r.Name, r.File)
		}
		if r.Package != "" || r.Reason != "" {
			return fmt.Errorf("declared reference %s carries a package or a reason", r.Name)
		}
	case ResolutionExternal:
		if r.Name == "" {
			return fmt.Errorf("external reference without a name")
		}
		if r.File != "" || r.Reason != "" {
			return fmt.Errorf("external reference %s carries a file or a reason", r.Name)
		}
	case ResolutionLocal:
		if r.Name == "" {
			return fmt.Errorf("local reference without a name")
		}
		if r.File != "" || r.Package != "" || r.Owner != "" || r.Reason != "" {
			return fmt.Errorf("local reference %s carries a file, package, owner, or reason", r.Name)
		}
	case ResolutionUndetermined:
		if strings.TrimSpace(r.Reason) == "" {
			return fmt.Errorf("undetermined reference without a reason")
		}
		if r.File != "" {
			return fmt.Errorf("undetermined reference carries file %q", r.File)
		}
	default:
		return fmt.Errorf("reference resolution %q invalid", r.Resolution)
	}
	return nil
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
	// CallsAvailable records whether the language can honestly supply
	// calls for this file, and whether they were requested.
	CallsAvailable bool
	// Calls is the normalized call view, valid when CallsAvailable
	// and no ParseFailure.
	Calls []Call
	// ResolvedCallsAvailable records whether every call carries its
	// Target and Receiver, and whether they were requested.
	ResolvedCallsAvailable bool
	// FieldTypesAvailable records whether every field declaration
	// carries what it Holds, and whether it was requested.
	FieldTypesAvailable bool
}

// Supports states whether a requested fact class is available.
func (f LanguageFacts) Supports(fact rule.Fact) bool {
	switch fact {
	case rule.FactImports:
		return f.ImportsAvailable && f.ParseFailure == ""
	case rule.FactDeclarations:
		return f.DeclarationsAvailable && f.ParseFailure == ""
	case rule.FactCalls:
		return f.CallsAvailable && f.ParseFailure == ""
	case rule.FactResolvedCalls:
		return f.ResolvedCallsAvailable && f.CallsAvailable && f.ParseFailure == ""
	case rule.FactFieldTypes:
		return f.FieldTypesAvailable && f.DeclarationsAvailable && f.ParseFailure == ""
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
			if d.Abstract && d.Kind != declKindClass {
				return Observations{}, fmt.Errorf("observations: %s: %s %s is marked abstract; only a class carries the mark", path, d.Kind, d.Name)
			}
		}
		if err := validResolution(f, seen); err != nil {
			return Observations{}, fmt.Errorf("observations: %s: %w", path, err)
		}
		copied[path] = copyLanguageFacts(f)
	}
	return Observations{files: sorted, facts: copied}, nil
}

// validResolution enforces that resolved facts are complete and
// honest: every call of a file whose calls are resolved carries a
// valid Target, every field of a file whose field types are resolved
// carries valid references, and no other declaration or call carries
// any.
func validResolution(f LanguageFacts, observed map[string]bool) error {
	if f.ResolvedCallsAvailable && !f.CallsAvailable {
		return fmt.Errorf("resolved calls without calls")
	}
	if f.FieldTypesAvailable && !f.DeclarationsAvailable {
		return fmt.Errorf("field types without declarations")
	}
	for _, c := range f.Calls {
		if !f.ResolvedCallsAvailable {
			if !c.Target.IsZero() || !c.Receiver.IsZero() {
				return fmt.Errorf("call %s at line %d carries a resolution that was not supplied", c.Callee, c.Line)
			}
			continue
		}
		if err := c.Target.validate(observed); err != nil {
			return fmt.Errorf("call %s at line %d: target: %w", c.Callee, c.Line, err)
		}
		if c.Receiver.IsZero() {
			continue
		}
		if err := c.Receiver.validate(observed); err != nil {
			return fmt.Errorf("call %s at line %d: receiver: %w", c.Callee, c.Line, err)
		}
	}
	for _, d := range f.Declarations {
		if d.Kind != declKindField || !f.FieldTypesAvailable {
			if len(d.Holds) > 0 || !d.Type.IsZero() {
				return fmt.Errorf("%s %s carries field types that were not supplied", d.Kind, d.Name)
			}
			continue
		}
		if err := d.Type.validate(observed); err != nil {
			return fmt.Errorf("field %s.%s: type: %w", d.Owner, d.Name, err)
		}
		for _, ref := range d.Holds {
			if err := ref.validate(observed); err != nil {
				return fmt.Errorf("field %s.%s: %w", d.Owner, d.Name, err)
			}
		}
	}
	return nil
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
	return copyLanguageFacts(f), ok
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

func copyLanguageFacts(f LanguageFacts) LanguageFacts {
	f.Imports = append([]Import(nil), f.Imports...)
	f.Declarations = copyDeclarations(f.Declarations, true)
	f.Calls = append([]Call(nil), f.Calls...)
	return f
}

// copyDeclarations copies declarations so a reader cannot change the
// observed ones, with the types each field holds when holds is set.
func copyDeclarations(ds []Declaration, holds bool) []Declaration {
	out := append([]Declaration(nil), ds...)
	for i := range out {
		out[i].Params = append([]DeclarationParam(nil), out[i].Params...)
		out[i].Results = append([]string(nil), out[i].Results...)
		if holds {
			out[i].Holds = append([]Reference(nil), out[i].Holds...)
		} else {
			out[i].Holds, out[i].Type = nil, Reference{}
		}
	}
	return out
}
