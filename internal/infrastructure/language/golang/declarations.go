package golang

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// extractDeclarations maps one fully parsed file onto the shared
// cross-language declaration vocabulary. Go emits its honest subset:
// struct, interface, type, func, method, field, const, var, parser
// exact, types never resolved.
func extractDeclarations(fset *token.FileSet, file *ast.File) []conformance.Declaration {
	var out []conformance.Declaration
	lines := func(n ast.Node) (int, int) {
		return fset.Position(n.Pos()).Line, fset.Position(n.End()).Line
	}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			start, end := lines(d)
			dec := conformance.Declaration{
				Kind:      "func",
				Name:      d.Name.Name,
				Exported:  ast.IsExported(d.Name.Name),
				StartLine: start,
				EndLine:   end,
				Params:    paramInfos(d.Type.Params),
				Results:   resultTexts(d.Type.Results),
			}
			if d.Recv != nil && len(d.Recv.List) > 0 {
				dec.Kind = "method"
				dec.Owner = receiverName(d.Recv.List[0].Type)
			}
			out = append(out, dec)
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					out = append(out, typeDeclarations(fset, s)...)
				case *ast.ValueSpec:
					kind := "var"
					if d.Tok == token.CONST {
						kind = "const"
					}
					for _, name := range s.Names {
						start, end := lines(s)
						out = append(out, conformance.Declaration{
							Kind:      kind,
							Name:      name.Name,
							Exported:  ast.IsExported(name.Name),
							StartLine: start,
							EndLine:   end,
						})
					}
				}
			}
		}
	}
	return out
}

// typeDeclarations emits the type itself plus its members: struct
// fields (embedded fields keep their type's base name) and interface
// methods with their signatures.
func typeDeclarations(fset *token.FileSet, s *ast.TypeSpec) []conformance.Declaration {
	lines := func(n ast.Node) (int, int) {
		return fset.Position(n.Pos()).Line, fset.Position(n.End()).Line
	}
	start, end := lines(s)
	owner := s.Name.Name
	kind := "type"
	var members []conformance.Declaration
	switch t := s.Type.(type) {
	case *ast.StructType:
		kind = "struct"
		for _, field := range t.Fields.List {
			fs, fe := lines(field)
			if len(field.Names) == 0 {
				name := baseTypeName(field.Type)
				members = append(members, conformance.Declaration{
					Kind: "field", Name: name, Owner: owner,
					Exported: ast.IsExported(name), StartLine: fs, EndLine: fe,
				})
				continue
			}
			for _, name := range field.Names {
				members = append(members, conformance.Declaration{
					Kind: "field", Name: name.Name, Owner: owner,
					Exported: ast.IsExported(name.Name), StartLine: fs, EndLine: fe,
				})
			}
		}
	case *ast.InterfaceType:
		kind = "interface"
		for _, method := range t.Methods.List {
			funcType, ok := method.Type.(*ast.FuncType)
			if !ok || len(method.Names) == 0 {
				continue // embedded interface
			}
			ms, me := lines(method)
			for _, name := range method.Names {
				members = append(members, conformance.Declaration{
					Kind: "method", Name: name.Name, Owner: owner,
					Exported: ast.IsExported(name.Name), StartLine: ms, EndLine: me,
					Params:  paramInfos(funcType.Params),
					Results: resultTexts(funcType.Results),
				})
			}
		}
	}
	out := make([]conformance.Declaration, 1, 1+len(members))
	out[0] = conformance.Declaration{
		Kind: kind, Name: owner,
		Exported: ast.IsExported(owner), StartLine: start, EndLine: end,
	}
	return append(out, members...)
}

func paramInfos(fields *ast.FieldList) []conformance.DeclarationParam {
	if fields == nil {
		return nil
	}
	var out []conformance.DeclarationParam
	for _, field := range fields.List {
		_, variadic := field.Type.(*ast.Ellipsis)
		typeText := types.ExprString(field.Type)
		if len(field.Names) == 0 {
			out = append(out, conformance.DeclarationParam{Type: typeText, Variadic: variadic})
			continue
		}
		for _, name := range field.Names {
			out = append(out, conformance.DeclarationParam{
				Name: name.Name, Type: typeText, Variadic: variadic,
			})
		}
	}
	return out
}

func resultTexts(fields *ast.FieldList) []string {
	if fields == nil {
		return nil
	}
	var out []string
	for _, field := range fields.List {
		text := types.ExprString(field.Type)
		n := len(field.Names)
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			out = append(out, text)
		}
	}
	return out
}

// receiverName unwraps pointers and type parameters to the receiver's
// base type name.
func receiverName(expr ast.Expr) string {
	for {
		switch t := expr.(type) {
		case *ast.StarExpr:
			expr = t.X
		case *ast.IndexExpr:
			expr = t.X
		case *ast.IndexListExpr:
			expr = t.X
		case *ast.Ident:
			return t.Name
		default:
			return types.ExprString(expr)
		}
	}
}

// baseTypeName names an embedded field by its type's base identifier.
func baseTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return baseTypeName(t.X)
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.Ident:
		return t.Name
	default:
		return types.ExprString(expr)
	}
}
