package workflow

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

	"github.com/wixregiga/arclint/internal/domain/workflow"
)

const activityLog = progressDirectory + "/activity.jsonl"

// loggedActivity is one line of the activity log.
type loggedActivity struct {
	Kind  string   `json:"kind"`
	Paths []string `json:"paths,omitempty"`
	Zones []string `json:"zones,omitempty"`
}

// selfReported are the kinds ArcLint's own commands record; any other line
// in the log is not theirs and is not credited.
var selfReported = map[workflow.Kind]bool{workflow.ContextObtained: true, workflow.CheckRan: true, workflow.DomainChanged: true}

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
func (log *ActivityLog) Record(activity workflow.Activity) (returnErr error) {
	project, err := os.OpenRoot(log.root)
	if err != nil {
		return fmt.Errorf("workflow activity log: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, project.Close()) }()
	if _, err := project.Stat(progressDirectory); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	line, err := json.Marshal(loggedActivity{Kind: string(activity.Kind), Paths: activity.Paths, Zones: activity.Zones})
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

// RecordingState is a digest of the domain recording's content, "absent"
// when it does not exist, or the reason it cannot be read.
func (log *ActivityLog) RecordingState() string {
	project, err := os.OpenRoot(log.root)
	if err != nil {
		return "unreadable: " + err.Error()
	}
	defer func() { _ = project.Close() }()
	content, err := project.ReadFile(log.recording)
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
// is left for a later read.
func readActivities(project *os.Root, cursor int64) ([]workflow.Activity, int64, error) {
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
	complete := bytes.LastIndexByte(data, '\n') + 1
	var activities []workflow.Activity
	for _, line := range bytes.Split(data[:complete], []byte("\n")) {
		var logged loggedActivity
		if len(line) == 0 || json.Unmarshal(line, &logged) != nil || !selfReported[workflow.Kind(logged.Kind)] {
			continue
		}
		activities = append(activities, workflow.Activity{Kind: workflow.Kind(logged.Kind), Paths: logged.Paths, Zones: logged.Zones})
	}
	return activities, cursor + int64(complete), nil
}
