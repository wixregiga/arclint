package golang

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// goModule is one go.mod found among the observed files.
type goModule struct {
	path string
	// dir is repo-relative, "" for the repository root. The nearest
	// enclosing module owns a file.
	dir         string
	requires    map[string]bool
	replaces    []replaceRule
	inWorkspace bool
}

// replaceRule is one replace directive from a go.mod or the root
// go.work. A replacement without a version is a filesystem path.
type replaceRule struct {
	oldPath string
	local   bool
	// repoDir is the repo-relative target directory when local and
	// inside the repository.
	repoDir string
	inRepo  bool
}

// resolver classifies import paths for files of one repository.
// A broken nested go.mod is skipped rather than fatal: its files then
// classify unknown, which the unknown-import policy surfaces instead
// of killing the whole check.
type resolver struct {
	// modules sorted by directory depth, deepest first, for owner
	// lookup.
	modules      []*goModule
	workReplaces []replaceRule
	hasWork      bool
	root         string
}

func newResolver(root string, files []conformance.ObservedFile) *resolver {
	r := &resolver{root: root}
	for _, f := range files {
		if path.Base(f.Path) != "go.mod" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path)))
		if err != nil {
			continue
		}
		mf, err := modfile.Parse(f.Path, data, nil)
		if err != nil || mf.Module == nil || mf.Module.Mod.Path == "" {
			continue
		}
		dir := path.Dir(f.Path)
		if dir == "." {
			dir = ""
		}
		m := &goModule{path: mf.Module.Mod.Path, dir: dir, requires: map[string]bool{}}
		for _, req := range mf.Require {
			m.requires[req.Mod.Path] = true
		}
		for _, rep := range mf.Replace {
			m.replaces = append(m.replaces, r.newReplace(dir, rep.Old.Path, rep.New.Path, rep.New.Version))
		}
		r.modules = append(r.modules, m)
	}
	sort.Slice(r.modules, func(i, j int) bool {
		di, dj := r.modules[i].dir, r.modules[j].dir
		if len(di) != len(dj) {
			return len(di) > len(dj)
		}
		return di < dj
	})

	r.loadWorkspace(root)
	return r
}

// loadWorkspace activates workspace resolution when a parsable go.work
// sits at the repository root: imports of any member module then
// classify internal, matching the go tool in workspace mode.
func (r *resolver) loadWorkspace(root string) {
	data, err := os.ReadFile(filepath.Join(root, "go.work"))
	if err != nil {
		return
	}
	wf, err := modfile.ParseWork("go.work", data, nil)
	if err != nil {
		return
	}
	r.hasWork = true
	useDirs := map[string]bool{}
	for _, u := range wf.Use {
		d := strings.TrimPrefix(path.Clean(filepath.ToSlash(u.Path)), "./")
		if d == "." {
			d = ""
		}
		useDirs[d] = true
	}
	for _, m := range r.modules {
		if useDirs[m.dir] {
			m.inWorkspace = true
		}
	}
	for _, rep := range wf.Replace {
		r.workReplaces = append(r.workReplaces, r.newReplace("", rep.Old.Path, rep.New.Path, rep.New.Version))
	}
}

func (r *resolver) newReplace(baseDir, oldPath, newPath, newVersion string) replaceRule {
	rep := replaceRule{oldPath: oldPath}
	if newVersion != "" {
		return rep
	}
	rep.local = true
	var abs string
	if filepath.IsAbs(newPath) {
		abs = filepath.Clean(newPath)
	} else {
		abs = filepath.Join(r.root, filepath.FromSlash(baseDir), filepath.FromSlash(newPath))
	}
	rel, err := filepath.Rel(r.root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return rep
	}
	rep.inRepo = true
	rep.repoDir = filepath.ToSlash(rel)
	if rep.repoDir == "." {
		rep.repoDir = ""
	}
	return rep
}

