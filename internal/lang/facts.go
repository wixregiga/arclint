package lang

import "strings"

// FileFacts is the language-neutral syntax-fact tier (M8 ADR): the
// declarations a file makes, with owners, visibility, and line spans.
// Facts are computed lazily per file, only when a rule asks. Go facts
// come from go/parser (exact); TypeScript and Python facts come from
// pinned tree-sitter grammars via arclint-owned walkers. A file that
// fails to parse yields empty Decls plus ParseError; rules see absence,
// never a crash.
type FileFacts struct {
	Path string `json:"path"`
	// Package is the Go package clause; "" for other languages.
	Package string `json:"package"`
	Decls   []Decl `json:"decls"`
	// ParseError is non-empty when the file could not be parsed.
	ParseError string `json:"parseError,omitempty"`
}

// Decl is one declaration. Kind values are a small cross-language
// vocabulary; a language emits only the kinds it has:
//
//	go:     struct interface type func method field const var
//	ts/js:  class interface type enum func method field const var
//	py:     class func method
type Decl struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	// Owner names the enclosing declaration for members: the Go receiver
	// type or interface, the TS/Python class, the enclosing Python
	// function for nested defs. "" for top-level declarations.
	Owner string `json:"owner,omitempty"`
	// Exported: Go identifier case, TS export/accessibility modifiers,
	// Python leading-underscore convention.
	Exported  bool `json:"exported"`
	StartLine int  `json:"startLine"`
	EndLine   int  `json:"endLine"`
	// Params and Results carry the syntactic signature of func and
	// method decls: types are source text, whitespace-collapsed, never
	// resolved (M8 ADR: no go/types, no tsc). nil for every other kind.
	// Results holds result type texts; Go result names are dropped, and
	// an unannotated TS/Python return is an empty list, indistinguishable
	// from a Go func without results — signature comparison is
	// structural, not proof.
	Params  []Param  `json:"params,omitempty"`
	Results []string `json:"results,omitempty"`
}

// Param is one function or method parameter at the syntactic tier.
type Param struct {
	// Name is the parameter identifier; "" where the language allows
	// unnamed parameters (Go interface methods) or the pattern has no
	// single name (TS destructuring). Python splat parameters keep
	// their prefix ("*args", "**kwargs") so the two flavors stay
	// distinguishable.
	Name string `json:"name,omitempty"`
	// Type is the source-text type annotation, whitespace-collapsed;
	// "" when the language or the author omitted it.
	Type string `json:"type,omitempty"`
	// Optional: a TS `?` marker or a TS/Python default value.
	Optional bool `json:"optional,omitempty"`
	// Variadic: Go `...T`, TS rest parameter, Python `*args`/`**kwargs`.
	Variadic bool `json:"variadic,omitempty"`
}

// NormalizeType collapses every whitespace run in a source-text type to
// one space, so multi-line type expressions compare stably across files
// and languages.
func NormalizeType(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
