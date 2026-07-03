package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindConfigRoot walks upward from start looking for a directory that
// contains .arclint/, stopping at the git root or the filesystem root
// (docs/design/cli.md, config discovery). It returns the repo root — the
// directory containing .arclint/ — not the .arclint directory itself.
func FindConfigRoot(start string) (string, error) {
	// Traversal is pure string manipulation via filepath.Dir/filepath.Abs —
	// it never calls os.Readlink or filepath.EvalSymlinks. That makes it
	// safe from symlink loops by construction: there is no step here that
	// can dereference a symlink and re-enter a directory already visited,
	// since we only ever move to filepath.Dir(dir). Do not "fix" this into
	// a symlink-resolving walk (e.g. via filepath.EvalSymlinks) — that
	// would reintroduce loop risk for no benefit this tool needs.
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("cannot resolve %s — %w", start, err)
	}
	for {
		if isDir(filepath.Join(dir, ".arclint")) {
			return dir, nil
		}
		// The git root is the discovery boundary: never look above it.
		// This is intentional even for nested .git (e.g. a git submodule):
		// if a submodule has its own .git, walking up stops there, and any
		// .arclint/ that lives above the submodule root — in the
		// superproject — is out of scope by design. A submodule is treated
		// as its own independently-configured unit; it never inherits the
		// parent repo's config implicitly.
		if isDir(filepath.Join(dir, ".git")) {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir { // filesystem root
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no .arclint/ found from %s upward — run `arclint init` in the repo root", start)
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
