// Package exprenv defines the typed environment for Layer-1 expr
// predicates. Expressions are type-checked at load time against these Go
// structs (the reason expr was chosen over CEL) and must evaluate to bool.
package exprenv

import (
	"fmt"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// File is the per-file view exposed to expr invariants as `file`.
type File struct {
	// Path is repo-relative with forward slashes.
	Path string `expr:"path"`
	// Name is the base name, Stem the name without final extension, Ext
	// the extension including the dot, Dir the repo-relative directory.
	Name string `expr:"name"`
	Stem string `expr:"stem"`
	Ext  string `expr:"ext"`
	Dir  string `expr:"dir"`
	// Module is the arclint module the rule is evaluating the file under.
	Module string `expr:"module"`
	// Lines is the newline-based line count, Size the byte size.
	Lines int `expr:"lines"`
	Size  int `expr:"size"`
	// Imports lists the file's import paths (Go target), empty otherwise.
	Imports []string `expr:"imports"`
}

// Env is the root expr environment.
type Env struct {
	File File `expr:"file"`
}

// Compile type-checks an assertion against Env and requires a bool result.
func Compile(src string) (*vm.Program, error) {
	prog, err := expr.Compile(src, expr.Env(Env{}), expr.AsBool())
	if err != nil {
		return nil, fmt.Errorf("expr %q: %w", src, err)
	}
	return prog, nil
}

// Run evaluates a compiled assertion for one file.
func Run(prog *vm.Program, f File) (bool, error) {
	out, err := expr.Run(prog, Env{File: f})
	if err != nil {
		return false, err
	}
	b, ok := out.(bool)
	if !ok {
		return false, fmt.Errorf("expr returned %T, want bool", out)
	}
	return b, nil
}
