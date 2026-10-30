//go:build unix

package studio

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// lockFeatures follows the existing workflow progress store's file-lock protocol.
func lockFeatures(file *os.File) (func() error, error) {
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil {
		return nil, fmt.Errorf("lock features: %w", err)
	}
	return func() error {
		if err := unix.Flock(int(file.Fd()), unix.LOCK_UN); err != nil {
			return fmt.Errorf("unlock features: %w", err)
		}
		return nil
	}, nil
}
