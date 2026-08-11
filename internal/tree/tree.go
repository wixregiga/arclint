// Package tree walks a repository into a plain file list. The walk runs
// directories in parallel and the result is deterministic: files are sorted
// by repo-relative path before anything consumes them.
package tree

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
)

// File is one regular file in the repository.
type File struct {
	// Path is repo-root-relative with forward slashes on every platform.
	Path string
	// Abs is the absolute filesystem path.
	Abs string
	// Size in bytes, from the directory entry.
	Size int64
}

// Name returns the base name.
func (f *File) Name() string { return path.Base(f.Path) }

// Ext returns the extension including the dot, or "".
func (f *File) Ext() string { return path.Ext(f.Path) }

// Stem returns the base name without the final extension.
func (f *File) Stem() string { return strings.TrimSuffix(f.Name(), f.Ext()) }

// Dir returns the repo-relative directory ("." for root files).
func (f *File) Dir() string { return path.Dir(f.Path) }

// Tree is the walked repository.
type Tree struct {
	Root  string // absolute
	Files []*File
	// Warnings collects non-fatal walk problems (unreadable directories).
	Warnings []string
}

// Options tunes the walk.
type Options struct {
	// Exclude adds repo-relative doublestar globs on top of the built-in
	// exclusions.
	Exclude []string
	// IncludeTestdata includes testdata/ directories, which Go convention
	// excludes by default.
	IncludeTestdata bool
}

// alwaysSkippedDirs are directory base names never walked: VCS metadata,
// arclint's own state, vendored third-party sources (classified external
// via the manifest, never scanned), and package-manager trees.
var alwaysSkippedDirs = map[string]bool{
	".git":         true,
	".hg":          true,
	".svn":         true,
	".arclint":     true,
	"vendor":       true,
	"node_modules": true,
}

// Walk lists every regular file under root, in parallel, deterministically.
// Symbolic links are not followed (no cycle risk, no duplicate trees).
func Walk(root string, opts Options) (*Tree, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("tree: %s is not a directory", absRoot)
	}
	for _, g := range opts.Exclude {
		if !doublestar.ValidatePattern(g) {
			return nil, fmt.Errorf("tree: invalid exclude pattern %q", g)
		}
	}

	t := &Tree{Root: absRoot}
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		sem      = make(chan struct{}, runtime.GOMAXPROCS(0))
		walkDir  func(abs, rel string)
		excluded = func(rel string, isDir bool) bool {
			for _, g := range opts.Exclude {
				if ok, _ := doublestar.Match(g, rel); ok {
					return true
				}
				// A directory is also excluded when the pattern targets
				// everything below it.
				if isDir {
					if ok, _ := doublestar.Match(g, rel+"/"); ok {
						return true
					}
				}
			}
			return false
		}
	)

	walkDir = func(abs, rel string) {
		defer wg.Done()
		sem <- struct{}{}
		entries, err := os.ReadDir(abs)
		<-sem
		if err != nil {
			mu.Lock()
			t.Warnings = append(t.Warnings, fmt.Sprintf("skipped unreadable directory %s: %v", rel, err))
			mu.Unlock()
			return
		}
		for _, e := range entries {
			name := e.Name()
			childAbs := filepath.Join(abs, name)
			childRel := name
			if rel != "" {
				childRel = rel + "/" + name
			}
			switch {
			case e.IsDir():
				if alwaysSkippedDirs[name] {
					continue
				}
				if name == "testdata" && !opts.IncludeTestdata {
					continue
				}
				if excluded(childRel, true) {
					continue
				}
				wg.Add(1)
				go walkDir(childAbs, childRel)
			case e.Type()&fs.ModeSymlink != 0:
				continue
			case e.Type().IsRegular():
				if excluded(childRel, false) {
					continue
				}
				fi, err := e.Info()
				if err != nil {
					mu.Lock()
					t.Warnings = append(t.Warnings, fmt.Sprintf("skipped unreadable file %s: %v", childRel, err))
					mu.Unlock()
					continue
				}
				mu.Lock()
				t.Files = append(t.Files, &File{Path: childRel, Abs: childAbs, Size: fi.Size()})
				mu.Unlock()
			}
		}
	}

	wg.Add(1)
	walkDir(absRoot, "")
	wg.Wait()

	sort.Slice(t.Files, func(i, j int) bool { return t.Files[i].Path < t.Files[j].Path })
	sort.Strings(t.Warnings)
	return t, nil
}
