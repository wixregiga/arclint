// Package workflow supplies native-host evidence and model adapters for advisory workflow review.
package workflow

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	workflowdomain "github.com/wixregiga/arclint/internal/domain/workflow"
)

//go:embed assets/instructions.md
var instructions []byte

const responseSchema = `{"type":"object","additionalProperties":false,"required":["findings","limits"],"properties":{"findings":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["evidence","quote","departure","correction"],"properties":{"evidence":{"type":"string"},"quote":{"type":"string"},"departure":{"type":"string"},"correction":{"type":"string"}}}},"limits":{"type":"array","items":{"type":"string"}}}}`

// Evaluator asks the existing Codex model connection for a candidate assessment.
// Domain acceptance belongs to ReviewWorkflow, never this process adapter.
type Evaluator struct {
	binary, model    string
	instructionsFile string
}

// NewEvaluator selects the installed Codex command and optional explicit model.
func NewEvaluator(binary, model string) *Evaluator {
	if binary == "" {
		binary = "codex"
	}
	return &Evaluator{binary: binary, model: model}
}

// WithInstructionsFile selects the editable installed instructions for this evaluation.
func (e *Evaluator) WithInstructionsFile(path string) *Evaluator {
	selected := *e
	selected.instructionsFile = path
	return &selected
}

// Instructions exposes the editable authored instructions shipped with this release.
func Instructions() string { return string(instructions) }

// EvaluateWorkflow isolates only the nested reviewer process from hooks and tools.
func (e *Evaluator) EvaluateWorkflow(ctx context.Context, evidence workflowdomain.Evidence) (candidate workflowdomain.Assessment, returnErr error) {
	temporary, err := os.MkdirTemp("", "arclint-workflow-review-")
	if err != nil {
		return workflowdomain.Assessment{}, fmt.Errorf("workflow reviewer temporary directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(temporary); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("workflow reviewer cleanup: %w", err))
		}
	}()
	schemaPath := filepath.Join(temporary, "schema.json")
	outputPath := filepath.Join(temporary, "assessment.json")
	if err := os.WriteFile(schemaPath, []byte(responseSchema), 0o600); err != nil {
		return workflowdomain.Assessment{}, fmt.Errorf("workflow reviewer schema: %w", err)
	}
	authored := instructions
	if e.instructionsFile != "" {
		instructionRoot, err := os.OpenRoot(filepath.Dir(e.instructionsFile))
		if err != nil {
			return workflowdomain.Assessment{}, fmt.Errorf("workflow instructions root: %w", err)
		}
		content, readErr := readWithin(instructionRoot, filepath.Base(e.instructionsFile), passageByteLimit)
		closeErr := instructionRoot.Close()
		if readErr != nil {
			return workflowdomain.Assessment{}, fmt.Errorf("workflow instructions: %w", readErr)
		}
		if closeErr != nil {
			return workflowdomain.Assessment{}, fmt.Errorf("workflow instructions root close: %w", closeErr)
		}
		if strings.TrimSpace(string(content)) == "" {
			return workflowdomain.Assessment{}, fmt.Errorf("workflow instructions are empty")
		}
		authored = content
	}
	input, err := json.Marshal(evidence)
	if err != nil {
		return workflowdomain.Assessment{}, fmt.Errorf("workflow reviewer evidence: %w", err)
	}
	const disableFeature = "--disable"
	arguments := []string{"--no-daemon", "exec", "--ephemeral", "--ignore-user-config", "--skip-git-repo-check", "--sandbox", "read-only", disableFeature, hooksField, disableFeature, "shell_tool", disableFeature, "multi_agent", disableFeature, "code_mode_host", "-c", `web_search="disabled"`, "--output-schema", schemaPath, "--output-last-message", outputPath, "--cd", temporary, "--color", "never"}
	if e.model != "" {
		arguments = append(arguments, "--model", e.model)
	}
	arguments = append(arguments, "-")
	executable, err := exec.LookPath(e.binary)
	if err != nil {
		return workflowdomain.Assessment{}, fmt.Errorf("locate workflow reviewer: %w", err)
	}
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Stdin = strings.NewReader(string(authored) + "\nSupplied evidence:\n" + string(input))
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return workflowdomain.Assessment{}, fmt.Errorf("workflow reviewer unavailable: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		return workflowdomain.Assessment{}, fmt.Errorf("workflow reviewer response: %w", err)
	}
	return parseAssessment(content)
}

func parseAssessment(content []byte) (workflowdomain.Assessment, error) {
	var envelope struct {
		Findings json.RawMessage `json:"findings"`
		Limits   json.RawMessage `json:"limits"`
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return workflowdomain.Assessment{}, fmt.Errorf("workflow reviewer JSON: %w", err)
	}
	if len(envelope.Findings) == 0 || len(envelope.Limits) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Findings), []byte("null")) || bytes.Equal(bytes.TrimSpace(envelope.Limits), []byte("null")) {
		return workflowdomain.Assessment{}, fmt.Errorf("workflow reviewer response must contain nonnull findings and limits arrays")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return workflowdomain.Assessment{}, fmt.Errorf("workflow reviewer response contains trailing data")
	}
	var candidate workflowdomain.Assessment
	if err := json.Unmarshal(envelope.Findings, &candidate.Findings); err != nil {
		return workflowdomain.Assessment{}, fmt.Errorf("workflow reviewer findings: %w", err)
	}
	if err := json.Unmarshal(envelope.Limits, &candidate.Limits); err != nil {
		return workflowdomain.Assessment{}, fmt.Errorf("workflow reviewer limits: %w", err)
	}

	return candidate, nil
}
