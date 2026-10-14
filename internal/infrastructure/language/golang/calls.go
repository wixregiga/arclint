package golang

import (
	"go/ast"
	"go/token"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// extractCalls maps one fully parsed file onto the call fact: callee
// identifier, line, and the enclosing func with the type a method
// belongs to. With a resolution every call also carries the
// declaration it reaches and, for a method call, the type of the value
// it is called on.
func extractCalls(fset *token.FileSet, file *ast.File, res resolution) []conformance.Call {
	var out []conformance.Call
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		owner := ""
		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			owner = receiverName(fn.Recv.List[0].Type)
		}
		walkCalls(fset, fn.Body, enclosingFunc{name: fn.Name.Name, owner: owner}, res, &out)
	}
	return out
}

// enclosingFunc is the declared function or method a call is in.
type enclosingFunc struct {
	name, owner string
}

func walkCalls(fset *token.FileSet, n ast.Node, enclosing enclosingFunc, res resolution, out *[]conformance.Call) {
	ast.Inspect(n, func(node ast.Node) bool {
		if node == nil {
			return false
		}
		switch x := node.(type) {
		case *ast.FuncLit:
			// Nested function: its body uses its own enclosing name
			// when it has none; keep the outer name so calls inside
			// a literal still attach to the declared func.
			walkCalls(fset, x.Body, enclosing, res, out)
			return false
		case *ast.CallExpr:
			if name := calleeIdent(x.Fun); name != "" {
				call := conformance.Call{
					Callee:         name,
					Line:           fset.Position(x.Pos()).Line,
					Enclosing:      enclosing.name,
					EnclosingOwner: enclosing.owner,
				}
				if res != nil {
					call.Target, call.Receiver = res.call(x)
				}
				*out = append(*out, call)
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
