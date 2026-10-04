package workflow

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	workflowdomain "github.com/wixregiga/arclint/internal/domain/workflow"
)

const (
	passageByteLimit   = 65536
	selectedFileLimit  = 64
	actionHistoryLimit = 64
)

// Collector bounds evidence to one project and the task's actual activity.
type Collector struct {
	root       string
	recordings []string
}

type taskActivity struct {
	Task            string            `json:"task"`
	Actions         []string          `json:"actions"`
	Baseline        map[string]string `json:"baseline"`
	Selected        []string          `json:"selected"`
	TaskLimited     bool              `json:"taskLimited"`
	ActionsLimited  bool              `json:"actionsLimited"`
	ActionTruncated bool              `json:"actionTruncated"`
	BaselineCommit  string            `json:"baselineCommit"`
	InitiallyUnborn bool              `json:"initiallyUnborn"`
}

// NewCollector accepts the recording paths selected by the project's configuration.
func NewCollector(root string, recordingPaths ...string) (*Collector, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("workflow project root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, fmt.Errorf("workflow project root: %w", err)
	}
	if len(recordingPaths) == 0 {
		recordingPaths = []string{"domain.arclint.yaml"}
	}
	return &Collector{root: resolved, recordings: append([]string(nil), recordingPaths...)}, nil
}

