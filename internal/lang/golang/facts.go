package golang

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"

	"github.com/wixregiga/arclint/internal/lang"
)

// Facts extracts declaration facts from one Go source file. go/parser is
// exact: there is no heuristic tier here.
func Facts(path string, src []byte) *lang.FileFacts {
	out := &lang.FileFacts{Path: path}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		out.ParseError = fmt.Sprintf("parse: %v", err)
		return out
	}
	out.Package = file.Name.Name

	span := func(n ast.Node) (int, int) {
		return fset.Position(n.Pos()).Line, fset.Position(n.End()).Line
	}
	add := func(kind, name, owner string, n ast.Node) {
		start, end := span(n)
		out.Decls = append(out.Decls, lang.Decl{
			Kind: kind, Name: name, Owner: owner,
			Exported: ast.IsExported(name), StartLine: start, EndLine: end,
		})
	}

	fieldNames := func(f *ast.Field) []string {
		var names []string
		for _, n := range f.Names {
			names = append(names, n.Name)
		}
		if len(names) == 0 { // embedded
			if name := embeddedName(f.Type); name != "" {
				names = append(names, name)
			}
		}
		return names
	}

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					switch t := s.Type.(type) {
					case *ast.StructType:
						add("struct", s.Name.Name, "", s)
						for _, f := range t.Fields.List {
							for _, name := range fieldNames(f) {
								add("field", name, s.Name.Name, f)
							}
						}
					case *ast.InterfaceType:
						add("interface", s.Name.Name, "", s)
						for _, m := range t.Methods.List {
							for _, name := range m.Names {
								add("method", name.Name, s.Name.Name, m)
							}
						}
					default:
						add("type", s.Name.Name, "", s)
					}
				case *ast.ValueSpec:
					kind := "var"
					if d.Tok == token.CONST {
						kind = "const"
					}
					for _, n := range s.Names {
						if n.Name != "_" {
							add(kind, n.Name, "", s)
						}
					}
				}
			}
		case *ast.FuncDecl:
			if d.Recv != nil && len(d.Recv.List) == 1 {
				add("method", d.Name.Name, embeddedName(d.Recv.List[0].Type), d)
			} else {
				add("func", d.Name.Name, "", d)
			}
		}
	}
	return out
}

// embeddedName resolves the base type identifier of an embedded field or
// receiver: T, *T, pkg.T, generic T[...].
func embeddedName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return embeddedName(t.X)
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.IndexExpr:
		return embeddedName(t.X)
	case *ast.IndexListExpr:
		return embeddedName(t.X)
	}
	return ""
}
