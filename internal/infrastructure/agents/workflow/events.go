package workflow

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	workflowdomain "github.com/wixregiga/arclint/internal/domain/workflow"
)

// Events adapts native events into independently reported workflow feedback.
type Events struct {
	root      string
	collector *Collector
	review    application.ReviewWorkflow
}

// NewEvents connects the native event boundary to task evidence and review.
func NewEvents(root string, collector *Collector, review application.ReviewWorkflow) (*Events, error) {
	if collector == nil {
		return nil, fmt.Errorf("workflow events: missing evidence collector")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("workflow events root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, fmt.Errorf("workflow events root: %w", err)
	}
	return &Events{root: resolved, collector: collector, review: review}, nil
}

// Handle never grants approval or blocks a tool. Unavailable review is feedback.
func (events *Events) Handle(ctx context.Context, input []byte) ([]byte, error) {
	var event CodexEvent
	if err := json.Unmarshal(input, &event); err != nil {
		return nativeOutput(map[string]string{"systemMessage": "ArcLint workflow review unavailable: invalid native event JSON."})
	}
	event.Cwd = normalizeEventCwd(event.Cwd)
	if !events.owns(event.Cwd) {
		return []byte("{}\n"), nil
	}
	switch event.HookEventName {
	case sessionStartEvent, userPromptSubmitEvent, preToolUseEvent, postToolUseEvent, stopEvent:
	default:
		return []byte("{}\n"), nil
	}
	evidence, err := events.collector.Collect(ctx, event)
	if err == nil {
		if history, historyErr := events.previousReports(event.SessionID); historyErr != nil {
			evidence.Limits = append(evidence.Limits, "Previous workflow reports unavailable: "+historyErr.Error())
		} else if history != "" {
			evidence.Passages["previous-workflow-reports"] = boundedPassage(history, &evidence.Limits, "previous workflow reports")
			evidence.Limits = append(evidence.Limits, "Up to five previous workflow reports are claims to reassess against current evidence, not proof that old defects persist or approval of current work.")
		}
	}
	assessment := workflowdomain.Assessment{Findings: []workflowdomain.Finding{}, Limits: append([]string{}, evidence.Limits...)}
	var feedback string
	switch {
	case err != nil:
		feedback = "ArcLint workflow review unavailable: " + err.Error()
	case event.HookEventName == sessionStartEvent:
		feedback = "ArcLint workflow evidence initialized; no review performed before a task or action."
	case strings.TrimSpace(evidence.Task) == "":
		feedback = "ArcLint workflow review not performed: no current task was supplied."
	default:
		assessment, err = events.review.Execute(ctx, evidence)
		if err != nil {
			feedback = "ArcLint workflow review unavailable: " + err.Error()
		} else {
			feedback = assessmentText(assessment)
		}
	}
	if err := events.record(event, evidence.Task, assessment, feedback); err != nil {
		feedback += "\nActivity record unavailable: " + err.Error()
	}
	if event.HookEventName == stopEvent {
		return nativeOutput(map[string]string{"systemMessage": feedback})
	}
	return nativeOutput(map[string]any{"hookSpecificOutput": map[string]string{"hookEventName": event.HookEventName, "additionalContext": feedback}})
}

func (events *Events) owns(cwd string) bool {
	if cwd == "" {
		return false
	}
	resolved, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(events.root, resolved)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func assessmentText(assessment workflowdomain.Assessment) string {
	var output strings.Builder
	output.WriteString("ArcLint workflow review: advisory feedback on the current task.\n")
	if len(assessment.Findings) == 0 {
		output.WriteString("No supported workflow departure in the supplied evidence.\n")
	}
	for _, finding := range assessment.Findings {
		_, _ = fmt.Fprintf(&output, "%s: %q\nDeparture: %s\nCorrection: %s\n", finding.Evidence, finding.Quote, finding.Departure, finding.Correction)
	}
	for _, limit := range assessment.Limits {
		_, _ = fmt.Fprintf(&output, "Coverage: %s\n", limit)
	}
	return output.String()
}

func (events *Events) record(event CodexEvent, task string, assessment workflowdomain.Assessment, feedback string) error {
	if event.SessionID == "" {
		return nil
	}
	name := reportName(event.SessionID)
	project, err := os.OpenRoot(events.root)
	if err != nil {
		return fmt.Errorf("workflow activity report: %w", err)
	}
	defer func() { _ = project.Close() }()
	if err := project.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		return fmt.Errorf("workflow activity report: %w", err)
	}
	file, err := project.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("workflow activity report: %w", err)
	}
	defer func() { _ = file.Close() }()
	err = json.NewEncoder(file).Encode(struct {
		Event      string                    `json:"event"`
		Task       string                    `json:"task"`
		Assessment workflowdomain.Assessment `json:"assessment"`
		Feedback   string                    `json:"feedback"`
	}{Event: event.HookEventName, Task: task, Assessment: assessment, Feedback: feedback})
	if err != nil {
		return fmt.Errorf("encode workflow activity report: %w", err)
	}
	return nil
}

func reportName(session string) string {
	sum := sha256.Sum256([]byte(session))
	return filepath.Join(".arclint", "cache", "workflow-guard", "reports", hex.EncodeToString(sum[:])+".jsonl")
}

func (events *Events) previousReports(session string) (string, error) {
	if session == "" {
		return "", nil
	}
	project, err := os.OpenRoot(events.root)
	if err != nil {
		return "", fmt.Errorf("read workflow report history: %w", err)
	}
	defer func() { _ = project.Close() }()
	file, err := project.Open(reportName(session))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read workflow report history: %w", err)
	}
	defer func() { _ = file.Close() }()
	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("read workflow report history: %w", err)
	}
	const historyLimit = 1024 * 1024
	partial := stat.Size() > historyLimit
	if partial {
		if _, err := file.Seek(stat.Size()-historyLimit, 0); err != nil {
			return "", fmt.Errorf("read workflow report history: %w", err)
		}
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), historyLimit)
	retained := []string{}
	for scanner.Scan() {
		if partial {
			partial = false
			continue
		}
		var record struct {
			Task     string `json:"task"`
			Feedback string `json:"feedback"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return "", fmt.Errorf("unreadable report: %w", err)
		}
		if record.Task == "" || record.Feedback == "" {
			continue
		}
		retained = append(retained, record.Feedback)
		if len(retained) > 5 {
			retained = retained[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read workflow report history: %w", err)
	}
	return strings.Join(retained, "\n\n"), nil
}

func nativeOutput(value any) ([]byte, error) {
	output, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode native workflow feedback: %w", err)
	}
	return output, nil
}

// normalizeEventCwd accepts only this process's actual WSL distribution namespace.
// wsl.exe forwards JSON stdin unchanged, so a desktop UNC cwd needs translation.
func normalizeEventCwd(cwd string) string {
	distro := os.Getenv("WSL_DISTRO_NAME")
	if distro == "" {
		return cwd
	}
	normalized := strings.ReplaceAll(cwd, "\\", "/")
	for _, host := range []string{"wsl.localhost", "wsl$"} {
		prefix := "//" + host + "/" + distro + "/"
		if strings.HasPrefix(normalized, prefix) {
			return filepath.Clean("/" + strings.TrimPrefix(normalized, prefix))
		}
	}
	return cwd
}
