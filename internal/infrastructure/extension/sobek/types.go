// Package sobekextension implements the TypeScript extension SDK:
// discovery under .arclint/extensions/, in-process transpilation via
// esbuild, execution on sobek (the k6 pattern), a two-phase
// register-then-run lifecycle, and a sandbox with no ambient I/O.
//
// The types in this file are the host/extension wire contract. The
// TypeScript declarations rule authors see are generated from these
// structs (tools/gensdktypes, via tygo), so the .d.ts can never drift from
// the host.
package sobekextension

//go:generate go run ../../../../tools/gensdktypes

// FileInfo is one repository file as exposed to ctx.files().
type FileInfo struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Stem string `json:"stem"`
	Ext  string `json:"ext"`
	Dir  string `json:"dir"`
	Size int    `json:"size"`
}

// ImportInfo is one classified import occurrence as exposed to
// ctx.imports(path) and ctx.facts(path).imports, for every active language target.
type ImportInfo struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	// Class is stdlib | internal | external | unknown | cgo.
	Class string `json:"class"`
	// TargetDir is the repo-relative package directory for internal
	// imports resolved into the tree, else "".
	TargetDir string `json:"targetDir"`
	// TargetFile is the repo-relative file an internal import resolves to
	// for file-granular languages (JS/TS, Python), else "".
	TargetFile string `json:"targetFile"`
	// TargetZones is the sorted membership of the exact target file or
	// the union for its package directory. It grants no target access.
	TargetZones []string `json:"targetZones"`
	// TargetObserved distinguishes an observed unzoned target from a target absent from the scan.
	TargetObserved bool `json:"targetObserved"`
}

// ParamInfo is one function or method parameter at the syntactic tier.
type ParamInfo struct {
	// Name is the parameter identifier; "" for unnamed parameters.
	Name string `json:"name,omitempty"`
	// Type is the source-text type annotation, never resolved.
	Type string `json:"type,omitempty"`
	// Optional marks a marker or default the language expresses.
	Optional bool `json:"optional,omitempty"`
	// Variadic marks rest or splat parameters.
	Variadic bool `json:"variadic,omitempty"`
}

// DeclInfo is one declaration as exposed through ctx.facts(path).
type DeclInfo struct {
	// Kind is a small cross-language vocabulary: struct, interface,
	// type, class, enum, func, method, field, const, var.
	Kind string `json:"kind"`
	Name string `json:"name"`
	// Owner names the enclosing declaration for members (receiver
	// type, interface, class), else "".
	Owner string `json:"owner"`
	// Exported follows the language's own visibility convention.
	Exported bool `json:"exported"`
	// Abstract marks a class its own declaration states to be abstract:
	// a TypeScript abstract class, or a Python class whose bases name
	// abc.ABC or typing.Protocol, or whose metaclass is abc.ABCMeta, as
	// the library provides them. False for every other kind.
	Abstract bool `json:"abstract"`
	// Generic marks a func, method, or type that declares its own type
	// parameters. False for every other declaration.
	Generic   bool `json:"generic"`
	StartLine int  `json:"startLine"`
	EndLine   int  `json:"endLine"`
	// Params and Results carry the syntactic signature of func and
	// method decls; absent for every other kind.
	Params  []ParamInfo `json:"params,omitempty"`
	Results []string    `json:"results,omitempty"`
	// Holds lists, for a field when field types are available, the
	// declarations whose values the field holds: its type and the
	// element types inside pointers, collections, and type arguments.
	Holds []ReferenceInfo `json:"holds,omitempty"`
	// Type is, for a field when field types are available, the
	// declaration the field's own type names, through pointers, the
	// repository's type aliases, and optional or nullable wrappers; for
	// a func or method, the declaration its first written result names
	// the same way (T for (T, error), T for a TypeScript Promise<T>).
	Type *ReferenceInfo `json:"type,omitempty"`
}

// ReferenceInfo is the declaration a call or a field's type reaches,
// as the language's own resolution followed it.
type ReferenceInfo struct {
	// Resolution is declared (an observed file of the repository),
	// external (the standard library, a dependency, or a builtin),
	// local (a type without a file-level name), or undetermined.
	Resolution string `json:"resolution"`
	// File is the repository-relative file of a declared reference.
	File string `json:"file,omitempty"`
	// Package is the import path or module of an external reference.
	Package string `json:"package,omitempty"`
	// Owner is the type declaring the member reached, "" for a type or
	// a function.
	Owner string `json:"owner,omitempty"`
	Name  string `json:"name,omitempty"`
	// Reason says why an undetermined reference was not followed.
	Reason string `json:"reason,omitempty"`
}

