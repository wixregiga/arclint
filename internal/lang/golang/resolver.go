package golang

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/wixregiga/arclint/internal/tree"
)

// Class is the exact classification of one import path. No heuristics:
// stdlib comes from the embedded `go list std` table, internal from module
// path ownership (including replace-to-local and go.work membership),
// external from go.mod require coverage. Anything else is unknown and
// subject to the configured policy.
type Class string

const (
	ClassStdlib   Class = "stdlib"
	ClassInternal Class = "internal"
	ClassExternal Class = "external"
	ClassUnknown  Class = "unknown"
	// ClassCgo is the "C" pseudo-import, which is neither a stdlib package
	// nor a resolvable module path.
	ClassCgo Class = "cgo"
)

// Replace is one replace directive, from a go.mod or the go.work file.
type Replace struct {
	OldPath string
	NewPath string
	// Local is true for filesystem replacements (no version in go.mod).
	Local bool
	// RepoDir is the repo-relative directory when Local and inside the
	// repository, else "".
	RepoDir string
	InRepo  bool
}

// GoModule is one go.mod in the repository.
type GoModule struct {
	Path string
	// Dir is repo-relative ("" for the repository root). The nearest
	// enclosing module owns a file.
	Dir      string
	Requires map[string]bool
	Replaces []Replace
	// InWorkspace is true when a root go.work lists this module.
	InWorkspace bool
}

// Resolver classifies import paths for files of this repository.
type Resolver struct {
	// modules sorted by directory depth, deepest first, for owner lookup.
	modules      []*GoModule
	workReplaces []Replace
	hasWork      bool
	root         string
	Warnings     []string
}

// Modules returns every module found, sorted by directory.
func (r *Resolver) Modules() []*GoModule {
	out := append([]*GoModule(nil), r.modules...)
	sort.Slice(out, func(i, j int) bool { return out[i].Dir < out[j].Dir })
	return out
}

// HasModules reports whether any go.mod was found.
func (r *Resolver) HasModules() bool { return len(r.modules) > 0 }

// IsStdlib reports exact membership in the embedded `go list std` table.
func IsStdlib(importPath string) bool {
	_, ok := stdlibPackages[importPath]
	return ok
}

// NewResolver parses every go.mod in the tree plus a root go.work.
// Parse failures are warnings, never fatal: a broken nested go.mod must not
// crash a whole-repo lint.
func NewResolver(t *tree.Tree) *Resolver {
	r := &Resolver{root: t.Root}

	for _, f := range t.Files {
		if f.Name() != "go.mod" {
			continue
		}
		data, err := os.ReadFile(f.Abs)
		if err != nil {
			r.Warnings = append(r.Warnings, fmt.Sprintf("unreadable %s: %v", f.Path, err))
			continue
		}
		mf, err := modfile.Parse(f.Path, data, nil)
		if err != nil {
			r.Warnings = append(r.Warnings, fmt.Sprintf("unparseable %s: %v", f.Path, err))
			continue
		}
		if mf.Module == nil || mf.Module.Mod.Path == "" {
			r.Warnings = append(r.Warnings, fmt.Sprintf("%s: missing module directive", f.Path))
			continue
		}
		dir := f.Dir()
		if dir == "." {
			dir = ""
		}
		m := &GoModule{Path: mf.Module.Mod.Path, Dir: dir, Requires: map[string]bool{}}
		for _, req := range mf.Require {
			m.Requires[req.Mod.Path] = true
		}
		for _, rep := range mf.Replace {
			m.Replaces = append(m.Replaces, r.newReplace(dir, rep.Old.Path, rep.New.Path, rep.New.Version))
		}
		r.modules = append(r.modules, m)
	}
	sort.Slice(r.modules, func(i, j int) bool {
		di, dj := r.modules[i].Dir, r.modules[j].Dir
		if len(di) != len(dj) {
			return len(di) > len(dj)
		}
		return di < dj
	})

	// A go.work at the repository root activates workspace resolution:
	// imports of any member module classify internal, matching how the go
	// tool resolves them in workspace mode.
	workAbs := filepath.Join(t.Root, "go.work")
	if data, err := os.ReadFile(workAbs); err == nil {
		wf, err := modfile.ParseWork("go.work", data, nil)
		if err != nil {
			r.Warnings = append(r.Warnings, fmt.Sprintf("unparseable go.work: %v", err))
		} else {
			r.hasWork = true
			useDirs := map[string]bool{}
			for _, u := range wf.Use {
				d := path.Clean(filepath.ToSlash(u.Path))
				d = strings.TrimPrefix(d, "./")
				if d == "." {
					d = ""
				}
				useDirs[d] = true
			}
			for _, m := range r.modules {
				if useDirs[m.Dir] {
					m.InWorkspace = true
				}
			}
			for _, rep := range wf.Replace {
				r.workReplaces = append(r.workReplaces, r.newReplace("", rep.Old.Path, rep.New.Path, rep.New.Version))
			}
		}
	}
	sort.Strings(r.Warnings)
	return r
}

