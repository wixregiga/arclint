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
// ctx.imports(path).
type ImportInfo struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	// Class is stdlib | internal | external | unknown | cgo.
	Class string `json:"class"`
	// TargetDir is the repo-relative package directory for internal
	// imports resolved into the tree, else "".
	TargetDir string `json:"targetDir"`
}

// ViolationInput is what ctx.report() accepts from a rule.
type ViolationInput struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Line    int    `json:"line,omitempty"`
	FixHint string `json:"fixHint,omitempty"`
	// Severity overrides the instance severity: error | warn | info.
	Severity string `json:"severity,omitempty"`
}
