// Package python produces normalized Language Facts for Python:
// lexer-grade import extraction with documented false-negative
// classes, an embedded stdlib table (sys.stdlib_module_names of the
// pinned CPython), and manifest-based (pyproject.toml) external
// classification. The extraction and resolution mechanics are the
// proven legacy analyzer's; the producer claims exactly the files the
// target vocabulary assigns Python (.py).
package python

//go:generate go run ../../../../tools/genpystdlib -out stdlib_gen.go -pkg python

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// Producer implements the observation FactProducer seam for Python.
type Producer struct{}

// NewProducer returns the Python fact producer.
func NewProducer() Producer { return Producer{} }

// Language identifies the facts this producer supplies.
func (Producer) Language() rule.Language { return rule.LanguagePython }

// Facts analyzes every .py file among the observed files, producing
// the requested fact classes: imports always, declarations (pinned
// tree-sitter grammar, deterministic) only when asked for.
func (Producer) Facts(root string, files []conformance.ObservedFile, requested []rule.Fact) (map[string]conformance.LanguageFacts, error) {
	wantDeclarations := false
	wantCalls := false
	for _, f := range requested {
		if f == rule.FactDeclarations {
			wantDeclarations = true
		}
		if f == rule.FactCalls {
			wantCalls = true
		}
	}
	res := newResolver(root, files)
	out := map[string]conformance.LanguageFacts{}
	for _, f := range files {
		if rule.LanguageOf(f.Path) != rule.LanguagePython {
			continue
		}
		out[f.Path] = analyzeFile(res, root, f.Path, wantDeclarations, wantCalls)
	}
	return out, nil
}

func analyzeFile(res *resolver, root, rel string, wantDeclarations, wantCalls bool) conformance.LanguageFacts {
	facts := conformance.LanguageFacts{Language: rule.LanguagePython, ImportsAvailable: true}
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		facts.ParseFailure = fmt.Sprintf("read: %v", err)
		return facts
	}
	dir := path.Dir(rel)
	for _, ri := range extract(string(src)) {
		imp := conformance.Import{Path: ri.module, Line: ri.line}
		imp.Class, imp.TargetDir, imp.TargetFile = res.classify(dir, ri.module)
		facts.Imports = append(facts.Imports, imp)
	}
	if !wantDeclarations && !wantCalls {
		return facts
	}
	// A strict-parse failure yields honest fact absence without
	// poisoning the scanner import view.
	df := extractDeclarations(src)
	if df.ParseError != "" {
		return facts
	}
	if wantDeclarations {
		facts.DeclarationsAvailable = true
		facts.Declarations = df.Decls
	}
	if wantCalls {
		facts.CallsAvailable = true
		facts.Calls = df.Calls
	}
	return facts
}