// ModuleInfo is one Go module found among the observed files, exposed
// for the toolchain oracle that proves classification against
// `go list` per module.
type ModuleInfo struct {
	Path string
	// Dir is repo-relative, "" for the repository root.
	Dir string
}

// Modules enumerates the Go modules among the observed files, sorted
// by directory.
func Modules(root string, files []conformance.ObservedFile) ([]ModuleInfo, error) {
	r := newResolver(root, files)
	out := make([]ModuleInfo, 0, len(r.modules))
	for _, m := range r.modules {
		out = append(out, ModuleInfo{Path: m.path, Dir: m.dir})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dir < out[j].Dir })
	return out, nil
}

// ownerOf returns the nearest enclosing module of a repo-relative
// directory ("." or "" for root), or nil.
func (r *resolver) ownerOf(dir string) *goModule {
	if dir == "." {
		dir = ""
	}
	for _, m := range r.modules {
		if m.dir == "" || m.dir == dir || strings.HasPrefix(dir, m.dir+"/") {
			return m
		}
	}
	return nil
}

// pathPrefix reports whether p is prefix itself or below it on a path
// segment boundary.
func pathPrefix(p, prefix string) bool {
	return p == prefix || strings.HasPrefix(p, prefix+"/")
}

// classify resolves one import path for a file owned by module owner
// (nil when the file is outside every module). For internal imports
// resolved into the tree it returns the repo-relative package
// directory ("." = repo root). Longest prefix wins across every
// candidate; at equal length, go.work replaces beat go.mod replaces
// beat module paths.
func (r *resolver) classify(owner *goModule, importPath string) (conformance.ImportClass, string) {
	if importPath == "C" {
		return conformance.ImportCgo, ""
	}
	if _, ok := stdlibPackages[importPath]; ok {
		return conformance.ImportStdlib, ""
	}
	if owner == nil {
		return conformance.ImportUnknown, ""
	}

	type candidate struct {
		prefix  string
		class   conformance.ImportClass
		baseDir string
		inTree  bool
		rank    int
	}
	var candidates []candidate
	if pathPrefix(importPath, owner.path) {
		candidates = append(candidates, candidate{owner.path, conformance.ImportInternal, owner.dir, true, 1})
	}
	if r.hasWork {
		for _, m := range r.modules {
			if m.inWorkspace && pathPrefix(importPath, m.path) {
				candidates = append(candidates, candidate{m.path, conformance.ImportInternal, m.dir, true, 1})
			}
		}
	}
	addReplace := func(rep replaceRule, rank int) {
		if !pathPrefix(importPath, rep.oldPath) {
			return
		}
		if rep.local {
			candidates = append(candidates, candidate{rep.oldPath, conformance.ImportInternal, rep.repoDir, rep.inRepo, rank})
			return
		}
		candidates = append(candidates, candidate{rep.oldPath, conformance.ImportExternal, "", false, rank})
	}
	for _, rep := range owner.replaces {
		addReplace(rep, 2)
	}
	for _, rep := range r.workReplaces {
		addReplace(rep, 3)
	}
	for req := range owner.requires {
		if pathPrefix(importPath, req) {
			candidates = append(candidates, candidate{req, conformance.ImportExternal, "", false, 0})
		}
	}

	if len(candidates) == 0 {
		return conformance.ImportUnknown, ""
	}
	best := candidates[0]
	for _, c := range candidates[1:] {
		if len(c.prefix) > len(best.prefix) ||
			(len(c.prefix) == len(best.prefix) && c.rank > best.rank) {
			best = c
		}
	}
	if best.class != conformance.ImportInternal {
		return best.class, ""
	}
	if !best.inTree {
		return conformance.ImportInternal, ""
	}
	suffix := strings.TrimPrefix(strings.TrimPrefix(importPath, best.prefix), "/")
	dir := best.baseDir
	switch {
	case suffix == "" && dir == "":
		dir = "."
	case dir == "":
		dir = suffix
	case suffix != "":
		dir = dir + "/" + suffix
	}
	return conformance.ImportInternal, dir
}
