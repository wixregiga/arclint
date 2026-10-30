package studio

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func lockFeatures(file *os.File) (func() error, error) {
	region := &windows.Overlapped{}
	if err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, region); err != nil {
		return nil, fmt.Errorf("lock features: %w", err)
	}
	return func() error {
		if err := windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, region); err != nil {
			return fmt.Errorf("unlock features: %w", err)
		}
		return nil
	}, nil
}
