package studio

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// SourceFiles reads repository files for Studio's read-only source
// excerpts. Every read goes through an os.Root opened on the project,
// so nothing outside the project is reached. Inside it, a link is never
// followed: a path whose file or any folder is a link is refused, so a
// source file swapped for a link to another file of the project, such
// as an environment file, is not read under the source file's name.
type SourceFiles struct{ Project string }

// ReadSource reads one regular file of at most limit bytes. It opens
// the file read-only, without following a final link and without
// waiting on a pipe or device, checks the path held no link before and
// after the open, and writes nothing.
func (s SourceFiles) ReadSource(name string, limit int64) ([]byte, error) {
	root, err := os.OpenRoot(s.Project)
	if err != nil {
		return nil, fmt.Errorf("source project: %w", err)
	}
	defer func() { _ = root.Close() }()
	if err := withoutLinks(root, name); err != nil {
		return nil, err
	}
	file, err := root.OpenFile(filepath.FromSlash(name), openSourceFlags, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", name, err)
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", name, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", name)
	}
	// The file opened is the one the path names now, through no link.
	if err := withoutLinks(root, name); err != nil {
		return nil, err
	}
	named, err := root.Lstat(filepath.FromSlash(name))
	if err != nil || !os.SameFile(info, named) {
		return nil, fmt.Errorf("%s changed while it was opened", name)
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("%s is %d bytes, more than the %d an excerpt reads", name, info.Size(), limit)
	}
	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	if int64(len(content)) > limit {
		return nil, errors.New(name + " grew past the excerpt limit while it was read")
	}
	return content, nil
}

// withoutLinks refuses a path any of whose folders, or the file itself,
// is a link.
func withoutLinks(root *os.Root, name string) error {
	parts := strings.Split(path.Clean(name), "/")
	for i := range parts {
		prefix := filepath.FromSlash(strings.Join(parts[:i+1], "/"))
		info, err := root.Lstat(prefix)
		if err != nil {
			return fmt.Errorf("stat %s: %w", name, err)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s passes through a link; a link is not followed", name)
		}
	}
	return nil
}
