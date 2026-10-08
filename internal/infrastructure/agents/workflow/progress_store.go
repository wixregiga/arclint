package workflow

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/wixregiga/arclint/internal/domain/workflow"
)

const (
	progressDirectory = ".arclint/cache/workflow"
	// staleSession is how long an untouched session record is kept.
	staleSession = 30 * 24 * time.Hour
)

// sessionRecord is one session's Progress and how far it has read the
// activity log. A record without a cursor is not one this store wrote.
type sessionRecord struct {
	Progress workflow.Progress
	Cursor   *int64
}

// ProgressStore keeps each session's workflow Progress in the project's
// ArcLint cache. A host can report parallel tool calls at once, so updates
// of one session hold an exclusive file lock.
type ProgressStore struct{ root string }

// NewProgressStore keeps progress for the project at root.
func NewProgressStore(root string) *ProgressStore { return &ProgressStore{root: root} }

// Update replaces the session's Progress with change's result. change also
// receives the activities ArcLint's own commands recorded since the session
// last read the log. A new session starts reading at the log's end, so
// earlier activity is not credited to it; an unreadable record starts the
// session over the same way.
func (store *ProgressStore) Update(session string, change func(workflow.Progress, []workflow.Activity) workflow.Progress) (returnErr error) {
	project, err := os.OpenRoot(store.root)
	if err != nil {
		return fmt.Errorf("workflow progress: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, project.Close()) }()
	if err := project.MkdirAll(progressDirectory, 0o700); err != nil {
		return fmt.Errorf("workflow progress: %w", err)
	}
	sum := sha256.Sum256([]byte(session))
	name := progressDirectory + "/" + hex.EncodeToString(sum[:]) + ".json"
	lock, err := project.OpenFile(name+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("workflow progress lock: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, lock.Close()) }()
	unlock, err := lockFile(lock)
	if err != nil {
		return fmt.Errorf("workflow progress lock: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, unlock()) }()

	record, found, err := readRecord(project, name)
	if err != nil {
		return err
	}
	if !found {
		if err := pruneSessions(project, name); err != nil {
			return err
		}
		end, err := logSize(project)
		if err != nil {
			return err
		}
		record.Cursor = &end
	}
	recorded, cursor, err := readActivities(project, *record.Cursor)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(sessionRecord{Progress: change(record.Progress, recorded), Cursor: &cursor})
	if err != nil {
		return fmt.Errorf("encode workflow progress: %w", err)
	}
	temporary := name + ".tmp-" + rand.Text()
	if err := project.WriteFile(temporary, encoded, 0o600); err != nil {
		return fmt.Errorf("workflow progress: %w", err)
	}
	if err := project.Rename(temporary, name); err != nil {
		return errors.Join(fmt.Errorf("workflow progress: %w", err), project.Remove(temporary))
	}
	return nil
}

// readRecord reads a session record; found is false for a missing or
// unreadable record.
func readRecord(project *os.Root, name string) (sessionRecord, bool, error) {
	var record sessionRecord
	data, err := project.ReadFile(name)
	if errors.Is(err, fs.ErrNotExist) {
		return record, false, nil
	}
	if err != nil {
		return record, false, fmt.Errorf("workflow progress: %w", err)
	}
	readable := json.Unmarshal(data, &record) == nil && record.Cursor != nil
	if !readable {
		record = sessionRecord{}
	}
	return record, readable, nil
}

// pruneSessions removes session records untouched for staleSession, and
// temporary files a crashed write left behind. A session that still holds
// its lock is in use and is kept. When no other session remains, the
// activity log starts over, which keeps it from growing across sessions
// that no longer read it.
func pruneSessions(project *os.Root, current string) error {
	directory, err := project.Open(progressDirectory)
	if err != nil {
		return fmt.Errorf("workflow progress: %w", err)
	}
	entries, err := directory.ReadDir(-1)
	closeErr := directory.Close()
	if err != nil || closeErr != nil {
		return fmt.Errorf("workflow progress: %w", errors.Join(err, closeErr))
	}
	others := false
	for _, entry := range entries {
		name := progressDirectory + "/" + entry.Name()
		info, err := entry.Info()
		if err != nil || name == current {
			continue
		}
		stale := time.Since(info.ModTime()) >= staleSession
		switch {
		case strings.Contains(entry.Name(), ".tmp-"):
			if stale {
				if err := removeIfPresent(project, name); err != nil {
					return err
				}
			}
		case strings.HasSuffix(name, ".json"):
			if !stale {
				others = true
			} else if removed, err := removeIdleSession(project, name); err != nil {
				return err
			} else if !removed {
				others = true
			}
		}
	}
	if others {
		return nil
	}
	return removeIfPresent(project, activityLog)
}

// removeIdleSession removes a session's record while holding its lock, so
// a session resuming at that moment keeps its record. The lock file goes
// afterwards; one left behind is empty and harmless.
func removeIdleSession(project *os.Root, name string) (bool, error) {
	lock, err := project.OpenFile(name+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return false, fmt.Errorf("workflow progress lock: %w", err)
	}
	unlock, held, err := tryLockFile(lock)
	if err != nil || !held {
		return false, errors.Join(err, lock.Close())
	}
	if err := errors.Join(removeIfPresent(project, name), unlock(), lock.Close()); err != nil {
		return false, err
	}
	_ = project.Remove(name + ".lock")
	return true, nil
}

func removeIfPresent(project *os.Root, names ...string) error {
	for _, name := range names {
		if err := project.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("workflow progress: %w", err)
		}
	}
	return nil
}
