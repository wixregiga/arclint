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
// ctx.imports(path), for every active language target.
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
	Exported  bool `json:"exported"`
	StartLine int  `json:"startLine"`
	EndLine   int  `json:"endLine"`
	// Params and Results carry the syntactic signature of func and
	// method decls; absent for every other kind.
	Params  []ParamInfo `json:"params,omitempty"`
	Results []string    `json:"results,omitempty"`
}

// FactsInfo is the cross-language declaration-fact view of one file as
// exposed through ctx.facts(path). Languages return only declarations
// they can support honestly.
type FactsInfo struct {
	Path string `json:"path"`
	// Package is the Go package clause; "" for other languages.
	Package    string     `json:"package"`
	Decls      []DeclInfo `json:"decls"`
	ParseError string     `json:"parseError,omitempty"`
}

// ViolationInput is what ctx.report() accepts from a rule. Severity
// is not part of the wire shape: in the target model it belongs to
// the Rule, never to one finding.
type ViolationInput struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Line    int    `json:"line,omitempty"`
	FixHint string `json:"fixHint,omitempty"`
}

// DomainDefinitionInfo is one recorded project domain definition as
// exposed through ctx.domain(). Line is where the term is written in
// ubiquitous-language.yaml, so a finding about the term can anchor at
// the term; 0 when the vocabulary was not read from a file.
type DomainDefinitionInfo struct {
	Name       string   `json:"name"`
	Definition string   `json:"definition,omitempty"`
	Aliases    []string `json:"aliases,omitempty"`
	Aggregate  bool     `json:"aggregate,omitempty"`
	Line       int      `json:"line"`
}

// DomainInvariantInfo is one recorded invariant (statement + owner)
// inside a bounded context as exposed through ctx.domain(). Line is
// where the invariant is written in ubiquitous-language.yaml. ID is
// the cluster identity when the owner is an aggregate named contract.
type DomainInvariantInfo struct {
	Statement string `json:"statement"`
	Owner     string `json:"owner"`
	ID        string `json:"id,omitempty"`
	Line      int    `json:"line"`
}

// DomainAssertionInfo is one recorded assertion as exposed through
// ctx.domain(). ID names the checking method; On names the operation
// that must call it.
type DomainAssertionInfo struct {
	Statement string `json:"statement"`
	Owner     string `json:"owner"`
	ID        string `json:"id"`
	On        string `json:"on"`
	Line      int    `json:"line"`
}

// DomainSpecificationInfo is one recorded specification as exposed
// through ctx.domain(): a named predicate, never a flag on a value
// object.
type DomainSpecificationInfo struct {
	Name       string `json:"name"`
	Definition string `json:"definition,omitempty"`
	Line       int    `json:"line"`
}

// DomainContextInfo is one bounded context and its recorded terms as
// exposed through ctx.domain(). Line is where the context is written
// in ubiquitous-language.yaml.
type DomainContextInfo struct {
	Name           string                    `json:"name"`
	Entities       []DomainDefinitionInfo    `json:"entities"`
	ValueObjects   []DomainDefinitionInfo    `json:"valueObjects"`
	Invariants     []DomainInvariantInfo     `json:"invariants"`
	Assertions     []DomainAssertionInfo     `json:"assertions"`
	Specifications []DomainSpecificationInfo `json:"specifications"`
	Events         []DomainDefinitionInfo    `json:"events"`
	Line           int                       `json:"line"`
}

// DomainRelationInfo is one context-map edge as exposed through
// ctx.domain(). Line is where the relation is written in
// ubiquitous-language.yaml.
type DomainRelationInfo struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
	Line int    `json:"line"`
}

// DomainInfo is the project's recorded domain model as exposed through
// ctx.domain(): empty collections when the project records none.
// Read-only: declaring knowledge never creates a diagnostic by itself.
type DomainInfo struct {
	Contexts  []DomainContextInfo  `json:"contexts"`
	Relations []DomainRelationInfo `json:"relations"`
}
