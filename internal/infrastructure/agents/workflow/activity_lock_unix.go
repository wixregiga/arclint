//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package workflow

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func lockActivity(file *os.File) (func() error, error) {
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil {
		return nil, fmt.Errorf("lock activity file: %w", err)
	}
	return func() error {
		if err := unix.Flock(int(file.Fd()), unix.LOCK_UN); err != nil {
			return fmt.Errorf("unlock activity file: %w", err)
		}
		return nil
	}, nil
}
