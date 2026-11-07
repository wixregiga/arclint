//go:build unix

package studio

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// openSource walks parts from the project with openat, one folder at a
// time from the folder already held. Each folder is opened with
// O_DIRECTORY|O_NOFOLLOW|O_NONBLOCK, so a link, a pipe or any other file
// in a folder's place, even one swapped in during the walk, fails at
// once; the file is opened with O_NOFOLLOW|O_NONBLOCK from the last
// folder, so a final link fails and a pipe does not block.
func openSource(project, name string, parts []string) (*os.File, error) {
	dir, err := unix.Open(project, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("source project: %w", err)
	}
	for _, part := range parts[:len(parts)-1] {
		next, err := unix.Openat(dir, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		_ = unix.Close(dir)
		if err != nil {
			return nil, notFollowed(name, part, err)
		}
		dir = next
	}
	fd, err := unix.Openat(dir, parts[len(parts)-1], unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	_ = unix.Close(dir)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", name, err)
	}
	return os.NewFile(uintptr(fd), name), nil
}
