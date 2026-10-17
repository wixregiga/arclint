package hooks

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/wixregiga/arclint/internal/domain/agent"
	"github.com/wixregiga/arclint/internal/domain/vocab"
	vocabfs "github.com/wixregiga/arclint/internal/infrastructure/vocab"
)

const activityLog = progressDirectory + "/activity.jsonl"

// loggedActivity is one line of the activity log: an activity and when
// the command that performed it finished.
type loggedActivity struct {
	Kind  string    `json:"kind"`
	Paths []string  `json:"paths,omitempty"`
	Zones []string  `json:"zones,omitempty"`
	At    time.Time `json:"at"`
}

// timedActivity is an activity and when it happened, so activities seen
// in different places can be put back in the order they happened.
type timedActivity struct {
	activity agent.Activity
	at       time.Time
}

// selfReported are the kinds ArcLint's own commands record; any other line
// in the log is not theirs and is not credited.
var selfReported = map[agent.Kind]bool{agent.ContextObtained: true, agent.CheckRan: true, agent.DomainChanged: true}

// ActivityLog appends the workflow activities ArcLint's own commands
// perform, so the hooks credit what actually ran rather than guessing from
// shell text. It records only in a project whose workflow hooks created the
// progress directory, so projects without the hooks, and CI, gain no files.
type ActivityLog struct{ root, recording string }

// NewActivityLog records for the project at root, whose domain recording is
// at the root-relative path recording.
func NewActivityLog(root, recording string) *ActivityLog {
	return &ActivityLog{root: root, recording: recording}
}

// Record appends activity to the log.
func (log *ActivityLog) Record(activity agent.Activity) (returnErr error) {
	project, err := os.OpenRoot(log.root)
	if err != nil {
		return fmt.Errorf("workflow activity log: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, project.Close()) }()
	if _, err := project.Stat(progressDirectory); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	line, err := json.Marshal(loggedActivity{Kind: string(activity.Kind), Paths: activity.Paths, Zones: activity.Zones, At: time.Now()})
	if err != nil {
		return fmt.Errorf("encode workflow activity: %w", err)
	}
	file, err := project.OpenFile(activityLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("workflow activity log: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, file.Close()) }()
	if _, err := file.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("workflow activity log: %w", err)
	}
	return nil
}

// Recording is the root-relative path of the project's domain recording.
func (log *ActivityLog) Recording() string { return log.recording }

// RecordingState is a digest of the domain recording's content, "absent"
// when it does not exist, or the reason it cannot be read.
func (log *ActivityLog) RecordingState() string {
	recording, err := vocab.NewDomainRecording(log.recording)
	if err != nil {
		return "unreadable: " + err.Error()
	}
	location, err := vocabfs.ResolveRecording(log.root, recording)
	if err != nil {
		return "unreadable: " + err.Error()
	}
	root, err := filepath.EvalSymlinks(log.root)
	if err != nil {
		return "unreadable: " + err.Error()
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "unreadable: " + err.Error()
	}
	relative, err := filepath.Rel(root, location)
	if err != nil {
		return "unreadable: " + err.Error()
	}
	project, err := os.OpenRoot(log.root)
	if err != nil {
		return "unreadable: " + err.Error()
	}
	defer func() { _ = project.Close() }()
	content, err := project.ReadFile(relative)
	if errors.Is(err, fs.ErrNotExist) {
		return "absent"
	}
	if err != nil {
		return "unreadable: " + err.Error()
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// logSize is the activity log's length, zero when there is none.
func logSize(project *os.Root) (int64, error) {
	info, err := project.Stat(activityLog)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("workflow activity log: %w", err)
	}
	return info.Size(), nil
}

// readActivities returns the self-reported activities in the complete log
// lines after cursor, and the cursor after them. A line still being written
// is left for a later read; a line without the time its command finished
// is not one ArcLint wrote. When until is not zero, reading stops at the
// first line whose command finished after it, which a later read returns.
func readActivities(project *os.Root, cursor int64, until time.Time) ([]timedActivity, int64, error) {
	size, err := logSize(project)
	if err != nil || cursor >= size {
		return nil, min(cursor, size), err
	}
	file, err := project.Open(activityLog)
	if err != nil {
		return nil, cursor, fmt.Errorf("workflow activity log: %w", err)
	}
	defer func() { _ = file.Close() }()
	if _, err := file.Seek(cursor, io.SeekStart); err != nil {
		return nil, cursor, fmt.Errorf("workflow activity log: %w", err)
	}
	data, err := io.ReadAll(io.LimitReader(file, size-cursor))
	if err != nil {
		return nil, cursor, fmt.Errorf("workflow activity log: %w", err)
	}
	var activities []timedActivity
	read := 0
	for {
		end := bytes.IndexByte(data[read:], '\n')
		if end < 0 {
			break
		}
		line := data[read : read+end]
		var logged loggedActivity
		if len(line) > 0 && json.Unmarshal(line, &logged) == nil && selfReported[agent.Kind(logged.Kind)] && !logged.At.IsZero() {
			if !until.IsZero() && logged.At.After(until) {
				break
			}
			activities = append(activities, timedActivity{activity: agent.Activity{Kind: agent.Kind(logged.Kind), Paths: logged.Paths, Zones: logged.Zones}, at: logged.At})
		}
		read += end + 1
	}
	return activities, cursor + int64(read), nil
}
