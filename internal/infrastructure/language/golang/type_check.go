package golang

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// unsafePackage is the import path the type checker supplies itself.
const unsafePackage = "unsafe"

// typedFile is one observed file as arclint type-checked it: its
// syntax tree, the file set that positions it, and the package that
// holds its type information.
type typedFile struct {
	file *ast.File
	fset *token.FileSet
	pkg  *types.Package
	info *types.Info
}

// typeCheck is what arclint resolved for one repository: the observed
// files it type-checked, the packages it checked from repository
// source, the packages outside the repository it knows by name only,
// and, for every observed Go file it did not type-check, why.
type typeCheck struct {
	root  string
	files map[string]typedFile
	// sources holds the import paths type-checked from repository
	// source.
	sources map[string]bool
	// unknown holds, for an import that names no package arclint can
	// read and no package outside the repository, why.
	unknown map[string]string
	// reasons records why an observed Go file has no type information.
	reasons  map[string]string
	observed map[string]bool
	// methods holds the name of every method a type-checked repository
	// type declares, interfaces included.
	methods map[string]bool
}

// sourcePackage is one directory of repository Go source the way the
// go command builds it for this platform: its non-test files that the
// build constraints keep.
type sourcePackage struct {
	dir   string
	path  string
	owner *goModule
	files []string
	syn   []*ast.File
	// imports maps each import path to the repository package it
	// reaches, nil for one outside the repository.
	imports map[string]*sourcePackage
	failure string
	done    chan struct{}
	types   *types.Package
	info    *types.Info
}

// newTypeCheck type-checks the repository's Go packages from source,
// in parallel and in dependency order, the way `go build` sees them
// for this platform. It runs no toolchain and reads nothing outside
// the repository but the package clause of a dependency: a package
// outside the repository is known by its name and the names the
// repository uses from it, so a reference into it is external without
// the toolchain, the module cache, or the network.
func newTypeCheck(root string, files []conformance.ObservedFile, res *resolver) *typeCheck {
	tc := &typeCheck{
		root:     root,
		files:    map[string]typedFile{},
		sources:  map[string]bool{},
		unknown:  map[string]string{},
		reasons:  map[string]string{},
		observed: map[string]bool{},
		methods:  map[string]bool{},
	}
	for _, f := range files {
		tc.observed[f.Path] = true
	}
	fset := token.NewFileSet()
	pkgs := tc.sourcePackages(root, files, res)
	parsePackages(root, fset, pkgs)
	byDir := map[string]*sourcePackage{}
	for _, p := range pkgs {
		if p.failure == "" {
			byDir[p.dir] = p
			tc.sources[p.path] = true
		}
	}
	stubs := newStubs(root, res)
	for _, p := range pkgs {
		if p.failure != "" {
			continue
		}
		p.imports = map[string]*sourcePackage{}
		for _, f := range p.syn {
			for _, spec := range f.Imports {
				ip, err := strconv.Unquote(spec.Path.Value)
				if err != nil || ip == unsafePackage {
					continue
				}
				class, dir := res.classify(p.owner, ip)
				if dep, ok := byDir[dir]; ok && class == conformance.ImportInternal {
					p.imports[ip] = dep
					continue
				}
				stubs.require(ip, p.owner, class, dir)
			}
		}
	}
	markCycles(pkgs)
	for _, p := range pkgs {
		if p.failure != "" {
			delete(tc.sources, p.path)
		}
	}
	for _, p := range pkgs {
		if p.failure != "" {
			continue
		}
		for ip, dep := range p.imports {
			if dep.failure != "" {
				p.imports[ip] = nil
				stubs.requireUnknown(ip, dep.failure)
			}
		}
	}
	for _, p := range pkgs {
		for _, f := range p.syn {
			stubs.collectUses(f)
		}
	}
	stubs.build()
	for ip, reason := range stubs.unknown {
		tc.unknown[ip] = reason
	}
	checkPackages(fset, pkgs, stubs)
	for _, p := range pkgs {
		if p.failure != "" {
			for _, rel := range p.files {
				tc.reasons[rel] = p.failure
			}
			continue
		}
		for i, rel := range p.files {
			tc.files[rel] = typedFile{file: p.syn[i], fset: fset, pkg: p.types, info: p.info}
		}
		for _, f := range p.syn {
			collectMethods(f, tc.methods)
		}
	}
	return tc
}

