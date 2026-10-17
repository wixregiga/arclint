// Package typescript produces normalized Language Facts for
// TypeScript: lexer-grade import extraction with documented
// false-negative classes, an embedded Node builtin table,
// manifest-based (package.json) external classification, and the
// project's tsconfig paths and baseUrl aliases. The
// extraction and resolution mechanics are the proven legacy jsts
// analyzer's; the producer claims only the files the target
// vocabulary assigns TypeScript (.ts, .tsx, minus .d.ts
// declaration files), while relative-specifier resolution still
// probes the full JS/TS extension set.
package typescript

//go:generate go run ../../../../tools/gennodestdlib -out stdlib_gen.go -pkg typescript
//go:generate npm exec --yes --package=typescript@5.9.3 --package=@types/node@24.10.1 -- node ../../../../tools/tsoracle/generate_builtins.cjs builtin_declarations_gen.go
//go:generate gofmt -w builtin_declarations_gen.go

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

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
// asked for. Files are read and parsed in parallel, every parse owning
// a fresh tree-sitter parser; imports are classified afterwards,
// because which configuration owns a file depends on what the other
// files import.
func (Producer) Facts(root string, files []conformance.ObservedFile, requested []rule.Fact) (map[string]conformance.LanguageFacts, error) {
	wantDeclarations := false
	wantCalls := false
	wantResolvedCalls, wantFieldTypes, wantImplementations := false, false, false
	for _, f := range requested {
		if f == rule.FactDeclarations {
			wantDeclarations = true
		}
		if f == rule.FactResolvedCalls {
			wantResolvedCalls, wantCalls = true, true
		}
		if f == rule.FactFieldTypes {
			wantFieldTypes, wantDeclarations = true, true
		}
		if f == rule.FactCalls {
			wantCalls = true
		}
		if f == rule.FactImplementations {
			wantImplementations, wantDeclarations = true, true
		}
	}
	facts := fanout.Analyze(files, analyzable, func() fanout.Analyzer {
		ps := newParsers()
		return func(rel string) conformance.LanguageFacts {
			return analyzeFile(ps, root, rel, wantDeclarations, wantCalls)
		}
	})
	r := newResolver(root, files)
	r.classifyImports(facts)
	if wantResolvedCalls || wantFieldTypes || wantImplementations {
		resolveFacts(root, r, facts, wantResolvedCalls, wantFieldTypes, wantImplementations)
	}
	return facts, nil
}

// classifyImports classifies every extracted import through the
// configuration that owns its file. Ownership is decided first, in one
// pass; the resolver is read-only afterwards, so files are classified
// in parallel.
func (r *resolver) classifyImports(facts map[string]conformance.LanguageFacts) {
	specs := make(map[string][]string, len(facts))
	rels := make([]string, 0, len(facts))
	for rel, fa := range facts {
		s := make([]string, len(fa.Imports))
		for i, imp := range fa.Imports {
			s[i] = imp.Path
		}
		specs[rel] = s
		rels = append(rels, rel)
	}
	r.own(specs)
	var next atomic.Int64
	var wg sync.WaitGroup
	for range min(runtime.GOMAXPROCS(0), len(rels)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := int(next.Add(1)) - 1; i < len(rels); i = int(next.Add(1)) - 1 {
				rel := rels[i]
				p, dir, imports := r.owners[rel], path.Dir(rel), facts[rel].Imports
				for j := range imports {
					imp := &imports[j]
					imp.Class, imp.TargetDir, imp.TargetFile = r.classify(p, dir, imp.Path)
				}
			}
		}()
	}
	wg.Wait()
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

// analyzeFile reads one file's imports, still unclassified, and the
// declarations and calls asked for.
func analyzeFile(ps *parsers, root, rel string, wantDeclarations, wantCalls bool) conformance.LanguageFacts {
	facts := conformance.LanguageFacts{Language: rule.LanguageTypeScript, ImportsAvailable: true}
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		facts.ParseFailure = fmt.Sprintf("read: %v", err)
		return facts
	}
	for _, ri := range extract(string(src)) {
		facts.Imports = append(facts.Imports, conformance.Import{Path: ri.spec, Line: ri.line})
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
