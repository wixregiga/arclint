// Package gitproject makes the directory a new project lives in: an
// empty directory holding a git repository and nothing else. It never
// writes into a directory that holds anything, a project that cannot be
// started is taken away again, and only a project's own directory is
// ever deleted.
package gitproject

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// gitTimeout bounds git init, so a request never waits on a stuck git.
const gitTimeout = 30 * time.Second

// Make creates dir and initializes a git repository in it, and says
// whether it made dir rather than finding it empty. dir must be an
// absolute path whose parent exists; dir itself must not exist or must
// be an empty directory. When git init fails, dir is taken back as
// Discard takes it back.
func Make(dir string) (bool, error) {
	if !filepath.IsAbs(dir) {
		return false, fmt.Errorf("%q is not an absolute path", dir)
	}
	dir = filepath.Clean(dir)
	parent := filepath.Dir(dir)
	info, err := os.Stat(parent)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false, fmt.Errorf("%s does not exist; create the directory that holds the project first", parent)
	case err != nil:
		return false, fmt.Errorf("inspect %s: %w", parent, err)
	case !info.IsDir():
		return false, fmt.Errorf("%s is not a directory", parent)
	}
	made, err := emptyDirectory(dir)
	if err != nil {
		return false, err
	}
	if err := gitInit(dir); err != nil {
		if discardErr := Discard(dir, made); discardErr != nil {
			return made, fmt.Errorf("%w; and %s could not be taken back: %w", err, dir, discardErr)
		}
		return false, err
	}
	return made, nil
}

// Discard takes back a project that could not be started. Make found
// dir empty or made it, so everything in it is the project's: Discard
// removes dir when made says Make made it, and empties it otherwise.
func Discard(dir string, made bool) error {
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("%q is not an absolute path", dir)
	}
	dir = filepath.Clean(dir)
	if made {
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("remove %s: %w", dir, err)
		}
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return fmt.Errorf("empty %s: %w", dir, err)
		}
	}
	return nil
}

// Delete removes the project at dir from disk, its git repository
// included. dir must be an absolute path to a directory that holds the
// project's rules.arclint.yaml, so nothing but a project's own
// directory is ever deleted.
func Delete(dir string) error {
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("%q is not an absolute path", dir)
	}
	dir = filepath.Clean(dir)
	if dir == filepath.Dir(dir) {
		return fmt.Errorf("%s is the root of the file system", dir)
	}
	info, err := os.Lstat(filepath.Join(dir, rule.RulesetFileName))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%s holds no %s; only a project's own directory is deleted", dir, rule.RulesetFileName)
	case err != nil:
		return fmt.Errorf("inspect %s: %w", dir, err)
	case !info.Mode().IsRegular():
		return fmt.Errorf("%s in %s is not a file; only a project's own directory is deleted", rule.RulesetFileName, dir)
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("delete %s: %w", dir, err)
	}
	return nil
}

// emptyDirectory makes dir, or accepts it when it is an empty
// directory, and says whether it made it.
func emptyDirectory(dir string) (bool, error) {
	err := os.Mkdir(dir, 0o750)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, fs.ErrExist) {
		return false, fmt.Errorf("create %s: %w", dir, err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return false, fmt.Errorf("inspect %s: %w", dir, err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("%s exists and is not a directory", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", dir, err)
	}
	if len(entries) > 0 {
		return false, fmt.Errorf("%s already holds files; choose a new or empty directory", dir)
	}
	return false, nil
}

// gitInit runs git init in dir. Variables that would point git at
// another repository are dropped, so the repository is always dir's.
func gitInit(dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "init", "--quiet")
	command.Dir = dir
	command.Env = slices.DeleteFunc(os.Environ(), func(variable string) bool {
		name, _, _ := strings.Cut(variable, "=")
		return slices.Contains([]string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_PREFIX", "GIT_OBJECT_DIRECTORY"}, name)
	})
	var stderr bytes.Buffer
	command.Stderr = &stderr
	err := command.Run()
	switch {
	case err == nil:
		return nil
	case errors.Is(err, exec.ErrNotFound):
		return errors.New("git is not installed")
	case strings.TrimSpace(stderr.String()) != "":
		return fmt.Errorf("git init: %s", strings.TrimSpace(stderr.String()))
	default:
		return fmt.Errorf("git init: %w", err)
	}
}
