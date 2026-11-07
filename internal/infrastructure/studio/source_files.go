package studio

import (
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

// SourceFiles reads repository files for Studio's read-only source
// excerpts. A read walks the path one folder at a time from the project,
// continuing from each folder it holds open, so nothing outside the
// project is reached and a link is never followed in any part of the
// path, including one swapped in while the walk runs. A pipe or device
// in its place is refused without waiting on it.
type SourceFiles struct{ Project string }

// ReadSource reads one regular file of at most limit bytes. It opens
// the file read-only and writes nothing.
func (s SourceFiles) ReadSource(name string, limit int64) ([]byte, error) {
	parts := strings.Split(path.Clean(name), "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return nil, fmt.Errorf("%s is not a path inside the project", name)
		}
	}
	file, err := openSource(s.Project, name, parts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", name, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", name)
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

// notFollowed says a part of the path is not a folder, or is a link,
// which the walk does not follow.
func notFollowed(name, part string, err error) error {
	return fmt.Errorf("%s passes through %s, which is not a folder or is a link; a link is not followed: %w", name, part, err)
}
