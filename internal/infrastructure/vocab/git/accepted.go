// Package gitvocab reads the project's domain file as the remote's
// default branch holds it: the recording the project has agreed on, as
// against the one the working tree proposes. It implements the
// domain-owned vocab.Repository port for reading alone; the default
// branch changes through git, never through this adapter.
package gitvocab

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// defaultBranchRef is the symbolic ref a clone keeps for the remote's
// default branch.
const defaultBranchRef = "refs/remotes/origin/HEAD"

// gitTimeout bounds one git command, so a page request never waits on a
// stuck repository.
const gitTimeout = 10 * time.Second

// ErrNoDefaultBranch marks a project with no remote default branch to
// read: not a git work tree, no git, or no origin/HEAD.
var ErrNoDefaultBranch = errors.New("no remote default branch")

// Repository reads one DomainRecording at the remote's default branch.
type Repository struct {
	root      string
	prefix    string
	ref       string
	branch    string
	recording vocab.DomainRecording
	parser    application.VocabularySource
}

// NewRepository binds the recording under root, the directory holding
// the ruleset, to the remote's default branch. It resolves the branch
// at once, so a project without one is refused with ErrNoDefaultBranch
// and the reason.
func NewRepository(root string, recording vocab.DomainRecording, parser application.VocabularySource) (Repository, error) {
	if root == "" {
		return Repository{}, errors.New("domain file root: empty path")
	}
	if recording.IsZero() {
		return Repository{}, errors.New("domain recording: names no file")
	}
	if parser == nil {
		return Repository{}, errors.New("domain recording parser: missing")
	}
	r := Repository{root: root, recording: recording, parser: parser}
	ref, err := r.git("symbolic-ref", "-q", defaultBranchRef)
	if err != nil {
		return Repository{}, fmt.Errorf("%w: %s is not set: %w", ErrNoDefaultBranch, defaultBranchRef, err)
	}
	r.ref = strings.TrimSpace(string(ref))
	r.branch = strings.TrimPrefix(r.ref, "refs/remotes/")
	prefix, err := r.git("rev-parse", "--show-prefix")
	if err != nil {
		return Repository{}, fmt.Errorf("%w: %w", ErrNoDefaultBranch, err)
	}
	r.prefix = strings.TrimSpace(string(prefix))
	return r, nil
}

// Branch is the remote branch the repository reads, such as origin/main.
func (r Repository) Branch() string { return r.branch }

// Recording returns the DomainRecording the repository reads.
func (r Repository) Recording() vocab.DomainRecording { return r.recording }

// RecordedLanguage reads the recording at the default branch and whether
// the branch holds the file. A file the branch does not hold is an empty
// language, false, and no error; a file that cannot become a valid
// language is an error, never a partial value.
func (r Repository) RecordedLanguage() (vocab.UbiquitousLanguage, bool, error) {
	// The full ref, so a local branch that happens to be named like the
	// remote one is never read in its place.
	data, err := r.git("show", r.ref+":"+r.prefix+r.recording.Path())
	if err != nil {
		if errors.Is(err, errPathMissing) {
			return vocab.UbiquitousLanguage{Recording: r.recording}, false, nil
		}
		return vocab.UbiquitousLanguage{}, false, fmt.Errorf("read %s at %s: %w", r.recording.Path(), r.branch, err)
	}
	lang, err := r.parser.ParseUbiquitousLanguage(data, r.recording)
	if err != nil {
		return vocab.UbiquitousLanguage{}, true, fmt.Errorf("at %s: %w", r.branch, err)
	}
	return lang, true, nil
}

// Record refuses: what the default branch records changes through git.
func (r Repository) Record(vocab.UbiquitousLanguage) error {
	return fmt.Errorf("%s at %s is what the project has accepted; it changes through git, not through this recording", r.recording.Path(), r.branch)
}

// errPathMissing marks a path the branch does not hold.
var errPathMissing = errors.New("the branch does not hold the file")

// git runs one git command in the project root, never taking git's
// optional index lock. Variables that would point git at another
// repository are dropped.
func (r Repository) git(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "git", append([]string{"--no-optional-locks"}, args...)...) //nolint:gosec // the arguments are git's own, spelled in this file; a request reaches none of them
	command.Dir = r.root
	command.Env = slices.DeleteFunc(os.Environ(), func(variable string) bool {
		name, _, _ := strings.Cut(variable, "=")
		return slices.Contains([]string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_PREFIX"}, name)
	})
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err == nil {
		return output, nil
	}
	message := strings.TrimSpace(stderr.String())
	switch {
	case errors.Is(err, exec.ErrNotFound):
		return nil, errors.New("git is not installed")
	case strings.Contains(message, "does not exist in") || strings.Contains(message, "exists on disk, but not in"):
		return nil, errPathMissing
	case message == "":
		return nil, fmt.Errorf("git %s: %w", args[0], err)
	default:
		return nil, fmt.Errorf("git %s: %s", args[0], message)
	}
}
