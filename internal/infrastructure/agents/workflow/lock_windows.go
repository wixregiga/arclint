package workflow

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// lockFile holds an exclusive lock on file until the returned func runs.
func lockFile(file *os.File) (func() error, error) {
	region := &windows.Overlapped{}
	if err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, region); err != nil {
		return nil, fmt.Errorf("lock %s: %w", file.Name(), err)
	}
	return unlocker(file, region), nil
}

// tryLockFile takes the exclusive lock only when no one holds it.
func tryLockFile(file *os.File) (func() error, bool, error) {
	region := &windows.Overlapped{}
	err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, region)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("lock %s: %w", file.Name(), err)
	}
	return unlocker(file, region), true, nil
}

func unlocker(file *os.File, region *windows.Overlapped) func() error {
	return func() error {
		if err := windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, region); err != nil {
			return fmt.Errorf("unlock %s: %w", file.Name(), err)
		}
		return nil
	}
}
