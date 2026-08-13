package lang

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
}
