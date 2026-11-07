package studio

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// SourceFiles reads repository files for Studio's read-only source
// excerpts. Every read goes through an os.Root opened on the project,
// so a path or a link leading outside the project is refused by the
// operating system, even when a link is swapped during the read.
type SourceFiles struct{ Project string }

// ReadSource reads one regular file of at most limit bytes. It opens
// the file read-only and writes nothing.
func (s SourceFiles) ReadSource(path string, limit int64) ([]byte, error) {
	root, err := os.OpenRoot(s.Project)
	if err != nil {
		return nil, fmt.Errorf("source project: %w", err)
	}
	defer func() { _ = root.Close() }()
	file, err := root.Open(filepath.FromSlash(path))
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("%s is %d bytes, more than the %d an excerpt reads", path, info.Size(), limit)
	}
	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if int64(len(content)) > limit {
		return nil, errors.New(path + " grew past the excerpt limit while it was read")
	}
	return content, nil
}