// newReplace normalizes one replace directive. A replacement without a
// version is a filesystem path (go.mod grammar).
func (r *Resolver) newReplace(baseDir, oldPath, newPath, newVersion string) Replace {
	rep := Replace{OldPath: oldPath, NewPath: newPath}
	if newVersion != "" {
		return rep
	}
	rep.Local = true
	var abs string
	if filepath.IsAbs(newPath) {
		abs = filepath.Clean(newPath)
	} else {
		abs = filepath.Join(r.root, filepath.FromSlash(baseDir), filepath.FromSlash(newPath))
	}
	if rel, err := filepath.Rel(r.root, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		rep.InRepo = true
		rep.RepoDir = filepath.ToSlash(rel)
		if rep.RepoDir == "." {
			rep.RepoDir = ""
		}
	}
	return rep
}

// OwnerOf returns the nearest enclosing module of a repo-relative
// directory ("" or "." for root), or nil.
func (r *Resolver) OwnerOf(dir string) *GoModule {
	if dir == "." {
		dir = ""
	}
	for _, m := range r.modules {
		if m.Dir == "" || m.Dir == dir || strings.HasPrefix(dir, m.Dir+"/") {
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

// Classify resolves one import path for a file owned by module owner
// (nil when the file is outside every module). It returns the class and,
// for internal imports resolved into the tree, the repo-relative package
// directory.
//
// Longest-prefix match across every candidate implements the go tool's
// module selection: a replace of a sub-path wins over the enclosing module
// path, a deeper require wins over a shallower one.
func (r *Resolver) Classify(owner *GoModule, importPath string) (Class, string) {
	if importPath == "C" {
		return ClassCgo, ""
	}
	if IsStdlib(importPath) {
		return ClassStdlib, ""
	}
	if owner == nil {
		return ClassUnknown, ""
	}

	type candidate struct {
		prefix  string
		class   Class
		baseDir string // repo-relative dir of the matched module root
		inTree  bool   // baseDir is meaningful (root module has baseDir "")
		rank    int    // tie-break at equal prefix length: higher wins
	}
	var cands []candidate

	// Self: the owning module's own path.
	if pathPrefix(importPath, owner.Path) {
		cands = append(cands, candidate{owner.Path, ClassInternal, owner.Dir, true, 1})
	}
	// Workspace members, when a root go.work is present.
	if r.hasWork {
		for _, m := range r.modules {
			if m.InWorkspace && pathPrefix(importPath, m.Path) {
				cands = append(cands, candidate{m.Path, ClassInternal, m.Dir, true, 1})
			}
		}
	}
	// Replace directives: go.work replaces override go.mod replaces on the
	// same old path (rank). Replace-to-local counts as internal for
	// boundaries per the resolution spec; only in-repo targets get a tree
	// directory (out-of-repo sources are not in the tree).
	addReplace := func(rep Replace, rank int) {
		if !pathPrefix(importPath, rep.OldPath) {
			return
		}
		if rep.Local {
			cands = append(cands, candidate{rep.OldPath, ClassInternal, rep.RepoDir, rep.InRepo, rank})
			return
		}
		cands = append(cands, candidate{rep.OldPath, ClassExternal, "", false, rank})
	}
	for _, rep := range owner.Replaces {
		addReplace(rep, 2)
	}
	for _, rep := range r.workReplaces {
		addReplace(rep, 3)
	}
	// Requires: resolvable third-party dependency.
	for req := range owner.Requires {
		if pathPrefix(importPath, req) {
			cands = append(cands, candidate{req, ClassExternal, "", false, 0})
		}
	}

	if len(cands) == 0 {
		return ClassUnknown, ""
	}
	best := cands[0]
	for _, c := range cands[1:] {
		if len(c.prefix) > len(best.prefix) ||
			(len(c.prefix) == len(best.prefix) && c.rank > best.rank) {
			best = c
		}
	}
	if best.class != ClassInternal {
		return best.class, ""
	}
	if !best.inTree {
		return ClassInternal, ""
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
	return ClassInternal, dir
}
