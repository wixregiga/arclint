// Package golang produces normalized Language Facts for Go: exact
// per-file import extraction (go/parser) and classification following
// the Go toolchain's resolution semantics: embedded `go list std`
// table, module-path ownership, replace directives, go.work
// membership, and require coverage, longest prefix winning.
package golang

//go:generate go run ../../../../tools/genstdlib -out stdlib_gen.go -pkg golang

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/infrastructure/language/fanout"
)

// Producer implements the observation FactProducer seam for Go.
type Producer struct{}

// NewProducer returns the Go fact producer.
func NewProducer() Producer { return Producer{} }

// Language identifies the facts this producer supplies.
func (Producer) Language() rule.Language { return rule.LanguageGo }

// Facts analyzes every analyzable .go file among the observed files,
// producing the requested fact classes: imports always, declarations
// only when asked for: observation costs follow what enforcement
// declares. Files are analyzed in parallel; the resolver is read-only
// once built, and each worker parses into its own token.FileSet.
func (Producer) Facts(root string, files []conformance.ObservedFile, requested []rule.Fact) (map[string]conformance.LanguageFacts, error) {
	return produce(root, files, requested, newTypeCheck)
}

// produce analyzes the files with the type check typeChecker builds.
func produce(root string, files []conformance.ObservedFile, requested []rule.Fact, typeChecker func(string, []conformance.ObservedFile, *resolver) *typeCheck) (map[string]conformance.LanguageFacts, error) {
	res := newResolver(root, files)
	want := wanted{}
	for _, f := range requested {
		switch f {
		case rule.FactDeclarations:
			want.declarations = true
		case rule.FactCalls:
			want.calls = true
		case rule.FactResolvedCalls:
			want.resolvedCalls = true
		case rule.FactFieldTypes:
			want.fieldTypes = true
		case rule.FactFileTree, rule.FactImports:
			// Imports are always produced; the file tree is the walk's.
		}
	}
	want.resolvedCalls = want.resolvedCalls && want.calls
	want.fieldTypes = want.fieldTypes && want.declarations
	var tc *typeCheck
	if want.resolvedCalls || want.fieldTypes {
		tc = typeChecker(root, files, res)
	}
	return fanout.Analyze(files, analyzable, func() fanout.Analyzer {
		fset := token.NewFileSet()
		return func(rel string) conformance.LanguageFacts {
			return analyzeFile(fset, res, tc, root, rel, want)
		}
	}), nil
}

// wanted is the fact classes one observation asks for; a resolved
// class is honored only together with the view it resolves.
type wanted struct {
	declarations, calls, resolvedCalls, fieldTypes bool
}

// analyzable reports whether the go tool would ever consider the file:
// .go files not under a path segment starting with "." or "_".
func analyzable(rel string) bool {
	if !strings.HasSuffix(rel, ".go") {
		return false
	}
	for _, seg := range strings.Split(rel, "/") {
		if strings.HasPrefix(seg, ".") || strings.HasPrefix(seg, "_") {
			return false
		}
	}
	return true
}

func analyzeFile(fset *token.FileSet, res *resolver, tc *typeCheck, root, rel string, want wanted) conformance.LanguageFacts {
	facts := conformance.LanguageFacts{Language: rule.LanguageGo, ImportsAvailable: true}
	parsed, fset, resolved, failure := parseFile(fset, tc, root, rel, want)
	if failure != "" {
		facts.ParseFailure = failure
		return facts
	}
	facts.Package = parsed.Name.Name
	if want.declarations {
		facts.DeclarationsAvailable = true
		var holds resolution
		if want.fieldTypes {
			facts.FieldTypesAvailable = true
			holds = resolved
		}
		facts.Declarations = extractDeclarations(fset, parsed, holds)
	}
	if want.calls {
		facts.CallsAvailable = true
		var calls resolution
		if want.resolvedCalls {
			facts.ResolvedCallsAvailable = true
			calls = resolved
		}
		facts.Calls = extractCalls(fset, parsed, calls)
	}
	owner := res.ownerOf(path.Dir(rel))
	for _, spec := range parsed.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		imp := conformance.Import{Path: p, Line: fset.Position(spec.Path.Pos()).Line}
		if spec.Name != nil {
			imp.Alias = spec.Name.Name
		}
		imp.Class, imp.TargetDir = res.classify(owner, p)
		facts.Imports = append(facts.Imports, imp)
	}
	return facts
}

// parseFile returns the file's syntax with the file set that positions
// it and, when resolution was asked for, what resolves its references:
// the toolchain's syntax and type information for a file it
// type-checked, otherwise the worker's own parse with every reference
// undetermined and the reason.
func parseFile(fset *token.FileSet, tc *typeCheck, root, rel string, want wanted) (*ast.File, *token.FileSet, resolution, string) {
	if tc != nil {
		if tf, ok := tc.files[rel]; ok {
			return tf.file, tf.fset, newTypedResolution(tc, tf), ""
		}
	}
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, nil, nil, fmt.Sprintf("read: %v", err)
	}
	mode := parser.ImportsOnly
	if want.declarations || want.calls {
		mode = parser.SkipObjectResolution
	}
	parsed, err := parser.ParseFile(fset, rel, src, mode)
	if err != nil {
		return nil, nil, nil, fmt.Sprintf("parse: %v", err)
	}
	var resolved resolution
	if tc != nil {
		resolved = unresolvedFile{reason: tc.unresolvedReason(rel)}
	}
	return parsed, fset, resolved, ""
}
