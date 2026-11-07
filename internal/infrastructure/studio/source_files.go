package studio

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
)

// SourceFiles reads repository files for Studio's read-only source
// excerpts. A read walks the path one folder at a time from an os.Root
// opened on the project, holding each folder open and continuing from
// it, so nothing outside the project is reached and a link is never
// followed in any part of the path: a folder or file that is a link is
// refused, and one swapped for a link while the walk runs is refused
// too, because each folder held is checked to be the folder the walk
// saw before it was opened.
type SourceFiles struct{ Project string }

// ReadSource reads one regular file of at most limit bytes. It opens
// the file read-only, without following a final link and without
// waiting on a pipe or device, and writes nothing.
func (s SourceFiles) ReadSource(name string, limit int64) ([]byte, error) {
	root, err := os.OpenRoot(s.Project)
	if err != nil {
		return nil, fmt.Errorf("source project: %w", err)
	}
	held := []*os.Root{root}
	defer func() {
		for _, r := range held {
			_ = r.Close()
		}
	}()
	parts := strings.Split(path.Clean(name), "/")
	dir := root
	for _, part := range parts[:len(parts)-1] {
		seen, err := dir.Lstat(part)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", name, err)
		}
		if seen.Mode()&fs.ModeSymlink != 0 || !seen.IsDir() {
			return nil, fmt.Errorf("%s passes through %s, which is not a folder; a link is not followed", name, part)
		}
		next, err := dir.OpenRoot(part)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", name, err)
		}
		held = append(held, next)
		opened, err := next.Stat(".")
		if err != nil || !os.SameFile(seen, opened) {
			return nil, fmt.Errorf("%s changed while it was opened", name)
		}
		dir = next
	}
	base := parts[len(parts)-1]
	seen, err := dir.Lstat(base)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", name, err)
	}
	if seen.Mode()&fs.ModeSymlink != 0 {
		return nil, fmt.Errorf("%s is a link; a link is not followed", name)
	}
	file, err := dir.OpenFile(base, openSourceFlags, 0)
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
	if !os.SameFile(seen, info) {
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
