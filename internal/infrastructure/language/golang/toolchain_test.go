// The toolchain validation suite for this adapter: it clones pinned
// SHAs of five real public Go repositories of varied shape and asserts
// the producer's per-file import extraction and
// internal/stdlib/external classification against ground truth from
// `go list -e -deps -test -json ./...` (.Standard, .Module.Path,
// .Module.Dir). Environment-dependent by design (network, the local
// Go toolchain); skipped under -short.
//
// Network-permitted and toolchain-dependent by design; part of the
// normal `go test ./...` run (clones cache under ~/.cache), skipped
// under -short.
//
// Contract per the M1 gate: zero crashes, zero extraction or classification
// mismatches on files go list covers, and the build-constraint divergence
// (files arclint scans that `go list` excludes for the current
// GOOS/GOARCH/tags) asserted as exactly the union of IgnoredGoFiles.
package golang_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	golangfacts "github.com/wixregiga/arclint/internal/infrastructure/language/golang"
	filesystemobservation "github.com/wixregiga/arclint/internal/infrastructure/observation/filesystem"
)

type repoSpec struct {
	name  string
	url   string
	sha   string
	shape string
	// minModules sanity-checks that the clone has the shape the spec
	// claims (workspace members, nested modules).
	minModules int
}

// Pinned default-branch SHAs, resolved 2026-08-10 via git ls-remote.
var repos = []repoSpec{
	{"cobra", "https://github.com/spf13/cobra.git", "adbc8813901bba65827259daa8e22ff94ec1f30e", "single module", 1},
	{"gin", "https://github.com/gin-gonic/gin.git", "34dac209ffb6ef85cc78c5d217bbb7ad001d68fd", "single module", 1},
	{"testfixtures", "https://github.com/go-testfixtures/testfixtures.git", "41332ec739cb1e88d7b7f22ba17a321c6326cd2d", "go.work workspace", 3},
	{"runc", "https://github.com/opencontainers/runc.git", "7495faeac77318158e6d5faece1b0b0d53e6ced4", "vendored, cgo", 1},
	{"otel-go", "https://github.com/open-telemetry/opentelemetry-go.git", "b05b3d3bdd8eb9cabb523ca390f6af8b2cc33b99", "multi-module with sibling replaces", 10},
}

type listModule struct {
	Path    string      `json:"Path"`
	Dir     string      `json:"Dir"`
	Version string      `json:"Version"`
	Replace *listModule `json:"Replace"`
}

type listPkg struct {
	ImportPath     string      `json:"ImportPath"`
	Dir            string      `json:"Dir"`
	Standard       bool        `json:"Standard"`
	ForTest        string      `json:"ForTest"`
	Module         *listModule `json:"Module"`
	GoFiles        []string    `json:"GoFiles"`
	CgoFiles       []string    `json:"CgoFiles"`
	TestGoFiles    []string    `json:"TestGoFiles"`
	XTestGoFiles   []string    `json:"XTestGoFiles"`
	IgnoredGoFiles []string    `json:"IgnoredGoFiles"`
	InvalidGoFiles []string    `json:"InvalidGoFiles"`
	Imports        []string    `json:"Imports"`
	TestImports    []string    `json:"TestImports"`
	XTestImports   []string    `json:"XTestImports"`
	Incomplete     bool        `json:"Incomplete"`
}

func TestToolchainGroundTruth(t *testing.T) {
	if testing.Short() {
		t.Skip("network ground truth over pinned real repositories; run without -short")
	}
	for _, spec := range repos {
		t.Run(spec.name, func(t *testing.T) {
			runOracle(t, spec)
		})
	}
}