// CallInfo is one source call: the callee identifier as written, its
// line, the enclosing function or method and the type owning it, and,
// when resolved calls are available, the declaration it reaches and
// the declared type of the value a method is called on.
type CallInfo struct {
	Callee         string         `json:"callee"`
	Line           int            `json:"line"`
	Enclosing      string         `json:"enclosing"`
	EnclosingOwner string         `json:"enclosingOwner"`
	Target         *ReferenceInfo `json:"target,omitempty"`
	Receiver       *ReferenceInfo `json:"receiver,omitempty"`
}

// FactsInfo exposes one file's already-observed declarations and imports.
// Availability distinguishes unsupported or failed facts from empty results.
type FactsInfo struct {
	Path string `json:"path"`
	// Zones is the sorted Zone membership of this selected source file.
	Zones []string `json:"zones"`
	// Package is the Go package clause; "" for other languages.
	Package string `json:"package"`
	// ImportsAvailable is false when imports are unsupported or parsing failed.
	ImportsAvailable bool `json:"importsAvailable"`
	// Imports contains parsed occurrences, never inferred runtime dependencies.
	// Empty with ImportsAvailable true means the file has no imports.
	Imports []ImportInfo `json:"imports"`
	// Dependencies preserves incoming and outgoing evidence involving this
	// subject. A directory target is never an exact file dependency.
	Dependencies []DependencyInfo `json:"dependencies"`

	// DeclarationsAvailable is false when declarations were not supplied or parsing failed.
	DeclarationsAvailable bool       `json:"declarationsAvailable"`
	Decls                 []DeclInfo `json:"decls"`
	// FieldTypesAvailable is true when every field declaration carries
	// the declarations it holds.
	FieldTypesAvailable bool `json:"fieldTypesAvailable"`
	// CallsAvailable is false when calls were not supplied or parsing failed.
	CallsAvailable bool       `json:"callsAvailable"`
	Calls          []CallInfo `json:"calls"`
	// ResolvedCallsAvailable is true when every call carries its target.
	ResolvedCallsAvailable bool   `json:"resolvedCallsAvailable"`
	ParseError             string `json:"parseError,omitempty"`
}

// DependencyInfo is the SDK view of the shared domain DependencyImport.
type DependencyInfo struct {
	SourcePath     string   `json:"sourcePath"`
	TargetPath     string   `json:"targetPath"`
	TargetKind     string   `json:"targetKind"`
	Specifier      string   `json:"specifier"`
	Line           int      `json:"line"`
	Classification string   `json:"classification"`
	SourceZones    []string `json:"sourceZones"`
	TargetZones    []string `json:"targetZones"`
	TargetObserved bool     `json:"targetObserved"`
}

// ViolationInput is what ctx.report() accepts from a rule. Severity
// is not part of the wire shape: in the target model it belongs to
// the Rule, never to one finding.
type ViolationInput struct {
	// SubjectPath names the selected subject when Path is a distinct evidence
	// location. The domain validates that location against supplied dependencies.
	SubjectPath string `json:"subjectPath,omitempty"`
	Path        string `json:"path"`
	Message     string `json:"message"`
	Line        int    `json:"line,omitempty"`
	FixHint     string `json:"fixHint,omitempty"`
}

// DomainInvariantInfo is one recorded invariant as exposed through
// ctx.domain(): the key that names the method enforcing it (an
// aggregate's) or the constructor that enforces it (a value object's),
// and the statement. Line is where it is written in the project's
// domain file; 0 when the vocabulary was not read from a file.
type DomainInvariantInfo struct {
	Key       string `json:"key"`
	Statement string `json:"statement"`
	Line      int    `json:"line"`
}

// DomainAssertionInfo is one recorded assertion as exposed through
// ctx.domain(). Key names the checking method on the owner; On names
// the operation that must call it.
type DomainAssertionInfo struct {
	Key       string `json:"key"`
	On        string `json:"on"`
	Statement string `json:"statement"`
	Line      int    `json:"line"`
}