// Collect preserves current task/action evidence without approval or blocking state.
func (c *Collector) Collect(ctx context.Context, event CodexEvent) (evidence workflowdomain.Evidence, returnErr error) {
	if event.Cwd != "" {
		eventRoot, err := filepath.EvalSymlinks(event.Cwd)
		if err != nil {
			return workflowdomain.Evidence{}, fmt.Errorf("workflow event cwd: %w", err)
		}
		relative, err := filepath.Rel(c.root, eventRoot)
		if err != nil || !filepath.IsLocal(relative) {
			return workflowdomain.Evidence{}, fmt.Errorf("workflow event belongs to another project; no evidence collected")
		}
	}
	project, err := os.OpenRoot(c.root)
	if err != nil {
		return workflowdomain.Evidence{}, fmt.Errorf("workflow evidence root: %w", err)
	}
	defer func() {
		if err := project.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("workflow evidence root close: %w", err))
		}
	}()
	evidence = workflowdomain.Evidence{Passages: map[string]string{}, Limits: []string{}}
	state := taskActivity{Baseline: map[string]string{}}
	stateName := ""
	if event.SessionID != "" {
		sum := sha256.Sum256([]byte(event.SessionID))
		stateName = ".arclint/cache/workflow-guard/" + hex.EncodeToString(sum[:]) + ".json"
		lockFile, release, err := acquireActivity(project, stateName)
		if err != nil {
			return evidence, fmt.Errorf("workflow task activity lock: %w", err)
		}
		defer func() {
			releaseErr := release()
			closeErr := lockFile.Close()
			if releaseErr != nil {
				returnErr = errors.Join(returnErr, fmt.Errorf("workflow activity unlock: %w", releaseErr))
			}
			if closeErr != nil {
				returnErr = errors.Join(returnErr, fmt.Errorf("workflow lock close: %w", closeErr))
			}
		}()
		state = c.loadActivity(project, stateName, &evidence.Limits)
	} else {
		evidence.Limits = append(evidence.Limits, "No session identity was supplied; prior task activity is unavailable.")
	}
	changes, gitErr := c.changedFiles(ctx, project, state.BaselineCommit, state.InitiallyUnborn)
	if gitErr != nil {
		evidence.Limits = append(evidence.Limits, "Git change evidence is unavailable: "+gitErr.Error())
	}
	if event.HookEventName == userPromptSubmitEvent && strings.TrimSpace(event.Prompt) != "" {
		if state.Task == "" {
			state.Task = event.Prompt
			state.Baseline = changes
			state.BaselineCommit, state.InitiallyUnborn = c.currentCommit(ctx)
		} else {
			state.Task += "\n\nUser follow-up:\n" + event.Prompt
		}
		if len(state.Task) > 4*passageByteLimit {
			state.Task = state.Task[:2*passageByteLimit] + "\n[Earlier intervening requests omitted from bounded evidence.]\n" + state.Task[len(state.Task)-2*passageByteLimit:]
			state.TaskLimited = true
		}
	}
	if state.TaskLimited {
		evidence.Limits = append(evidence.Limits, "Current task retains the beginning and latest requests; intervening request text exceeded the bounded collection.")
	}
	evidence.Task = state.Task
	if state.Task != "" {
		evidence.Passages["current-task"] = boundedPassage(state.Task, &evidence.Limits, "current task passage")
	}
	if event.ToolName != "" {
		result := event.ToolResponse
		if len(result) == 0 {
			result = event.ToolResult
		}
		action, _ := json.Marshal(struct {
			Stage, Tool   string
			Input, Result json.RawMessage
		}{event.HookEventName, event.ToolName, event.ToolInput, result})
		if len(action) > passageByteLimit {
			action = action[:passageByteLimit]
			state.ActionTruncated = true
		}
		state.Actions = append(state.Actions, string(action))
	}
	if len(state.Actions) > actionHistoryLimit {
		state.Actions = state.Actions[len(state.Actions)-actionHistoryLimit:]
		state.ActionsLimited = true
	}
	if state.ActionsLimited {
		evidence.Limits = append(evidence.Limits, "Only the most recent 64 native action records are available.")
	}
	if state.ActionTruncated {
		evidence.Limits = append(evidence.Limits, "At least one supplied native action record exceeded the bounded text limit and was truncated.")
	}
	evidence.Passages["current-stage"] = event.HookEventName
	if len(state.Actions) > 0 {
		evidence.Passages["task-actions"] = strings.Join(state.Actions, "\n")
	}
	if event.LastAssistantMessage != "" {
		evidence.Passages["current-response"] = boundedPassage(event.LastAssistantMessage, &evidence.Limits, "current response")
	}
	if strings.TrimSpace(evidence.Task) == "" {
		evidence.Limits = append(evidence.Limits, "No current task is recorded; no workflow review is established.")
	} else {
		for _, name := range c.recordings {
			c.addFile(project, name, "guidance/", &evidence, true)
		}
		rulesPath := "rules.arclint.yaml"
		if _, err := project.Stat(rulesPath); os.IsNotExist(err) {
			rulesPath = ".arclint/rules.yaml"
		}
		c.addFile(project, rulesPath, "guidance/", &evidence, true)
		c.addFile(project, "AGENTS.md", "guidance/", &evidence, true)
		selected := map[string]bool{}
		for name, current := range changes {
			if previous, found := state.Baseline[name]; !found || previous != current {
				selected[name] = true
			}
		}
		for _, name := range nativePaths(event.ToolInput) {
			selected[name] = true
		}
		for _, name := range state.Selected {
			selected[name] = true
		}
		for _, name := range c.taskPaths(state.Task, project) {
			selected[name] = true
		}
		names := make([]string, 0, len(selected))
		for name := range selected {
			names = append(names, name)
		}
		sort.Strings(names)
		state.Selected = names
		for index, name := range names {
			if index >= selectedFileLimit {
				evidence.Limits = append(evidence.Limits, "Only the first 64 selected files are included.")
				break
			}
			c.addFile(project, name, "work/", &evidence, false)
			c.addNestedGuidance(project, name, &evidence)
		}
	}
	if stateName != "" {
		if err := saveActivity(project, stateName, state); err != nil {
			evidence.Limits = append(evidence.Limits, "Task activity could not be persisted: "+err.Error())
		}
	}
	return evidence, nil
}

func boundedPassage(text string, limits *[]string, subject string) string {
	if len(text) > passageByteLimit {
		*limits = append(*limits, subject+" was truncated at 64 KiB.")
		return text[:passageByteLimit]
	}
	return text
}

