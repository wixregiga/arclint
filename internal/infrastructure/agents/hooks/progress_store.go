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
	"slices"
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

// sessionRecord is one session's Progress, how far it has read the
// activity log, and the checkout as it last saw it. A record without a
// cursor is not one this store wrote.
type sessionRecord struct {
	Progress agent.Progress
	Cursor   *int64
	// Snapshot is the checkout at the session's last update, when Observed;
	// a project outside a git work tree has none.
	Snapshot snapshot `json:",omitempty"`
	Observed bool     `json:",omitempty"`
	// BetweenTurns is true from the end of a turn, or the session's start,
	// until the user's next prompt.
	BetweenTurns bool `json:",omitempty"`
	// AttributedFrom is when the agent's work began after a time between
	// turns, zero once a snapshot has been taken since: a file that changed
	// before it is not the agent's work.
	AttributedFrom time.Time `json:",omitzero"`
}

// ProgressStore keeps each session's workflow Progress in the project's
// ArcLint cache. A host can report parallel tool calls at once, so updates
// of one session hold an exclusive file lock.
type ProgressStore struct {
	root, recording string
	checkout        checkout
	// sample takes a snapshot of the checkout.
	sample func() (snapshot, error)
}

// NewProgressStore keeps progress for the project at root, whose domain
// recording is at the root-relative path recording.
func NewProgressStore(root, recording string) *ProgressStore {
	view := checkout{root: root, now: time.Now}
	return &ProgressStore{root: root, recording: recording, checkout: view, sample: view.snapshot}
}

// Update replaces the session's Progress with change's result. change also
// receives, in the order they happened, the activities ArcLint's own
// commands recorded since the session last read the log and the files the
// checkout shows changed since the session last saw it. A new session
// starts reading at the log's end and from the checkout as it is, so
// earlier work is not credited to it; an unreadable record starts the
// session over the same way.
func (store *ProgressStore) Update(session string, change func(agent.Progress, []agent.Activity) agent.Progress) error {
	return store.locked(session, func(project *os.Root, name string) error {
		record, started, err := store.current(project, name)
		if err != nil {
			return err
		}
		recorded, err := store.observe(project, &record, started)
		if err != nil {
			return err
		}
		record.Progress = change(record.Progress, recorded)
		return writeRecord(project, name, record)
	})
}

// StartSession notes that the session started or resumed. A session
// starts between turns: files that changed while it was between turns, as
// before it resumes, are not the agent's work, and the next update sees the
// checkout afresh. A session whose last turn never finished keeps its view,
// so the next update reports what that turn changed.
func (store *ProgressStore) StartSession(session string) error {
	return store.locked(session, func(project *os.Root, name string) error {
		record, _, err := store.current(project, name)
		if err != nil {
			return err
		}
		if record.BetweenTurns {
			record.AttributedFrom = store.checkout.now()
		}
		return writeRecord(project, name, record)
	})
}

// StartTurn notes that the user's prompt started a turn. Files that changed
// since the last turn finished, or since the session started, are not the
// agent's work: the next update sees the checkout afresh and reports none
// of them. After a turn that ended without finishing, such as one the user
// interrupted, the session keeps its view, so the next update reports what
// that turn changed.
func (store *ProgressStore) StartTurn(session string) error {
	return store.locked(session, func(project *os.Root, name string) error {
		record, _, err := store.current(project, name)
		if err != nil {
			return err
		}
		if record.BetweenTurns {
			record.AttributedFrom = store.checkout.now()
		}
		record.BetweenTurns = false
		return writeRecord(project, name, record)
	})
}

// FinishTurn notes that the agent's turn finished.
func (store *ProgressStore) FinishTurn(session string) error {
	return store.locked(session, func(project *os.Root, name string) error {
		record, _, err := store.current(project, name)
		if err != nil {
			return err
		}
		record.BetweenTurns = true
		return writeRecord(project, name, record)
	})
}

// current reads the session's record, or starts one and reports so: it
// reads the activity log from its end, sees the checkout as it is, and is
// between turns until the user's first prompt.
func (store *ProgressStore) current(project *os.Root, name string) (sessionRecord, bool, error) {
	record, found, err := readRecord(project, name)
	if err != nil || found {
		return record, false, err
	}
	if err := pruneSessions(project, name); err != nil {
		return record, false, err
	}
	end, err := logSize(project)
	if err != nil {
		return record, false, err
	}
	record.Cursor = &end
	record.Snapshot, _, record.Observed = store.look()
	record.BetweenTurns = true
	return record, true, nil
}

// look takes a snapshot of the checkout and reports when it began and
// whether it could.
func (store *ProgressStore) look() (snapshot, time.Time, bool) {
	began := store.checkout.now()
	taken, err := store.sample()
	return taken, began, err == nil
}

// observe returns the activities since the record's last update, in the
// order they happened, and advances the record past them. A record that
// started just now has seen the checkout already. When the checkout cannot
// be read this time, the record keeps its snapshot, so a later update
// reports the files changed meanwhile.
//
// An update covers what happened before its sample of the checkout began.
// Whatever a parallel tool call does while the sample runs may or may not
// show in it, so a log line whose command finished then, and a file that
// changed then, wait for the next update; the record keeps the file's
// earlier fingerprint, so that update finds the change again and puts both
// in the order they happened.
func (store *ProgressStore) observe(project *os.Root, record *sessionRecord, started bool) ([]agent.Activity, error) {
	var now snapshot
	var began time.Time
	seen := false
	if !started {
		now, began, seen = store.look()
	}
	logged, cursor, err := readActivities(project, *record.Cursor, began)
	if err != nil {
		return nil, err
	}
	record.Cursor = &cursor
	timed := logged
	if seen {
		if record.Observed {
			for _, file := range store.checkout.changes(record.Snapshot, now) {
				switch {
				case file.at.After(began):
					if was, known := record.Snapshot[file.path]; known {
						now[file.path] = was
					} else {
						delete(now, file.path)
					}
				case file.at.Before(record.AttributedFrom):
				default:
					timed = append(timed, timedActivity{activity: store.changeOf(file.path), at: file.at})
				}
			}
		}
		record.Snapshot, record.Observed, record.AttributedFrom = now, true, time.Time{}
	}
	return untimed(timed), nil
}

// untimed puts activities in the order they happened and drops the times.
func untimed(timed []timedActivity) []agent.Activity {
	slices.SortStableFunc(timed, func(a, b timedActivity) int { return a.at.Compare(b.at) })
	activities := make([]agent.Activity, len(timed))
	for index, entry := range timed {
		activities[index] = entry.activity
	}
	return activities
}

// changeOf is the activity a changed file of the checkout stands for.
func (store *ProgressStore) changeOf(path string) agent.Activity {
	if path == store.recording {
		return agent.Activity{Kind: agent.DomainChanged, Paths: []string{path}}
	}
	return agent.Activity{Kind: agent.FilesChanged, Paths: []string{path}}
}

func writeRecord(project *os.Root, name string, record sessionRecord) error {
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode workflow progress: %w", err)
	}
	return replaceFile(project, name, encoded)
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