func runOracle(t *testing.T, spec repoSpec) {
	repoDir := ensureClone(t, spec)

	source, err := filesystemobservation.NewSource(repoDir)
	if err != nil {
		t.Fatalf("source: %v", err)
	}
	observed, err := source.Observe(nil, rule.Scan{}, nil)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	analyzed, err := golangfacts.NewProducer().Facts(repoDir, observed.Files(), nil)
	if err != nil {
		t.Fatalf("facts: %v", err)
	}

	// Zero crashes, zero unparseable files: these repos are valid Go.
	var parseErrs []string
	for p, fa := range analyzed {
		if fa.ParseFailure != "" {
			parseErrs = append(parseErrs, p+": "+fa.ParseFailure)
		}
	}
	if len(parseErrs) > 0 {
		sort.Strings(parseErrs)
		t.Fatalf("parse errors on a real repo:\n%s", strings.Join(parseErrs, "\n"))
	}

	mods, err := golangfacts.Modules(repoDir, observed.Files())
	if err != nil {
		t.Fatalf("modules: %v", err)
	}
	if len(mods) < spec.minModules {
		t.Fatalf("shape check: found %d modules, want >= %d (%s)", len(mods), spec.minModules, spec.shape)
	}
	ownerOf := func(dir string) *golangfacts.ModuleInfo {
		if dir == "." {
			dir = ""
		}
		var best *golangfacts.ModuleInfo
		for i := range mods {
			m := &mods[i]
			if m.Dir == "" || m.Dir == dir || strings.HasPrefix(dir, m.Dir+"/") {
				if best == nil || len(m.Dir) > len(best.Dir) {
					best = m
				}
			}
		}
		return best
	}
	t.Logf("%s: %d files analyzed, %d modules (%s)", spec.name, len(analyzed), len(mods), spec.shape)

	var (
		mismatches   []string
		divergent    int
		testdataDivs int
		coveredCnt   int
		assertedIm   int
	)
	// Workspace mode rejects -mod=mod; the default (readonly) is correct
	// there. Everywhere else -mod=mod bypasses vendor/ so ground truth
	// always comes from go.mod resolution, matching arclint's rule.
	workspace := false
	if _, err := os.Stat(filepath.Join(repoDir, "go.work")); err == nil {
		workspace = true
	}

	for _, m := range mods {
		runDir := filepath.Join(repoDir, filepath.FromSlash(m.Dir))
		pkgs := goList(t, runDir, workspace)

		// Ground-truth classification facts for every package in this
		// run's closure. Synthetic test variants ("pkg [pkg.test]") are
		// skipped: their base packages carry the same facts under the
		// clean import path.
		gtByImport := map[string]*listPkg{}
		for _, p := range pkgs {
			if p.ForTest == "" && !strings.Contains(p.ImportPath, " [") {
				gtByImport[p.ImportPath] = p
			}
		}

		// Extraction and divergence assert against this module's own
		// packages only: dependency views of sibling modules can carry
		// partial test-file information, and each module gets its own run.
		coveredDirs := map[string]bool{}
		for _, p := range pkgs {
			if p.ForTest != "" || p.Dir == "" || strings.Contains(p.ImportPath, " [") {
				continue
			}
			if p.Module == nil || p.Module.Path != m.Path {
				continue
			}
			rel, ok := relInside(repoDir, p.Dir)
			if !ok || coveredDirs[rel] {
				continue
			}
			// Imported testdata packages: go list covers a testdata
			// directory when a test imports it explicitly (gin's
			// protoexample); arclint excludes testdata/ by Go convention
			// unless scan.include_testdata is set. Documented divergence,
			// asserted by count, not a mismatch.
			if underTestdata(rel) {
				testdataDivs++
				continue
			}
			coveredDirs[rel] = true
			coveredCnt++

			// --- Extraction: per-file union must equal go list's import
			// lists for every file group.
			groups := []struct {
				label   string
				files   []string
				imports []string
			}{
				{"Imports", append(append([]string{}, p.GoFiles...), p.CgoFiles...), p.Imports},
				{"TestImports", p.TestGoFiles, p.TestImports},
				{"XTestImports", p.XTestGoFiles, p.XTestImports},
			}
			for _, g := range groups {
				if len(g.files) == 0 {
					continue
				}
				union := map[string]bool{}
				missing := false
				for _, f := range g.files {
					fp := path.Join(rel, f)
					fa, analyzedFile := analyzed[fp]
					if !analyzedFile {
						mismatches = append(mismatches, fmt.Sprintf("%s: covered by go list (%s) but not analyzed by arclint", fp, g.label))
						missing = true
						continue
					}
					for _, imp := range fa.Imports {
						union[imp.Path] = true
					}
				}
				if missing {
					continue
				}
				want := map[string]bool{}
				for _, imp := range g.imports {
					want[imp] = true
				}
				for imp := range want {
					if !union[imp] {
						mismatches = append(mismatches, fmt.Sprintf("%s (%s): go list has %q, arclint extraction lacks it", rel, g.label, imp))
					}
				}
				for imp := range union {
					if !want[imp] {
						mismatches = append(mismatches, fmt.Sprintf("%s (%s): arclint extracted %q, go list lacks it", rel, g.label, imp))
					}
				}
			}

			// --- Divergence: analyzed files in this dir that go list did
			// not include must be exactly the build-constraint-excluded
			// set (IgnoredGoFiles ∪ InvalidGoFiles).
			listed := map[string]bool{}
			for _, group := range [][]string{p.GoFiles, p.CgoFiles, p.TestGoFiles, p.XTestGoFiles, p.IgnoredGoFiles, p.InvalidGoFiles} {
				for _, f := range group {
					listed[f] = true
				}
			}
			ignored := map[string]bool{}
			for _, f := range append(append([]string{}, p.IgnoredGoFiles...), p.InvalidGoFiles...) {
				ignored[f] = true
			}
			for fp, fa := range analyzed {
				if path.Dir(fp) != rel {
					continue
				}
				base := path.Base(fp)
				if !listed[base] {
					mismatches = append(mismatches, fmt.Sprintf("%s: analyzed by arclint but absent from every go list file group of %s", fp, rel))
				}
				if ignored[base] {
					divergent++
					_ = fa
				}
			}
			for f := range ignored {
				fp := path.Join(rel, f)
				if _, analyzedFile := analyzed[fp]; !analyzedFile {
					mismatches = append(mismatches, fmt.Sprintf("%s: build-constrained (go list ignored) but NOT scanned by arclint — the spec requires scanning", fp))
				}
			}
		}

		// --- Classification: for every import in files owned by this
		// module inside directories this run covers, arclint's class must
		// match the go-list-derived class.
		for fp, fa := range analyzed {
			owner := ownerOf(path.Dir(fp))
			if owner == nil || owner.Path != m.Path || owner.Dir != m.Dir || !coveredDirs[path.Dir(fp)] {
				continue
			}
			for _, imp := range fa.Imports {
				want, why := expectedClass(imp.Path, gtByImport, repoDir)
				if want == "" {
					continue // not in this run's closure (e.g. import only in an ignored file)
				}
				assertedIm++
				if imp.Class != want {
					mismatches = append(mismatches, fmt.Sprintf("%s:%d: import %q classified %s, ground truth %s (%s)",
						fp, imp.Line, imp.Path, imp.Class, want, why))
				}
			}
		}
	}

	sort.Strings(mismatches)
	if len(mismatches) > 0 {
		max := len(mismatches)
		if max > 50 {
			max = 50
		}
		t.Errorf("%d mismatches (showing %d):\n%s", len(mismatches), max, strings.Join(mismatches[:max], "\n"))
	}
	t.Logf("%s: %d packages covered, %d imports classification-asserted, divergences: %d build-constrained files scanned by arclint but excluded by go list, %d imported-testdata packages covered by go list but excluded by arclint",
		spec.name, coveredCnt, assertedIm, divergent, testdataDivs)
}

