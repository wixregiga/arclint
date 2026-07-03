// Package walk is arclint's parallel file walker: it sweeps directory
// trees with fastwalk and filters them with doublestar globs. It is the
// shared traversal layer under `arclint check` and structure rules.
package walk

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/charlievieth/fastwalk"
)

// DefaultExcludes are directory basenames pruned at any depth on every
// walk, before user excludes apply (docs/design/rules.md: node_modules and
// .git are always excluded; the rest are build outputs and arclint's own
// config root).
var DefaultExcludes = []string{
	".git",
	"node_modules",
	"vendor",
	"dist",
	"build",
	".arclint",
}

// WalkFiles walks each root in parallel and returns every regular file
// found, excluding DefaultExcludes directories and any path matching one
// of the exclude globs. Exclude globs are matched against the
// slash-separated path relative to the walked root. Results are
// deduplicated (overlapping roots) and sorted for deterministic output.
func WalkFiles(roots []string, excludes []string) ([]string, error) {
	conf := fastwalk.Config{
		Follow: false, // never follow symlinks
	}

	var (
		mu    sync.Mutex
		files []string
		seen  = make(map[string]struct{})
	)

	for _, root := range roots {
		root := filepath.Clean(root)
		err := fastwalk.Walk(&conf, root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)

			if d.IsDir() {
				if rel == "." {
					return nil
				}
				if isDefaultExcluded(d.Name()) || matchesAnyDir(excludes, rel) {
					return fs.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			if matchesAny(excludes, rel) {
				return nil
			}

			mu.Lock()
			if _, dup := seen[path]; !dup {
				seen[path] = struct{}{}
				files = append(files, path)
			}
			mu.Unlock()
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	// fastwalk visits concurrently, so order is nondeterministic; sort so
	// every downstream consumer (output, tests, baselines) is stable.
	sort.Strings(files)
	return files, nil
}

// Match reports whether the doublestar pattern matches the slash- or
// OS-separated relative path. Invalid patterns simply do not match;
// pattern validation belongs to config loading, not the hot walk path.
func Match(pattern, relpath string) bool {
	return doublestar.MatchUnvalidated(pattern, filepath.ToSlash(relpath))
}

func isDefaultExcluded(basename string) bool {
	for _, d := range DefaultExcludes {
		if basename == d {
			return true
		}
	}
	return false
}

// matchesAny reports whether any exclude glob matches the file path.
func matchesAny(patterns []string, rel string) bool {
	for _, p := range patterns {
		if Match(p, rel) {
			return true
		}
	}
	return false
}

// matchesAnyDir reports whether a directory should be pruned: either the
// glob matches the directory path itself, or the glob is of the form
// "<prefix>/**" and the prefix matches the directory (so "dist/**" prunes
// dist/ instead of descending into it and excluding file by file).
func matchesAnyDir(patterns []string, rel string) bool {
	for _, p := range patterns {
		if Match(p, rel) {
			return true
		}
		if prefix, ok := strings.CutSuffix(p, "/**"); ok && Match(prefix, rel) {
			return true
		}
	}
	return false
}
