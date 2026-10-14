package golang

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/mod/module"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// stubs are the packages the repository imports that arclint does not
// type-check: the standard library and the dependencies, known by
// their package name and by the type names the repository writes from
// them, and the imports that reach no package arclint can read, known
// the same way with the reason they stay unknown.
type stubs struct {
	root  string
	res   *resolver
	names map[string]string
	// typeNames maps an import path to the type names the repository
	// writes from it and the number of type arguments each takes.
	typeNames map[string]map[string]int
	unknown   map[string]string
	pkgs      map[string]*types.Package
}

func newStubs(root string, res *resolver) *stubs {
	return &stubs{
		root:      root,
		res:       res,
		names:     map[string]string{},
		typeNames: map[string]map[string]int{},
		unknown:   map[string]string{},
		pkgs:      map[string]*types.Package{},
	}
}

// require records an import outside the repository's type-checked
// packages, classified for the importing module.
func (s *stubs) require(ip string, owner *goModule, class conformance.ImportClass, dir string) {
	if ip == "C" {
		return
	}
	if _, ok := s.names[ip]; ok {
		return
	}
	switch class {
	case conformance.ImportStdlib:
		s.names[ip] = assumedName(ip)
	case conformance.ImportExternal:
		s.names[ip] = s.dependencyName(owner, ip)
	case conformance.ImportInternal:
		s.names[ip] = assumedName(ip)
		if dir == "" {
			s.unknown[ip] = ip + " is replaced by a directory outside the repository"
		} else {
			s.unknown[ip] = ip + " names a directory with no Go package arclint type-checks"
		}
	default:
		s.names[ip] = assumedName(ip)
		s.unknown[ip] = ip + " is not provided by any module the go.mod requires"
	}
}

// requireUnknown records an import of a repository package that is not
// type-checked, with the reason.
func (s *stubs) requireUnknown(ip, reason string) {
	if _, ok := s.names[ip]; !ok {
		s.names[ip] = assumedName(ip)
	}
	s.unknown[ip] = ip + ": " + reason
}

// dependencyName reads a dependency's package name from its vendored
// copy or its module cache directory, the package clause only; when
// neither holds it, the name its import path suggests.
func (s *stubs) dependencyName(owner *goModule, ip string) string {
	if dir := s.dependencyDir(owner, ip); dir != "" {
		if name := packageClause(dir); name != "" {
			return name
		}
	}
	return assumedName(ip)
}

func (s *stubs) dependencyDir(owner *goModule, ip string) string {
	modRoot := filepath.Join(s.root, filepath.FromSlash(owner.dir))
	if _, err := os.Stat(filepath.Join(modRoot, "vendor", "modules.txt")); err == nil {
		return filepath.Join(modRoot, "vendor", filepath.FromSlash(ip))
	}
	best := ""
	for req := range owner.requires {
		if pathPrefix(ip, req) && len(req) > len(best) {
			best = req
		}
	}
	if best == "" {
		return ""
	}
	for _, rep := range append(append([]replaceRule(nil), owner.replaces...), s.res.workReplaces...) {
		if pathPrefix(ip, rep.oldPath) {
			return ""
		}
	}
	escPath, err := module.EscapePath(best)
	if err != nil {
		return ""
	}
	escVersion, err := module.EscapeVersion(owner.requires[best])
	if err != nil {
		return ""
	}
	sub := strings.TrimPrefix(strings.TrimPrefix(ip, best), "/")
	return filepath.Join(moduleCache(), filepath.FromSlash(escPath+"@"+escVersion), filepath.FromSlash(sub))
}

// moduleCache is where the go command keeps downloaded modules.
func moduleCache() string {
	if dir := os.Getenv("GOMODCACHE"); dir != "" {
		return dir
	}
	gopath := filepath.SplitList(build.Default.GOPATH)
	if len(gopath) == 0 {
		return ""
	}
	return filepath.Join(gopath[0], "pkg", "mod")
}

// packageClause is the package name of the first file in dir the go
// command builds for this platform.
func packageClause(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		if ok, err := build.Default.MatchFile(dir, name); err != nil || !ok {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, parser.PackageClauseOnly)
		if err == nil {
			return f.Name.Name
		}
	}
	return ""
}

