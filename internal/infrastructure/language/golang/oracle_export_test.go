package golang

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// ToolchainFacts produces the facts with the Go toolchain's own type
// check (go/packages over `go list -export`), dependencies typed from
// export data. It is the oracle the source type check is proved
// against: the producer's answers must never contradict it.
func ToolchainFacts(root string, files []conformance.ObservedFile, requested []rule.Fact) (map[string]conformance.LanguageFacts, error) {
	return produce(root, files, requested, toolchainTypeCheck)
}

// OracleUnchecked opens the reason of every reference in a file the
// toolchain did not type-check without errors.
const OracleUnchecked = "the Go toolchain did not type-check"

const oracleLoadMode = packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
	packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedModule

// toolchainTypeCheck loads every module of the repository through the
// Go toolchain, offline: the module proxy off, go.mod read-only or
// vendored, the toolchain never switching itself.
func toolchainTypeCheck(root string, files []conformance.ObservedFile, res *resolver) *typeCheck {
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
	type group struct {
		dir       string
		patterns  []string
		workspace bool
	}
	var groups []group
	work := group{workspace: true}
	for _, m := range res.modules {
		if res.hasWork && m.inWorkspace {
			pattern := "./..."
			if m.dir != "" {
				pattern = "./" + m.dir + "/..."
			}
			work.patterns = append(work.patterns, pattern)
			continue
		}
		groups = append(groups, group{dir: m.dir, patterns: []string{"./..."}})
	}
	if len(work.patterns) > 0 {
		groups = append(groups, work)
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, g := range groups {
		wg.Go(func() {
			dir := filepath.Join(root, filepath.FromSlash(g.dir))
			pkgs, err := packages.Load(&packages.Config{Mode: oracleLoadMode, Dir: dir, Env: oracleEnv(dir, g.workspace)}, g.patterns...)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				return
			}
			for _, pkg := range pkgs {
				if pkg.Types != nil {
					tc.sources[pkg.PkgPath] = true
				}
				if len(pkg.Errors) > 0 || pkg.IllTyped {
					continue
				}
				for _, file := range pkg.Syntax {
					rel := tc.relative(pkg.Fset.File(file.Pos()).Name())
					if !tc.observed[rel] {
						continue
					}
					tc.files[rel] = typedFile{file: file, fset: pkg.Fset, pkg: pkg.Types, info: pkg.TypesInfo}
					collectMethods(file, tc.methods)
				}
			}
		})
	}
	wg.Wait()
	for _, f := range files {
		if _, ok := tc.files[f.Path]; !ok && analyzable(f.Path) {
			tc.reasons[f.Path] = fmt.Sprintf("%s %s", OracleUnchecked, f.Path)
		}
	}
	return tc
}

func oracleEnv(moduleDir string, workspace bool) []string {
	var env []string
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		switch key {
		case "GOFLAGS", "GOPROXY", "GOTOOLCHAIN", "GOWORK":
			continue
		}
		env = append(env, kv)
	}
	flag := "-mod=readonly"
	if _, err := os.Stat(filepath.Join(moduleDir, "vendor", "modules.txt")); err == nil {
		flag = "-mod=vendor"
	}
	env = append(env, "GOFLAGS="+flag, "GOPROXY=off", "GOTOOLCHAIN=local")
	if !workspace {
		env = append(env, "GOWORK=off")
	}
	return env
}
