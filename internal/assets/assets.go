// Package assets holds the embedded defaults that `arclint init` writes
// into a fresh .arclint/ directory: the default rules.yaml (the full
// commented example from docs/design/rules.md §6) and the three builtin
// templates (repo, service, component — docs/design/templating.md §1).
//
// Two encodings keep the embedded tree friendly to the Go toolchain:
//
//  1. Every payload file under a template's files/ carries a ".tmpl"
//     suffix so the toolchain never tries to compile scaffolded .go
//     sources that contain {{ }} interpolation.
//  2. Interpolated path segments use embed-safe placeholders (Go's embed
//     rejects names containing '|', and "{{ name | kebab }}" is not a
//     valid embed name). DecodePath expands them back to the on-disk
//     spelling at init-copy time.
package assets

import (
	"embed"
	"io/fs"
	"strings"
)

//go:embed all:defaults
var defaults embed.FS

// DefaultRules returns the default rules.yaml content shipped by
// `arclint init`. It is the docs/design/rules.md §6 example, adjusted so
// a fresh init passes `arclint check` (see comments inside the file).
func DefaultRules() []byte {
	b, err := defaults.ReadFile("defaults/rules.yaml")
	if err != nil {
		// Unreachable: the file is embedded at compile time.
		panic("assets: embedded defaults/rules.yaml missing: " + err.Error())
	}
	return b
}

// Templates returns the embedded builtin template tree. Its top-level
// directories are the template names (repo, service, component); paths
// inside it are encoded — pass each through DecodePath before writing
// to disk.
func Templates() fs.FS {
	sub, err := fs.Sub(defaults, "defaults/templates")
	if err != nil {
		// Unreachable: the directory is embedded at compile time.
		panic("assets: embedded defaults/templates missing: " + err.Error())
	}
	return sub
}

// pathDecodes maps embed-safe placeholder path segments to their real
// on-disk names. Placeholders exist only because Go's embed rejects the
// literal interpolated spelling.
var pathDecodes = map[string]string{
	"__name_kebab__": "{{ name | kebab }}",
}

// DecodePath translates an embedded template path to its on-disk name:
// it strips the trailing ".tmpl" payload suffix and expands placeholder
// segments. Directory paths (no suffix, no placeholder) pass through
// unchanged.
func DecodePath(p string) string {
	p = strings.TrimSuffix(p, ".tmpl")
	for enc, dec := range pathDecodes {
		p = strings.ReplaceAll(p, enc, dec)
	}
	return p
}
