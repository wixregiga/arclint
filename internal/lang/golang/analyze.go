package golang

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/wixregiga/arclint/internal/lang"
	"github.com/wixregiga/arclint/internal/tree"
)

// Import is one import declaration occurrence in one file (the shared
// language model; Alias is Go-specific).
type Import = lang.Import

// FileAnalysis is the import extraction result for one Go file.
type FileAnalysis = lang.FileAnalysis

// Analysis is the Go target's view of the repository.
type Analysis struct {
	// Files keys repo-relative paths of every analyzed .go file.
	Files    map[string]*FileAnalysis
	Resolver *Resolver
	Warnings []string
}

// analyzable reports whether the go tool would ever consider the file:
// .go files not under a path segment starting with "." or "_" (the go tool
// ignores such files and directories entirely; build-constrained files ARE
// analyzable — arclint scans them and documents the divergence from a
// GOOS/tag-filtered build).
func analyzable(relPath string) bool {
	if !strings.HasSuffix(relPath, ".go") {
		return false
	}
	for _, seg := range strings.Split(relPath, "/") {
		if strings.HasPrefix(seg, ".") || strings.HasPrefix(seg, "_") {
			return false
		}
	}
	return true
}

// Analyze extracts and classifies imports for every analyzable .go file,
// in parallel, deterministically.
func Analyze(t *tree.Tree) *Analysis {
	res := NewResolver(t)
	a := &Analysis{Files: map[string]*FileAnalysis{}, Resolver: res}
	a.Warnings = append(a.Warnings, res.Warnings...)

	var mu sync.Mutex
	fset := token.NewFileSet()
	g := new(errgroup.Group)
	g.SetLimit(runtime.GOMAXPROCS(0))

	for _, f := range t.Files {
		if !analyzable(f.Path) {
			continue
		}
		g.Go(func() error {
			fa := analyzeFile(fset, res, f)
			mu.Lock()
			a.Files[f.Path] = fa
			if fa.ParseError != "" {
				a.Warnings = append(a.Warnings, fmt.Sprintf("%s: skipped: %s", f.Path, fa.ParseError))
			}
			mu.Unlock()
			return nil
		})
	}
	// The group never returns errors; parse failures are per-file warnings.
	_ = g.Wait()
	sort.Strings(a.Warnings)
	return a
}

func analyzeFile(fset *token.FileSet, res *Resolver, f *tree.File) *FileAnalysis {
	fa := &FileAnalysis{Path: f.Path}
	src, err := os.ReadFile(f.Abs)
	if err != nil {
		fa.ParseError = fmt.Sprintf("read: %v", err)
		return fa
	}
	parsed, err := parser.ParseFile(fset, f.Path, src, parser.ImportsOnly)
	if err != nil {
		fa.ParseError = fmt.Sprintf("parse: %v", err)
		return fa
	}
	owner := res.OwnerOf(f.Dir())
	for _, spec := range parsed.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		imp := Import{Path: p, Line: fset.Position(spec.Path.Pos()).Line}
		if spec.Name != nil {
			imp.Alias = spec.Name.Name
		}
		imp.Class, imp.TargetDir = res.Classify(owner, p)
		if imp.Class == ClassCgo {
			fa.HasCgo = true
		}
		fa.Imports = append(fa.Imports, imp)
	}
	return fa
}
