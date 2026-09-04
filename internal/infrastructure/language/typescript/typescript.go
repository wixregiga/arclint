// Package typescript produces normalized Language Facts for
// TypeScript: lexer-grade import extraction with documented
// false-negative classes, an embedded Node builtin table, and
// manifest-based (package.json) external classification. The
// extraction and resolution mechanics are the proven legacy jsts
// analyzer's; the producer claims only the files the target
// vocabulary assigns TypeScript (.ts, .tsx, minus .d.ts
// declaration files), while relative-specifier resolution still
// probes the full JS/TS extension set.
package typescript

//go:generate go run ../../../../tools/gennodestdlib -out stdlib_gen.go -pkg typescript

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/infrastructure/language/fanout"
)

// Producer implements the observation FactProducer seam for TypeScript.
type Producer struct{}

// NewProducer returns the TypeScript fact producer.
func NewProducer() Producer { return Producer{} }

// Language identifies the facts this producer supplies.
func (Producer) Language() rule.Language { return rule.LanguageTypeScript }

// Facts analyzes every analyzable .ts/.tsx file among the observed
// files, producing the requested fact classes: imports always,
// declarations (pinned tree-sitter grammar, deterministic) only when
// asked for. Files are analyzed in parallel: the resolver is read-only
// once built and every parse owns a fresh tree-sitter parser.
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
	return fanout.Analyze(files, analyzable, func() fanout.Analyzer {
		ps := newParsers()
		return func(rel string) conformance.LanguageFacts {
			return analyzeFile(ps, res, root, rel, wantDeclarations, wantCalls)
		}
	}), nil
}

// analyzable selects the files this producer owns: the target
// vocabulary assigns TypeScript .ts and .tsx; .d.ts declaration files
// carry no runtime imports and are skipped, matching the legacy
// analyzer.
func analyzable(rel string) bool {
	if strings.HasSuffix(rel, ".d.ts") {
		return false
	}
	return rule.LanguageOf(rel) == rule.LanguageTypeScript
}

func analyzeFile(ps *parsers, res *resolver, root, rel string, wantDeclarations, wantCalls bool) conformance.LanguageFacts {
	facts := conformance.LanguageFacts{Language: rule.LanguageTypeScript, ImportsAvailable: true}
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		facts.ParseFailure = fmt.Sprintf("read: %v", err)
		return facts
	}
	dir := path.Dir(rel)
	for _, ri := range extract(string(src)) {
		imp := conformance.Import{Path: ri.spec, Line: ri.line}
		imp.Class, imp.TargetDir, imp.TargetFile = res.classify(dir, ri.spec)
		facts.Imports = append(facts.Imports, imp)
	}
	if !wantDeclarations && !wantCalls {
		return facts
	}
	// A strict-parse failure yields honest fact absence without
	// poisoning the masking-scanner import view.
	df := extractDeclarations(ps, rel, src)
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
