// Package vocabfs resolves the filesystem location of a DomainRecording.
package vocabfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// ResolveRecording applies inside-the-repository on the filesystem. It
// resolves the root and the longest existing ancestor, including a file
// symlink, then appends any missing components. A dangling symlink is an
// error, never an absent recording that a writer may replace. Callers resolve
// again at each access because symlinks can change after they are composed.
func ResolveRecording(root string, recording vocab.DomainRecording) (string, error) {
	fail := func(location string, err error) (string, error) {
		return "", fmt.Errorf("DomainRecording inside-the-repository: %q resolves to %q under root %q: %w", recording.Path(), location, root, err)
	}
	if recording.IsZero() {
		return fail("", errors.New("no recording named"))
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return fail(root, err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return fail(abs, err)
	}
	ancestor := filepath.Join(resolvedRoot, filepath.FromSlash(recording.Path()))
	var missing []string
	for {
		_, err := os.Lstat(ancestor)
		if err == nil {
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return fail(ancestor, err)
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return fail(ancestor, err)
		}
		missing = append(missing, filepath.Base(ancestor))
		ancestor = parent
	}
	resolved, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return fail(ancestor, err)
	}
	for i := len(missing) - 1; i >= 0; i-- {
		resolved = filepath.Join(resolved, missing[i])
	}
	relative, err := filepath.Rel(resolvedRoot, resolved)
	if err != nil {
		return fail(resolved, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", locationViolation(fmt.Sprintf("DomainRecording inside-the-repository: %q resolves to %q under root %q: location leaves the repository", recording.Path(), resolved, root))
	}
	return resolved, nil
}

// ErrRecordingLocation matches the error ResolveRecording returns when
// the filesystem shows a DomainRecording leaving the repository through
// a link: the path was a valid DomainRecording and the file it reaches
// is not. A failure to read the filesystem never matches it.
var ErrRecordingLocation = errors.New("the domain file's location violates a DomainRecording invariant")

// locationViolation is a DomainRecording invariant the filesystem shows
// violated; its message names the invariant.
type locationViolation string

func (v locationViolation) Error() string { return string(v) }

// Is reports that every location violation is ErrRecordingLocation.
func (v locationViolation) Is(target error) bool { return target == ErrRecordingLocation }