// DomainEntityInfo is one member entity of an aggregate as exposed
// through ctx.domain(). Identity names the value object that
// identifies it; empty when implied.
type DomainEntityInfo struct {
	Name       string   `json:"name"`
	Definition string   `json:"definition"`
	Identity   string   `json:"identity,omitempty"`
	Aliases    []string `json:"aliases,omitempty"`
	Line       int      `json:"line"`
}

// DomainAggregateInfo is one recorded aggregate as exposed through
// ctx.domain(): its root carries the aggregate's name; Identity names
// the value object identifying the root; Entities are the members;
// Repository and Factory name the declarations when recorded.
type DomainAggregateInfo struct {
	Name       string                `json:"name"`
	Definition string                `json:"definition"`
	Identity   string                `json:"identity"`
	Aliases    []string              `json:"aliases,omitempty"`
	Entities   []DomainEntityInfo    `json:"entities"`
	Invariants []DomainInvariantInfo `json:"invariants"`
	Assertions []DomainAssertionInfo `json:"assertions"`
	Repository string                `json:"repository,omitempty"`
	Factory    string                `json:"factory,omitempty"`
	Line       int                   `json:"line"`
}

// DomainValueObjectInfo is one recorded value object as exposed
// through ctx.domain(), with the invariants its constructor enforces.
type DomainValueObjectInfo struct {
	Name       string                `json:"name"`
	Definition string                `json:"definition"`
	Aliases    []string              `json:"aliases,omitempty"`
	Invariants []DomainInvariantInfo `json:"invariants"`
	Line       int                   `json:"line"`
}

// DomainEventInfo is one recorded domain event as exposed through
// ctx.domain(). RaisedBy names the aggregate that raises it; empty
// when not recorded.
type DomainEventInfo struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
	RaisedBy   string `json:"raisedBy,omitempty"`
	Line       int    `json:"line"`
}

// DomainServiceInfo is one recorded domain service as exposed through
// ctx.domain(), with the assertions that state its contract.
type DomainServiceInfo struct {
	Name       string                `json:"name"`
	Definition string                `json:"definition"`
	Assertions []DomainAssertionInfo `json:"assertions"`
	Line       int                   `json:"line"`
}

// DomainTermInfo is one recorded term that carries a name and a
// definition and nothing else: a specification.
type DomainTermInfo struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
	Line       int    `json:"line"`
}

// DomainQuestionInfo is one open question recorded in a bounded
// context as exposed through ctx.domain().
type DomainQuestionInfo struct {
	Key  string `json:"key"`
	Text string `json:"text"`
	Line int    `json:"line"`
}

// DomainContextInfo is one bounded context and its recorded terms as
// exposed through ctx.domain(). Line is where the context is written
// in the project's domain file.
type DomainContextInfo struct {
	Name           string                  `json:"name"`
	Definition     string                  `json:"definition"`
	Aggregates     []DomainAggregateInfo   `json:"aggregates"`
	ValueObjects   []DomainValueObjectInfo `json:"valueObjects"`
	Events         []DomainEventInfo       `json:"events"`
	Services       []DomainServiceInfo     `json:"services"`
	Specifications []DomainTermInfo        `json:"specifications"`
	Questions      []DomainQuestionInfo    `json:"questions"`
	Line           int                     `json:"line"`
}

// DomainRelationInfo is one context-map edge as exposed through
// ctx.domain(). Line is where the relation is written in the
// project's domain file.
type DomainRelationInfo struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Kind        string `json:"kind"`
	Description string `json:"description,omitempty"`
	Line        int    `json:"line"`
}

// DomainInfo is the project's recorded domain model as exposed through
// ctx.domain(): empty collections when the project records none.
// Read-only: declaring knowledge never creates a diagnostic by itself.
// Source is the repository-relative path of the domain file the model
// is (or would be) recorded in, so a finding about a recorded term
// anchors there without the extension spelling the file name. Project
// names the software whose domain is recorded.
type DomainInfo struct {
	Source    string               `json:"source"`
	Project   string               `json:"project"`
	Contexts  []DomainContextInfo  `json:"contexts"`
	Relations []DomainRelationInfo `json:"relations"`
}