func readWithin(root *os.Root, name string, limit int64) ([]byte, error) {
	file, err := root.Open(name)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", name, err)
	}
	content, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	closeErr := file.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read %s: %w", name, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close %s: %w", name, closeErr)
	}
	if int64(len(content)) > limit {
		return nil, fmt.Errorf("%s exceeds %d bytes", name, limit)
	}
	return content, nil
}

func (c *Collector) localName(name string) (string, error) {
	if filepath.IsAbs(name) {
		relative, err := filepath.Rel(c.root, name)
		if err != nil {
			return "", fmt.Errorf("selected path: %w", err)
		}
		name = relative
	}
	name = filepath.ToSlash(name)
	if !filepath.IsLocal(name) || strings.Contains(name, "\\") {
		return "", fmt.Errorf("selected path is outside the project: %s", name)
	}
	if name == ".git" || strings.HasPrefix(name, ".git/") || strings.HasPrefix(name, ".arclint/cache/") {
		return "", fmt.Errorf("runtime metadata is not task file evidence: %s", name)
	}
	return name, nil
}

func (c *Collector) addFile(project *os.Root, name, prefix string, evidence *workflowdomain.Evidence, optional bool) {
	local, err := c.localName(name)
	if err != nil {
		evidence.Limits = append(evidence.Limits, err.Error())
		return
	}
	content, err := readWithin(project, local, passageByteLimit)
	if err != nil {
		if optional && os.IsNotExist(err) {
			evidence.Limits = append(evidence.Limits, "No project guidance at "+local)
			return
		}
		evidence.Limits = append(evidence.Limits, "Selected file evidence unavailable: "+err.Error())
		return
	}
	if bytes.ContainsRune(content, 0) {
		evidence.Limits = append(evidence.Limits, "Binary content was not supplied as text: "+local)
		return
	}
	evidence.Passages[prefix+local] = string(content)
}

func (c *Collector) gitOutput(ctx context.Context, arguments ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git")
	command.Args = append(command.Args, append([]string{"-C", c.root}, arguments...)...)
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("collect git paths: %w", err)
	}
	return output, nil
}

func (c *Collector) currentCommit(ctx context.Context) (string, bool) {
	output, err := c.gitOutput(ctx, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return "", true
	}
	return strings.TrimSpace(string(output)), false
}

func (c *Collector) changedFiles(ctx context.Context, project *os.Root, baselineCommit string, initiallyUnborn bool) (map[string]string, error) {
	changed := map[string]string{}
	status, err := c.gitOutput(ctx, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return changed, err
	}
	names := []string{}
	records := strings.Split(string(status), "\x00")
	for index := 0; index < len(records); index++ {
		record := records[index]
		if len(record) < 4 {
			continue
		}
		names = append(names, record[3:])
		if strings.ContainsAny(record[:2], "RC") && index+1 < len(records) {
			index++
			names = append(names, records[index])
		}
	}
	if baselineCommit != "" {
		committed, err := c.gitOutput(ctx, "diff", "--name-only", "-z", baselineCommit, "--")
		if err != nil {
			return changed, err
		}
		names = append(names, strings.Split(string(committed), "\x00")...)
	} else if initiallyUnborn {
		committed, err := c.gitOutput(ctx, "ls-files", "-z")
		if err != nil {
			return changed, err
		}
		names = append(names, strings.Split(string(committed), "\x00")...)
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		local, err := c.localName(name)
		if err != nil {
			continue
		}
		fingerprint, err := hashWithin(project, local)
		if err != nil {
			changed[local] = "unavailable:" + err.Error()
			continue
		}
		changed[local] = fingerprint
	}
	return changed, nil
}