// sourcePackages groups the observed Go files into the packages the go
// command would build, recording why every other file is left out.
func (tc *typeCheck) sourcePackages(root string, files []conformance.ObservedFile, res *resolver) []*sourcePackage {
	ctxt := build.Default
	byDir := map[string]*sourcePackage{}
	for _, f := range files {
		rel := f.Path
		if !analyzable(rel) || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		dir := path.Dir(rel)
		owner := res.ownerOf(dir)
		if owner == nil {
			tc.reasons[rel] = "the file belongs to no Go module"
			continue
		}
		sub := strings.TrimPrefix(strings.TrimPrefix(dir, owner.dir), "/")
		if dir == "." || dir == owner.dir {
			sub = ""
		}
		if reason := outsidePackages(sub); reason != "" {
			tc.reasons[rel] = reason
			continue
		}
		abs := filepath.Join(root, filepath.FromSlash(dir))
		ok, err := ctxt.MatchFile(abs, path.Base(rel))
		if err != nil {
			tc.reasons[rel] = "the file's build constraints could not be read: " + err.Error()
			continue
		}
		if !ok {
			tc.reasons[rel] = fmt.Sprintf("the go command does not build this file for %s/%s: its build constraints exclude it", ctxt.GOOS, ctxt.GOARCH)
			continue
		}
		p, found := byDir[dir]
		if !found {
			ip := owner.path
			if sub != "" {
				ip += "/" + sub
			}
			p = &sourcePackage{dir: dir, path: ip, owner: owner, done: make(chan struct{})}
			byDir[dir] = p
		}
		p.files = append(p.files, rel)
	}
	pkgs := make([]*sourcePackage, 0, len(byDir))
	for _, p := range byDir {
		sort.Strings(p.files)
		pkgs = append(pkgs, p)
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].dir < pkgs[j].dir })
	return pkgs
}

// outsidePackages says why a directory of a module holds no package of
// the module, the directories `./...` leaves out.
func outsidePackages(sub string) string {
	for _, seg := range strings.Split(sub, "/") {
		switch seg {
		case "testdata":
			return "testdata is not part of the module's packages"
		case "vendor":
			return "vendored source is read as the dependency it copies"
		}
	}
	return ""
}

// parsePackages parses every file of every package in parallel; a
// directory whose files name two packages, or a file that does not
// parse, fails its package.
func parsePackages(root string, fset *token.FileSet, pkgs []*sourcePackage) {
	type job struct {
		p *sourcePackage
		i int
	}
	jobs := make(chan job)
	errs := make([][]error, len(pkgs))
	index := map[*sourcePackage]int{}
	for i, p := range pkgs {
		p.syn = make([]*ast.File, len(p.files))
		errs[i] = make([]error, len(p.files))
		index[p] = i
	}
	var wg sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		wg.Go(func() {
			for j := range jobs {
				rel := j.p.files[j.i]
				f, err := parser.ParseFile(fset, rel, readSource(root, rel), parser.SkipObjectResolution)
				j.p.syn[j.i] = f
				errs[index[j.p]][j.i] = err
			}
		})
	}
	for _, p := range pkgs {
		for i := range p.files {
			jobs <- job{p, i}
		}
	}
	close(jobs)
	wg.Wait()
	for i, p := range pkgs {
		names := map[string]bool{}
		for k, f := range p.syn {
			if errs[i][k] != nil || f == nil {
				p.failure = fmt.Sprintf("%s does not parse, so its package is not type-checked", path.Base(p.files[k]))
				break
			}
			names[f.Name.Name] = true
		}
		if p.failure == "" && len(names) > 1 {
			list := make([]string, 0, len(names))
			for n := range names {
				list = append(list, n)
			}
			sort.Strings(list)
			p.failure = "the directory holds the packages " + strings.Join(list, " and ") + ", so it is not type-checked"
		}
	}
}

// readSource reads one repository file; a file that cannot be read
// parses as empty input and fails its package.
func readSource(root, rel string) []byte {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return []byte{}
	}
	return data
}

