//go:build !unix

package studio

import (
	"fmt"
	"io/fs"
	"os"
)

// openSource walks parts from an os.Root on the project, holding each
// folder open and continuing from it. A folder or file that is a link is
// refused, and each folder held, and the file opened, must be the one
// the walk saw before opening it, so a link swapped in during the walk
// is refused too. These systems have no pipes in the file system to
// block on.
func openSource(project, name string, parts []string) (*os.File, error) {
	root, err := os.OpenRoot(project)
	if err != nil {
		return nil, fmt.Errorf("source project: %w", err)
	}
	held := []*os.Root{root}
	defer func() {
		for _, r := range held {
			_ = r.Close()
		}
	}()
	dir := root
	for _, part := range parts[:len(parts)-1] {
		seen, err := dir.Lstat(part)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", name, err)
		}
		if seen.Mode()&fs.ModeSymlink != 0 || !seen.IsDir() {
			return nil, notFollowed(name, part, fs.ErrInvalid)
		}
		next, err := dir.OpenRoot(part)
		if err != nil {
			return nil, notFollowed(name, part, err)
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
	file, err := dir.Open(base)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", name, err)
	}
	info, err := file.Stat()
	if err != nil || !os.SameFile(seen, info) {
		_ = file.Close()
		return nil, fmt.Errorf("%s changed while it was opened", name)
	}
	return file, nil
}
