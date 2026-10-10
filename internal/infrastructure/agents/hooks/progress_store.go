package hooks

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

	"github.com/wixregiga/arclint/internal/domain/agent"
)

const (
	progressDirectory = ".arclint/cache/hooks"
	// deliveredSuffix names the file beside a session record holding the
	// digest of the last event the session received.
	deliveredSuffix = ".event"
	// staleSession is how long an untouched session record is kept.
	staleSession = 30 * 24 * time.Hour
)

// sessionRecord is one session's Progress and how far it has read the
// activity log. A record without a cursor is not one this store wrote.
type sessionRecord struct {
	Progress agent.Progress
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
func (store *ProgressStore) Update(session string, change func(agent.Progress, []agent.Activity) agent.Progress) error {
	return store.locked(session, func(project *os.Root, name string) error {
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
		return replaceFile(project, name, encoded)
	})
}

// Deliver reports whether event, a digest of one host event, differs from
// the event the session last received, and keeps it as the last one. A
// host that lists the hooks in two configuration files runs both with the
// same event at once; the session lock lets only the first through.
func (store *ProgressStore) Deliver(session, event string) (bool, error) {
	first := false
	err := store.locked(session, func(project *os.Root, name string) error {
		last, err := project.ReadFile(name + deliveredSuffix)
		if err == nil && string(last) == event {
			return nil
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("workflow delivery: %w", err)
		}
		first = true
		return replaceFile(project, name+deliveredSuffix, []byte(event))
	})
	return first, err
}

// locked runs work while holding the session's exclusive lock, with the
// project's ArcLint cache open and the session record's name.
func (store *ProgressStore) locked(session string, work func(project *os.Root, name string) error) (returnErr error) {
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
	return work(project, name)
}

// replaceFile writes a file of the cache through a temporary file, so a
// reader never sees part of it.
func replaceFile(project *os.Root, name string, content []byte) error {
	temporary := name + ".tmp-" + rand.Text()
	if err := project.WriteFile(temporary, content, 0o600); err != nil {
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
		case strings.Contains(entry.Name(), ".tmp-") || strings.HasSuffix(entry.Name(), deliveredSuffix):
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
	if err := errors.Join(removeIfPresent(project, name, name+deliveredSuffix), unlock(), lock.Close()); err != nil {
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
