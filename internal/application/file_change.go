package application

import (
	"errors"
	"fmt"
)

// FileChange is one file a command changes, worked out in memory before
// anything is written. Apply writes it, and a failed Apply leaves the
// file as it was. Revert puts an applied change back exactly as the file
// was when the change was worked out.
type FileChange interface {
	Apply() error
	Revert() error
}

// applyTogether writes changes in order, all or nothing: when one fails,
// the ones already written are put back, last first, so the error means
// nothing changed. When putting one back fails as well, both failures
// are returned, because the files are then neither before nor after.
func applyTogether(changes ...FileChange) error {
	for i, change := range changes {
		err := change.Apply()
		if err == nil {
			continue
		}
		var undo []error
		for j := i - 1; j >= 0; j-- {
			if revertErr := changes[j].Revert(); revertErr != nil {
				undo = append(undo, revertErr)
			}
		}
		if len(undo) > 0 {
			return fmt.Errorf("%w; putting the files back failed too, so they are left part way: %w", err, errors.Join(undo...))
		}
		return fmt.Errorf("%w; nothing was changed", err)
	}
	return nil
}