// markCycles fails every package on an import cycle; the go command
// refuses to build one.
func markCycles(pkgs []*sourcePackage) {
	const (
		unvisited = iota
		visiting
		visited
	)
	state := map[*sourcePackage]int{}
	var stack []*sourcePackage
	var visit func(p *sourcePackage)
	visit = func(p *sourcePackage) {
		state[p] = visiting
		stack = append(stack, p)
		for _, dep := range p.imports {
			if dep == nil {
				continue
			}
			switch state[dep] {
			case unvisited:
				visit(dep)
			case visiting:
				for i := len(stack) - 1; i >= 0; i-- {
					stack[i].failure = "the package is on an import cycle, so it is not type-checked"
					if stack[i] == dep {
						break
					}
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[p] = visited
	}
	for _, p := range pkgs {
		if p.failure == "" && state[p] == unvisited {
			visit(p)
		}
	}
}

// checkPackages type-checks every package once its imports are
// checked, as many at a time as the machine runs threads.
func checkPackages(fset *token.FileSet, pkgs []*sourcePackage, stubs *stubs) {
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	sizes := types.SizesFor("gc", build.Default.GOARCH)
	var wg sync.WaitGroup
	for _, p := range pkgs {
		if p.failure != "" {
			close(p.done)
			continue
		}
		wg.Go(func() {
			defer close(p.done)
			for _, dep := range p.imports {
				if dep != nil {
					<-dep.done
				}
			}
			sem <- struct{}{}
			defer func() { <-sem }()
			conf := types.Config{
				Importer:    importerFunc(func(ip string) (*types.Package, error) { return p.importPackage(ip, stubs) }),
				Error:       func(error) {},
				FakeImportC: true,
				Sizes:       sizes,
			}
			p.info = &types.Info{
				Types:      map[ast.Expr]types.TypeAndValue{},
				Defs:       map[*ast.Ident]types.Object{},
				Uses:       map[*ast.Ident]types.Object{},
				Selections: map[*ast.SelectorExpr]*types.Selection{},
			}
			p.types, _ = conf.Check(p.path, fset, p.syn, p.info)
			if p.types != nil {
				settle(p.types)
			}
		})
	}
	wg.Wait()
}

// settle completes every type a checked package exposes before the
// packages importing it are checked. go/types fills an instantiated
// type in lazily, under a lock not every reader takes: the race
// detector caught go/packages, which checks packages in parallel the
// same way, reading a shared instance while another check filled it.
// Filled in before any importer starts, a type is only ever read.
func settle(pkg *types.Package) {
	seen := map[types.Type]bool{}
	var walk func(t types.Type)
	walkVars := func(tuple *types.Tuple) {
		for i := range tuple.Len() {
			walk(tuple.At(i).Type())
		}
	}
	walk = func(t types.Type) {
		if t == nil || seen[t] {
			return
		}
		seen[t] = true
		switch x := t.(type) {
		case *types.Named:
			walk(x.Underlying())
			for i := range x.TypeArgs().Len() {
				walk(x.TypeArgs().At(i))
			}
			for i := range x.NumMethods() {
				walk(x.Method(i).Type())
			}
		case *types.Alias:
			walk(x.Rhs())
			for i := range x.TypeArgs().Len() {
				walk(x.TypeArgs().At(i))
			}
		case *types.Pointer:
			walk(x.Elem())
		case *types.Slice:
			walk(x.Elem())
		case *types.Array:
			walk(x.Elem())
		case *types.Map:
			walk(x.Key())
			walk(x.Elem())
		case *types.Chan:
			walk(x.Elem())
		case *types.Struct:
			for i := range x.NumFields() {
				walk(x.Field(i).Type())
			}
		case *types.Signature:
			if recv := x.Recv(); recv != nil {
				walk(recv.Type())
			}
			walkVars(x.Params())
			walkVars(x.Results())
		case *types.Interface:
			for i := range x.NumEmbeddeds() {
				walk(x.EmbeddedType(i))
			}
			for i := range x.NumExplicitMethods() {
				walk(x.ExplicitMethod(i).Type())
			}
		case *types.Union:
			for i := range x.Len() {
				walk(x.Term(i).Type())
			}
		case *types.TypeParam:
			walk(x.Constraint())
		}
	}
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		walk(scope.Lookup(name).Type())
	}
}

func (p *sourcePackage) importPackage(ip string, stubs *stubs) (*types.Package, error) {
	if ip == unsafePackage {
		return types.Unsafe, nil
	}
	if dep := p.imports[ip]; dep != nil && dep.types != nil {
		return dep.types, nil
	}
	if s, ok := stubs.pkgs[ip]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("package %s is not known", ip)
}

type importerFunc func(path string) (*types.Package, error)

func (f importerFunc) Import(path string) (*types.Package, error) { return f(path) }

// collectMethods records the name of every method a file declares on a
// type, and of every method its interfaces list.
func collectMethods(f *ast.File, into map[string]bool) {
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Recv != nil {
				into[x.Name.Name] = true
			}
		case *ast.InterfaceType:
			for _, m := range x.Methods.List {
				for _, name := range m.Names {
					into[name.Name] = true
				}
			}
		}
		return true
	})
}

// relative maps the file name of a declaration onto the repository:
// arclint parses under repository-relative names.
func (tc *typeCheck) relative(name string) string {
	if !filepath.IsAbs(name) {
		return name
	}
	rel, err := filepath.Rel(tc.root, name)
	if err != nil {
		return name
	}
	return filepath.ToSlash(rel)
}

// unresolvedReason says why an observed Go file has no type
// information.
func (tc *typeCheck) unresolvedReason(rel string) string {
	if strings.HasSuffix(rel, "_test.go") {
		return "test files are not type-checked"
	}
	if reason, ok := tc.reasons[rel]; ok {
		return reason
	}
	return "the file is in no package arclint type-checks"
}
