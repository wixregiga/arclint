// Package golang produces normalized Language Facts for Go: exact
// per-file import extraction (go/parser) and classification following
// the Go toolchain's resolution semantics — embedded `go list std`
// table, module-path ownership, replace directives, go.work
// membership, and require coverage, longest prefix winning.
package golang

//go:generate go run ../../../../tools/genstdlib -out stdlib_gen.go -pkg golang

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// Producer implements the observation FactProducer seam for Go.
type Producer struct{}

// NewProducer returns the Go fact producer.
func NewProducer() Producer { return Producer{} }

// Language identifies the facts this producer supplies.
func (Producer) Language() rule.Language { return rule.LanguageGo }

// Facts analyzes every analyzable .go file among the observed files,
// producing the requested fact classes: imports always, declarations
// only when asked for — observation costs follow what enforcement
// declares.
func (Producer) Facts(root string, files []conformance.ObservedFile, requested []rule.Fact) (map[string]conformance.LanguageFacts, error) {
	res := newResolver(root, files)
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
	out := map[string]conformance.LanguageFacts{}
	fset := token.NewFileSet()
	for _, f := range files {
		if !analyzable(f.Path) {
			continue
		}
		out[f.Path] = analyzeFile(fset, res, root, f.Path, wantDeclarations, wantCalls)
	}
	return out, nil
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

func analyzeFile(fset *token.FileSet, res *resolver, root, rel string, wantDeclarations, wantCalls bool) conformance.LanguageFacts {
	facts := conformance.LanguageFacts{Language: rule.LanguageGo, ImportsAvailable: true}
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		facts.ParseFailure = fmt.Sprintf("read: %v", err)
		return facts
	}
	mode := parser.ImportsOnly
	if wantDeclarations || wantCalls {
		mode = parser.SkipObjectResolution
	}
	parsed, err := parser.ParseFile(fset, rel, src, mode)
	if err != nil {
		facts.ParseFailure = fmt.Sprintf("parse: %v", err)
		return facts
	}
	facts.Package = parsed.Name.Name
	if wantDeclarations {
		facts.DeclarationsAvailable = true
		facts.Declarations = extractDeclarations(fset, parsed)
	}
	if wantCalls {
		facts.CallsAvailable = true
		facts.Calls = extractCalls(fset, parsed)
	}
	owner := res.ownerOf(path.Dir(rel))
	for _, spec := range parsed.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		imp := conformance.Import{Path: p, Line: fset.Position(spec.Path.Pos()).Line}
		imp.Class, imp.TargetDir = res.classify(owner, p)
		facts.Imports = append(facts.Imports, imp)
	}
	return facts
}