func nativePaths(input json.RawMessage) []string {
	var value map[string]any
	if json.Unmarshal(input, &value) != nil {
		return nil
	}
	paths := []string{}
	for _, key := range []string{"file_path", "path", "filename"} {
		if path, ok := value[key].(string); ok {
			paths = append(paths, path)
		}
	}
	for _, key := range []string{"patch", "input"} {
		if patch, ok := value[key].(string); ok {
			for _, line := range strings.Split(patch, "\n") {
				for _, prefix := range []string{"*** Add File: ", "*** Update File: ", "*** Delete File: "} {
					if strings.HasPrefix(line, prefix) {
						paths = append(paths, strings.TrimPrefix(line, prefix))
					}
				}
			}
		}
	}
	return paths
}

func (c *Collector) taskPaths(task string, project *os.Root) []string {
	paths := []string{}
	for _, token := range strings.Fields(task) {
		name := strings.Trim(token, "`\"'(),;:")
		if index := strings.Index(name, "]("); index >= 0 {
			name = strings.TrimSuffix(name[index+2:], ")")
		}
		local, err := c.localName(name)
		if err != nil {
			continue
		}
		info, err := project.Stat(local)
		if err == nil && info.Mode().IsRegular() {
			paths = append(paths, local)
		}
	}
	return paths
}

func hashWithin(root *os.Root, name string) (string, error) {
	file, err := root.Open(name)
	if err != nil {
		return "", fmt.Errorf("open baseline file: %w", err)
	}
	hash := sha256.New()
	_, readErr := io.Copy(hash, file)
	closeErr := file.Close()
	if readErr != nil {
		return "", fmt.Errorf("hash baseline file: %w", readErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close baseline file: %w", closeErr)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (c *Collector) loadActivity(project *os.Root, stateName string, limits *[]string) taskActivity {
	state := taskActivity{Baseline: map[string]string{}}
	data, err := readWithin(project, stateName, 8*1024*1024)
	if os.IsNotExist(err) {
		return state
	}
	if err != nil {
		*limits = append(*limits, "Previous task activity could not be read: "+err.Error())
		return state
	}
	if err := json.Unmarshal(data, &state); err != nil {
		*limits = append(*limits, "Previous task activity was unreadable; earlier activity is not established.")
	}
	return state
}

func saveActivity(project *os.Root, stateName string, state taskActivity) error {
	content, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode task activity: %w", err)
	}
	if err := project.MkdirAll(filepath.Dir(stateName), 0o700); err != nil {
		return fmt.Errorf("prepare task activity directory: %w", err)
	}
	temporary := stateName + ".tmp-" + rand.Text()
	file, err := project.OpenFile(temporary, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("open task activity: %w", err)
	}
	_, writeErr := file.Write(content)
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write task activity: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close task activity: %w", closeErr)
	}
	if err := project.Rename(temporary, stateName); err != nil {
		return fmt.Errorf("replace task activity: %w", err)
	}

	return nil
}

func (c *Collector) addNestedGuidance(project *os.Root, name string, evidence *workflowdomain.Evidence) {
	local, err := c.localName(name)
	if err != nil {
		return
	}
	for directory := filepath.Dir(local); directory != "."; directory = filepath.Dir(directory) {
		instructionsPath := filepath.ToSlash(filepath.Join(directory, "AGENTS.md"))
		if _, err := project.Stat(instructionsPath); err == nil {
			c.addFile(project, instructionsPath, "guidance/", evidence, true)
		}
	}
}

func acquireActivity(project *os.Root, stateName string) (*os.File, func() error, error) {
	if err := project.MkdirAll(filepath.Dir(stateName), 0o700); err != nil {
		return nil, nil, fmt.Errorf("prepare activity lock: %w", err)
	}
	file, err := project.OpenFile(stateName+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open activity lock: %w", err)
	}
	release, err := lockActivity(file)
	if err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return nil, nil, fmt.Errorf("activity lock and close failed: %w", errors.Join(err, closeErr))
		}
		return nil, nil, fmt.Errorf("lock task activity: %w", err)
	}
	return file, release, nil
}