// assumedName is the package name an import path suggests: its last
// element, the one before a major version suffix, without a go-
// prefix and cut at the first character an identifier cannot hold.
func assumedName(ip string) string {
	base := path.Base(ip)
	if strings.HasPrefix(base, "v") {
		if _, err := strconv.Atoi(base[1:]); err == nil && path.Dir(ip) != "." {
			base = path.Base(path.Dir(ip))
		}
	}
	base = strings.TrimPrefix(base, "go-")
	if i := strings.IndexFunc(base, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' }); i >= 0 {
		base = base[:i]
	}
	return base
}

// collectUses records the type names one file writes from the stubbed
// packages it imports, with their number of type arguments.
func (s *stubs) collectUses(f *ast.File) {
	local := map[string]string{}
	for _, spec := range f.Imports {
		ip, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name, ok := s.names[ip]
		if !ok {
			continue
		}
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name != "_" && name != "." {
			local[name] = ip
		}
	}
	if len(local) == 0 {
		return
	}
	var typeExpr func(e ast.Expr, arity int)
	typeExpr = func(e ast.Expr, arity int) {
		switch x := e.(type) {
		case *ast.SelectorExpr:
			id, ok := x.X.(*ast.Ident)
			if !ok {
				return
			}
			ip, ok := local[id.Name]
			if !ok {
				return
			}
			names := s.typeNames[ip]
			if names == nil {
				names = map[string]int{}
				s.typeNames[ip] = names
			}
			if names[x.Sel.Name] < arity {
				names[x.Sel.Name] = arity
			} else if _, seen := names[x.Sel.Name]; !seen {
				names[x.Sel.Name] = arity
			}
		case *ast.StarExpr:
			typeExpr(x.X, 0)
		case *ast.ParenExpr:
			typeExpr(x.X, arity)
		case *ast.ArrayType:
			typeExpr(x.Elt, 0)
		case *ast.MapType:
			typeExpr(x.Key, 0)
			typeExpr(x.Value, 0)
		case *ast.ChanType:
			typeExpr(x.Value, 0)
		case *ast.Ellipsis:
			typeExpr(x.Elt, 0)
		case *ast.IndexExpr:
			typeExpr(x.X, 1)
			typeExpr(x.Index, 0)
		case *ast.IndexListExpr:
			typeExpr(x.X, len(x.Indices))
			for _, idx := range x.Indices {
				typeExpr(idx, 0)
			}
		case *ast.UnaryExpr:
			typeExpr(x.X, 0)
		case *ast.BinaryExpr:
			typeExpr(x.X, 0)
			typeExpr(x.Y, 0)
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Field:
			typeExpr(x.Type, 0)
		case *ast.ValueSpec:
			if x.Type != nil {
				typeExpr(x.Type, 0)
			}
		case *ast.TypeSpec:
			typeExpr(x.Type, 0)
		case *ast.CompositeLit:
			if x.Type != nil {
				typeExpr(x.Type, 0)
			}
		case *ast.TypeAssertExpr:
			if x.Type != nil {
				typeExpr(x.Type, 0)
			}
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok && (id.Name == "new" || id.Name == "make") && len(x.Args) > 0 {
				typeExpr(x.Args[0], 0)
			}
		}
		return true
	})
}

// build creates one complete package per stubbed import: a defined
// type for every type name the repository writes from it, generic when
// the repository instantiates it, with no fields and no methods.
func (s *stubs) build() {
	anyType := types.Universe.Lookup("any").Type()
	for ip, name := range s.names {
		pkg := types.NewPackage(ip, name)
		for typeName, arity := range s.typeNames[ip] {
			tn := types.NewTypeName(token.NoPos, pkg, typeName, nil)
			named := types.NewNamed(tn, types.NewStruct(nil, nil), nil)
			if arity > 0 {
				params := make([]*types.TypeParam, arity)
				for i := range params {
					params[i] = types.NewTypeParam(types.NewTypeName(token.NoPos, pkg, "T"+strconv.Itoa(i), nil), anyType)
				}
				named.SetTypeParams(params)
			}
			pkg.Scope().Insert(tn)
		}
		pkg.MarkComplete()
		s.pkgs[ip] = pkg
	}
}
