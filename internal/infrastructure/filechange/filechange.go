// Package filechange works out a change to one file in memory and writes
// it later, keeping what the file held before so the change can be put
// back. It implements the application's FileChange for the adapters that
// write the project's files.
package filechange

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// newFileMode is the mode of a file a change creates, as the ruleset
// writer creates one.
const newFileMode fs.FileMode = 0o600

// entry is what one path holds: nothing, a symbolic link, or a file with
// its content and mode.
type entry struct {
	exists bool
	link   string
	data   []byte
	mode   fs.FileMode
}

func (e entry) equal(other entry) bool {
	return e.exists == other.exists && e.link == other.link && e.mode == other.mode && bytes.Equal(e.data, other.data)
}

// Change is one planned change to the file at one path. Nothing is
// written until Apply.
type Change struct {
	path    string
	before  entry
	after   entry
	created []string
}

// Write plans path to hold data. A symbolic link is written through, so
// the link stays and the file it points to changes. An existing file
// keeps its mode; a new file is created owner-only, and so are the
// directories it needs.
func Write(path string, data []byte) (*Change, error) {
	path, err := writeTarget(path)
	if err != nil {
		return nil, err
	}
	before, err := read(path, os.Stat)
	if err != nil {
		return nil, err
	}
	mode := newFileMode
	if before.exists && before.link == "" {
		mode = before.mode
	}
	return &Change{path: path, before: before, after: entry{exists: true, data: data, mode: mode}}, nil
}

// writeTarget is the file a write to path changes: path itself, or the
// file a symbolic link at path points to. A link that points to nothing
// is refused rather than replaced.
func writeTarget(path string) (string, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return path, nil
	}
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", path, err)
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		return path, nil
	}
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("%s is a link to a file that cannot be reached: %w", path, err)
	}
	return target, nil
}

// Remove plans path to hold nothing. A symbolic link is removed itself,
// not the file it points to, and is put back as the same link.
func Remove(path string) (*Change, error) {
	before, err := read(path, os.Lstat)
	if err != nil {
		return nil, err
	}
	return &Change{path: path, before: before}, nil
}

// read records what path holds now; stat chooses whether a symbolic link
// is followed.
func read(path string, stat func(string) (fs.FileInfo, error)) (entry, error) {
	info, err := stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return entry{}, nil
	}
	if err != nil {
		return entry{}, fmt.Errorf("inspect %s: %w", path, err)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		link, err := os.Readlink(path)
		if err != nil {
			return entry{}, fmt.Errorf("read link %s: %w", path, err)
		}
		return entry{exists: true, link: link}, nil
	}
	if info.IsDir() {
		return entry{}, fmt.Errorf("%s is a directory, not a file", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return entry{}, fmt.Errorf("read %s: %w", path, err)
	}
	return entry{exists: true, data: data, mode: info.Mode().Perm()}, nil
}

// Apply writes the planned change. When it fails, the file and any
// directory it created are left as they were.
func (c *Change) Apply() error {
	if c.before.equal(c.after) {
		return nil
	}
	if !c.after.exists {
		if err := os.Remove(c.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", c.path, err)
		}
		return nil
	}
	created, err := makeParents(filepath.Dir(c.path))
	if err != nil {
		return err
	}
	c.created = created
	if err := writeFile(c.path, c.after.data, c.after.mode); err != nil {
		return errors.Join(err, c.removeCreated())
	}
	return nil
}

// Revert puts back what the path held when the change was planned, and
// removes any directory Apply created.
func (c *Change) Revert() error {
	if c.before.equal(c.after) {
		return nil
	}
	switch {
	case !c.before.exists:
		if err := os.Remove(c.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("put back %s: remove it: %w", c.path, err)
		}
		return c.removeCreated()
	case c.before.link != "":
		if err := os.Remove(c.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("put back %s: %w", c.path, err)
		}
		if err := os.Symlink(c.before.link, c.path); err != nil {
			return fmt.Errorf("put back the link %s: %w", c.path, err)
		}
		return nil
	default:
		if err := writeFile(c.path, c.before.data, c.before.mode); err != nil {
			return fmt.Errorf("put back %s: %w", c.path, err)
		}
		return nil
	}
}

// removeCreated removes the directories Apply created, deepest first.
func (c *Change) removeCreated() error {
	var errs []error
	for i := len(c.created) - 1; i >= 0; i-- {
		if err := os.Remove(c.created[i]); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, fmt.Errorf("remove directory %s: %w", c.created[i], err))
		}
	}
	c.created = nil
	return errors.Join(errs...)
}

// makeParents creates dir and any missing parent, owner-only, and
// returns the directories it created, outermost first.
func makeParents(dir string) ([]string, error) {
	var missing []string
	for d := dir; ; d = filepath.Dir(d) {
		_, err := os.Stat(d)
		if err == nil {
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("inspect %s: %w", d, err)
		}
		missing = append(missing, d)
		if filepath.Dir(d) == d {
			break
		}
	}
	var created []string
	for i := len(missing) - 1; i >= 0; i-- {
		if err := os.Mkdir(missing[i], 0o750); err != nil && !errors.Is(err, fs.ErrExist) {
			undo := (&Change{created: created}).removeCreated()
			return nil, errors.Join(fmt.Errorf("create %s: %w", missing[i], err), undo)
		}
		created = append(created, missing[i])
	}
	return created, nil
}

// writeFile replaces path with data through a temporary sibling and a
// rename, so a reader sees the old file or the new one, never part of
// either. No temporary file is left behind.
func writeFile(path string, data []byte, mode fs.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
