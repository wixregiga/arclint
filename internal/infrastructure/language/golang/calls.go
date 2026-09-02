package golang

import (
	"go/ast"
	"go/token"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// extractCalls maps one fully parsed file onto the call fact: callee
// identifier, line, enclosing func. Parser-exact; types unresolved.
func extractCalls(fset *token.FileSet, file *ast.File) []conformance.Call {
	var out []conformance.Call
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		walkCalls(fset, fn.Body, fn.Name.Name, &out)
	}
	return out
}

func walkCalls(fset *token.FileSet, n ast.Node, enclosing string, out *[]conformance.Call) {
	ast.Inspect(n, func(node ast.Node) bool {
		if node == nil {
			return false
		}
		switch x := node.(type) {
		case *ast.FuncLit:
			// Nested function: its body uses its own enclosing name
			// when it has none; keep the outer name so calls inside
			// a literal still attach to the declared func.
			walkCalls(fset, x.Body, enclosing, out)
			return false
		case *ast.CallExpr:
			if name := calleeIdent(x.Fun); name != "" {
				*out = append(*out, conformance.Call{
					Callee:    name,
					Line:      fset.Position(x.Pos()).Line,
					Enclosing: enclosing,
				})
			}
		}
		return true
	})
}

func calleeIdent(fun ast.Expr) string {
	switch t := fun.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.IndexExpr:
		return calleeIdent(t.X)
	case *ast.IndexListExpr:
		return calleeIdent(t.X)
	case *ast.ParenExpr:
		return calleeIdent(t.X)
	default:
		return ""
	}
}