// underTestdata reports whether a repo-relative dir sits below a testdata
// path segment.
func underTestdata(rel string) bool {
	for _, seg := range strings.Split(rel, "/") {
		if seg == "testdata" {
			return true
		}
	}
	return false
}

// expectedClass maps go list facts onto arclint's classification contract:
// stdlib = .Standard; internal = module directory inside the repository
// (self, workspace member, or replace-to-local); external = any other
// module. "C" is the cgo pseudo-import.
func expectedClass(importPath string, gt map[string]*listPkg, repoDir string) (conformance.ImportClass, string) {
	if importPath == "C" {
		return conformance.ImportCgo, "pseudo-import"
	}
	p := gt[importPath]
	if p == nil {
		return "", "not in go list closure"
	}
	if p.Standard {
		return conformance.ImportStdlib, ".Standard"
	}
	if p.Module == nil {
		return "", "no module info"
	}
	if p.Module.Dir != "" {
		if _, ok := relInside(repoDir, p.Module.Dir); ok {
			return conformance.ImportInternal, ".Module.Dir inside repo"
		}
	}
	if p.Module.Replace != nil && p.Module.Replace.Version == "" {
		// Directory replace resolving outside the repo: internal for
		// boundaries per the resolution spec.
		return conformance.ImportInternal, "replace-to-local outside repo"
	}
	return conformance.ImportExternal, ".Module.Path external"
}

func relInside(root, dir string) (string, bool) {
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func goList(t *testing.T, dir string, workspace bool) []*listPkg {
	t.Helper()
	cmd := exec.Command("go", "list", "-e", "-deps", "-test", "-json", "./...")
	cmd.Dir = dir
	goflags := "GOFLAGS=-mod=mod"
	if workspace {
		goflags = "GOFLAGS="
	}
	cmd.Env = append(os.Environ(),
		goflags,
		"GOTOOLCHAIN=local",
		"CGO_ENABLED=1",
	)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("go list in %s: %v\n%s", dir, err, errb.String())
	}
	var pkgs []*listPkg
	dec := json.NewDecoder(&out)
	for {
		var p listPkg
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("go list json in %s: %v", dir, err)
		}
		pkgs = append(pkgs, &p)
	}
	return pkgs
}

// ensureClone fetches the pinned SHA into a persistent cache directory.
func ensureClone(t *testing.T, spec repoSpec) string {
	t.Helper()
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(cacheRoot, "arclint-oracle", spec.name+"@"+spec.sha[:12])
	if sha, err := gitOutput(dir, "rev-parse", "HEAD"); err == nil && strings.TrimSpace(sha) == spec.sha {
		return dir
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	steps := [][]string{
		{"init", "-q"},
		{"remote", "add", "origin", spec.url},
		{"fetch", "-q", "--depth", "1", "origin", spec.sha},
		{"checkout", "-q", "--detach", "FETCH_HEAD"},
	}
	for _, args := range steps {
		if out, err := gitOutput(dir, args...); err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
		}
	}
	return dir
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	// Hook runners export GIT_DIR/GIT_WORK_TREE; inheriting them would
	// aim every command here at the enclosing repository instead of
	// the oracle cache clone. Scrub the git location variables so dir
	// alone decides the target.
	for _, env := range os.Environ() {
		name, _, _ := strings.Cut(env, "=")
		switch name {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY":
			continue
		}
		cmd.Env = append(cmd.Env, env)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}
