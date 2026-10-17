package hooks

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// clockFile is the file the hooks write to read the project's file clock.
const clockFile = progressDirectory + "/clock"

// fileClock returns the time the project's file system stamps on a write
// made now. A file's change time comes from the kernel's coarse clock,
// which runs several milliseconds behind the process clock, so a change
// made just after a process-clock reading can carry an earlier time. Every
// time the hooks compare with a file's change time is read here instead:
// a change made after this reading is never stamped before it, and one
// made in the same tick is stamped the same.
func fileClock(root string) (at time.Time, returnErr error) {
	project, err := os.OpenRoot(root)
	if err != nil {
		return time.Time{}, fmt.Errorf("workflow file clock: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, project.Close()) }()
	if err := project.MkdirAll(progressDirectory, 0o700); err != nil {
		return time.Time{}, fmt.Errorf("workflow file clock: %w", err)
	}
	if err := project.WriteFile(clockFile, []byte{'\n'}, 0o600); err != nil {
		return time.Time{}, fmt.Errorf("workflow file clock: %w", err)
	}
	stamped := fingerprintOf(filepath.Join(root, filepath.FromSlash(clockFile)))
	if stamped.Absent {
		return time.Time{}, fmt.Errorf("workflow file clock: %s disappeared after it was written", clockFile)
	}
	return stamped.at(), nil
}
