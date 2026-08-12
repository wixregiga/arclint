// Package lang defines the shared import model every language target
// produces: one classified import occurrence per source statement, with
// tree-resolved targets where the language allows it.
package lang

import "strings"

// Class is the exact classification of one import specifier.
type Class string

const (
	ClassStdlib   Class = "stdlib"
	ClassInternal Class = "internal"
	ClassExternal Class = "external"
	ClassUnknown  Class = "unknown"
	// ClassCgo is Go's "C" pseudo-import.
	ClassCgo Class = "cgo"
)

// Import is one import occurrence in one file.
type Import struct {
	// Path is the specifier as written: a Go import path, a JS/TS module
	// specifier, or a Python dotted module (relative Python imports keep
	// their leading dots).
	Path string
	Line int
	// Alias is Go-specific: "", "_", ".", or an identifier.
	Alias string
	Class Class
	// TargetDir is the repo-relative directory the import resolves to for
	// internal imports resolved into the tree ("." = repo root), else "".
	TargetDir string
	// TargetFile is the repo-relative file for file-granular languages
	// (JS/TS, Python) when the specifier resolves to one file, else "".
	TargetFile string
}

// TargetOf maps a file path to the language target that analyzes it:
// "go", "ts", "py", or "" for files no target owns. This is the single
// extension-to-target mapping; the per-language analyzers select their
// files through it.
func TargetOf(path string) string {
	switch {
	case strings.HasSuffix(path, ".go"):
		return "go"
	case strings.HasSuffix(path, ".ts"), strings.HasSuffix(path, ".tsx"),
		strings.HasSuffix(path, ".js"), strings.HasSuffix(path, ".jsx"),
		strings.HasSuffix(path, ".mjs"), strings.HasSuffix(path, ".cjs"):
		return "ts"
	case strings.HasSuffix(path, ".py"):
		return "py"
	default:
		return ""
	}
}

// FileAnalysis is the extraction result for one source file.
type FileAnalysis struct {
	Path    string
	Imports []Import
	// ParseError is non-empty when the file could not be scanned; the
	// file was warned about and skipped, never fatal.
	ParseError string
	// HasCgo is Go-specific: the file imports "C".
	HasCgo bool
}
