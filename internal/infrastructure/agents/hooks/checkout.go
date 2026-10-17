package hooks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// cacheDirectory is ArcLint's cache, which no change of the agent's
// touches; excludeCache is the git pathspec that leaves it out.
const (
	cacheDirectory = ".arclint/cache"
	excludeCache   = ":(exclude)" + cacheDirectory
)

// gitTimeout bounds one reading of the checkout, so a hook never waits on a
// stuck repository for the host's whole hook timeout.
const gitTimeout = 10 * time.Second

// fingerprint is what a file of a snapshot looked like: its size, last
// modification, mode and, where the system records one, last change of
// its content or metadata, or that it did not exist. A copy that keeps the
// source's size, modification time and mode, as `cp -p` does, still moves
// the change time, which no program can set. A file whose fingerprint
// differs from the snapshot's changed since it was taken.
type fingerprint struct {
	Absent   bool   `json:"absent,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Modified int64  `json:"modified,omitempty"`
	Mode     uint32 `json:"mode,omitempty"`
	Changed  int64  `json:"changed,omitempty"`
}

// snapshot holds the fingerprint of every project file git shows changed,
// keyed by its slash-separated path relative to the project root.
type snapshot map[string]fingerprint

// changedFile is a file that changed since a snapshot, and when.
type changedFile struct {
	path string
	at   time.Time
}

// checkout reads which files of the project git shows changed: modified,
// added, deleted or untracked and not ignored. It never takes git's
// optional index lock, so it cannot make a git command the agent runs at
// the same moment fail.
type checkout struct {
	root string
	now  func() time.Time
}

// errNoGit marks a project that is not inside a git work tree, or a
// system without git; its shell changes cannot be observed.
var errNoGit = errors.New("not a git work tree")

// snapshot fingerprints every file git shows changed in the project.
// ArcLint's cache is left out: the hooks write it on every event.
func (c checkout) snapshot() (snapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	prefix, err := c.git(exec.CommandContext(ctx, "git", "--no-optional-locks", "rev-parse", "--show-prefix"))
	if err != nil {
		return nil, err
	}
	output, err := c.git(exec.CommandContext(ctx, "git", "--no-optional-locks", "status", "--porcelain=v1", "-z", "--untracked-files=all", "--no-renames", "--ignore-submodules=all", "--", ".", excludeCache))
	if err != nil {
		return nil, err
	}
	base := strings.TrimSpace(string(prefix))
	taken := snapshot{}
	for _, entry := range bytes.Split(output, []byte{0}) {
		// An entry is two status letters, a space and the path from the
		// repository root.
		if len(entry) < 4 {
			continue
		}
		path, inside := strings.CutPrefix(string(entry[3:]), base)
		// A path ending in a slash is a nested repository, whose files
		// are not this project's.
		if !inside || path == "" || strings.HasSuffix(path, "/") {
			continue
		}
		taken[path] = c.fingerprint(path)
	}
	return taken, nil
}

// git runs one git command in the project root. Variables that would point
// git at another repository or index are dropped.
func (c checkout) git(command *exec.Cmd) ([]byte, error) {
	command.Dir = c.root
	command.Env = slices.DeleteFunc(os.Environ(), func(variable string) bool {
		name, _, _ := strings.Cut(variable, "=")
		return slices.Contains([]string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_PREFIX"}, name)
	})
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	var exit *exec.ExitError
	switch {
	case errors.Is(err, exec.ErrNotFound) || (errors.As(err, &exit) && strings.Contains(stderr.String(), "not a git repository")):
		return nil, errNoGit
	case err != nil:
		return nil, fmt.Errorf("%s: %w: %s", strings.Join(command.Args[:3], " "), err, strings.TrimSpace(stderr.String()))
	}
	return output, nil
}

// fingerprint reads one project file without following a link.
func (c checkout) fingerprint(path string) fingerprint {
	return fingerprintOf(filepath.Join(c.root, filepath.FromSlash(path)))
}

// removedAt is when a file that no longer exists was removed, as near as
// the file system tells: removing a file changes its directory, so the
// nearest directory above it that still exists last changed then or
// later. It is never later than now.
func (c checkout) removedAt(path string) time.Time {
	now := c.now()
	for directory := filepath.Dir(filepath.Join(c.root, filepath.FromSlash(path))); ; directory = filepath.Dir(directory) {
		if info, err := os.Stat(directory); err == nil {
			if modified := info.ModTime(); modified.Before(now) {
				return modified
			}
			return now
		}
		if directory == c.root || directory == filepath.Dir(directory) {
			return now
		}
	}
}

// changes lists the files that changed between before and after, ordered
// by when they changed. A file git shows changed in both with another
// fingerprint changed again; a file only after shows changed for the first
// time. A file only before shows is clean again: committed, which changes
// nothing in the file, or restored, which does; its current fingerprint
// tells the two apart. A file that no longer exists changed when it was
// removed.
func (c checkout) changes(before, after snapshot) []changedFile {
	var changed []changedFile
	add := func(path string, now fingerprint) {
		at := time.Unix(0, now.Modified)
		if now.Absent {
			at = c.removedAt(path)
		}
		changed = append(changed, changedFile{path: path, at: at})
	}
	for path, now := range after {
		if was, known := before[path]; !known || was != now {
			add(path, now)
		}
	}
	for path, was := range before {
		if _, still := after[path]; still {
			continue
		}
		if now := c.fingerprint(path); now != was {
			add(path, now)
		}
	}
	slices.SortStableFunc(changed, func(a, b changedFile) int {
		if compared := a.at.Compare(b.at); compared != 0 {
			return compared
		}
		return strings.Compare(a.path, b.path)
	})
	return changed
}
