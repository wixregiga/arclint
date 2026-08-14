// Package ext implements the TypeScript extension SDK: discovery under
// .arclint/extensions/, in-process transpilation via esbuild, execution on
// sobek (the k6 pattern), a two-phase register-then-run lifecycle, and a
// sandbox with no ambient I/O.
//
// The types in this file are the host/extension wire contract. The
// TypeScript declarations rule authors see are generated from these
// structs (tools/gensdktypes, via tygo), so the .d.ts can never drift from
// the host.
package ext

//go:generate go run ../../tools/gensdktypes

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
	// Name is the parameter identifier; "" for unnamed parameters (Go
	// interface methods) and TS destructuring patterns. Python splat
	// parameters keep their prefix: "*args", "**kwargs".
	Name string `json:"name,omitempty"`
	// Type is the source-text type annotation, whitespace-collapsed,
	// never resolved; "" when omitted. Signature comparison is
	// structural, not proof.
	Type string `json:"type,omitempty"`
	// Optional: a TS `?` marker or a TS/Python default value.
	Optional bool `json:"optional,omitempty"`
	// Variadic: Go `...T`, TS rest parameter, Python `*args`/`**kwargs`.
	Variadic bool `json:"variadic,omitempty"`
}

// DeclInfo is one declaration as exposed through ctx.facts(path).
type DeclInfo struct {
	// Kind is a small cross-language vocabulary: struct, interface,
	// type, class, enum, func, method, field, const, var.
	Kind string `json:"kind"`
	Name string `json:"name"`
	// Owner names the enclosing declaration for members (receiver type,
	// interface, class, enclosing function), else "".
	Owner string `json:"owner"`
	// Exported: Go identifier case, TS export/accessibility modifiers,
	// Python leading-underscore convention.
	Exported  bool `json:"exported"`
	StartLine int  `json:"startLine"`
	EndLine   int  `json:"endLine"`
	// Params and Results carry the syntactic signature of func and
	// method decls; absent for every other kind. Results holds result
	// type texts; an unannotated TS/Python return is an empty list.
	Params  []ParamInfo `json:"params,omitempty"`
	Results []string    `json:"results,omitempty"`
}

// FactsInfo is the declaration-fact view of one file as exposed to
// ctx.facts(path). A file that failed to parse has empty decls and a
// non-empty parseError; rules see absence, never a crash.
type FactsInfo struct {
	Path string `json:"path"`
	// Package is the Go package clause; "" for other languages.
	Package    string     `json:"package"`
	Decls      []DeclInfo `json:"decls"`
	ParseError string     `json:"parseError,omitempty"`
}

// ViolationInput is what ctx.report() accepts from a rule.
type ViolationInput struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Line    int    `json:"line,omitempty"`
	FixHint string `json:"fixHint,omitempty"`
	// Severity overrides the instance severity: error | warn | info.
	Severity string `json:"severity,omitempty"`
	// Contract overrides the rule type's contract clause for this one
	// finding: consumes | provides | invariant. A rule that enforces both
	// sides of a contract labels each finding truthfully.
	Contract string `json:"contract,omitempty"`
	// Blame overrides the rule type's blame side for this one finding:
	// consumer | provider.
	Blame string `json:"blame,omitempty"`
}
