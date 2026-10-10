//go:build unix

package hooks

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// lockFile holds an exclusive lock on file until the returned func runs.
func lockFile(file *os.File) (func() error, error) {
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil {
		return nil, fmt.Errorf("lock %s: %w", file.Name(), err)
	}
	return unlocker(file), nil
}

// tryLockFile takes the exclusive lock only when no one holds it.
func tryLockFile(file *os.File) (func() error, bool, error) {
	err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("lock %s: %w", file.Name(), err)
	}
	return unlocker(file), true, nil
}

func unlocker(file *os.File) func() error {
	return func() error {
		if err := unix.Flock(int(file.Fd()), unix.LOCK_UN); err != nil {
			return fmt.Errorf("unlock %s: %w", file.Name(), err)
		}
		return nil
	}
}
