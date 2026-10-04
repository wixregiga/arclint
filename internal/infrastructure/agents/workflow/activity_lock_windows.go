package workflow

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func lockActivity(file *os.File) (func() error, error) {
	handle := windows.Handle(file.Fd())
	overlap := &windows.Overlapped{}
	if err := windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, overlap); err != nil {
		return nil, fmt.Errorf("lock activity file: %w", err)
	}
	return func() error {
		if err := windows.UnlockFileEx(handle, 0, 1, 0, overlap); err != nil {
			return fmt.Errorf("unlock activity file: %w", err)
		}
		return nil
	}, nil
}
