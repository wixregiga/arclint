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
	// SubjectPath is the governed file; omitted means Path is also the subject.
	SubjectPath string `json:"subjectPath,omitempty"`
	Path        string `json:"path"`
	Message     string `json:"message"`
	Line        int    `json:"line,omitempty"`
	FixHint     string `json:"fixHint,omitempty"`
}

// DomainInvariantInfo is one recorded invariant as exposed through
// ctx.domain(): the key that names the method enforcing it (an
// aggregate's) or the constructor that enforces it (a value object's),
// and the statement. Line is where it is written in domain.arclint.yaml;
// 0 when the vocabulary was not read from a file.
type DomainInvariantInfo struct {
	Key       string `json:"key"`
	Statement string `json:"statement"`
	Line      int    `json:"line"`
}

// DomainAssertionInfo is one recorded assertion as exposed through
// ctx.domain(). Key names the checking method on the root; On names
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

// DomainTermInfo is one recorded term that carries a name and a
// definition and nothing else: a domain service or a specification.
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
// in domain.arclint.yaml.
type DomainContextInfo struct {
	Name           string                  `json:"name"`
	Definition     string                  `json:"definition"`
	Aggregates     []DomainAggregateInfo   `json:"aggregates"`
	ValueObjects   []DomainValueObjectInfo `json:"valueObjects"`
	Events         []DomainEventInfo       `json:"events"`
	Services       []DomainTermInfo        `json:"services"`
	Specifications []DomainTermInfo        `json:"specifications"`
	Questions      []DomainQuestionInfo    `json:"questions"`
	Line           int                     `json:"line"`
}

// DomainRelationInfo is one context-map edge as exposed through
// ctx.domain(). Line is where the relation is written in
// domain.arclint.yaml.
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

// ScopeInfo is resolved Rule evaluation policy, separate from evidence.
type ScopeInfo struct {
	// Files are the governed repository-relative file paths after exclusions.
	Files []string `json:"files"`
	// ExcludedFiles would be governed without the Rule's exclusions.
	ExcludedFiles []string        `json:"excludedFiles"`
	Exclusions    []ExclusionInfo `json:"exclusions"`
}

// ExclusionInfo records the selectors and reason for an evaluation exclusion.
// Exclusions never redact repository evidence.
type ExclusionInfo struct {
	Paths  []string `json:"paths"`
	Zones  []string `json:"zones"`
	Reason string   `json:"reason"`
}
