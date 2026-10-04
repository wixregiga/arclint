package workflow

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	workflowdomain "github.com/wixregiga/arclint/internal/domain/workflow"
)

func TestEvaluatorProcessValidatesResponseAndUsesEditableInstructions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake shell process requires a POSIX host; decoder tests run on every platform")
	}
	directory := t.TempDir()
	binary := filepath.Join(directory, "fake-codex")
	request := filepath.Join(directory, "request")
	arguments := filepath.Join(directory, "arguments")
	const fake = `#!/bin/sh
printf '%s\n' "$@" > "$WORKFLOW_TEST_ARGUMENTS"
cat > "$WORKFLOW_TEST_REQUEST"
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--output-last-message" ]; then
    shift
    printf '%s' "$WORKFLOW_TEST_RESPONSE" > "$1"
    exit 0
  fi
  shift
done
exit 1
`
	if err := os.WriteFile(binary, []byte(fake), 0o700); err != nil {
		t.Fatal(err)
	}
	instructionPath := filepath.Join(directory, "instructions.md")
	if err := os.WriteFile(instructionPath, []byte("Editable review instruction."), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WORKFLOW_TEST_REQUEST", request)
	t.Setenv("WORKFLOW_TEST_ARGUMENTS", arguments)
	evaluator := NewEvaluator(binary, "explicit-model").WithInstructionsFile(instructionPath)
	for _, response := range []string{`{}`, `null`, `{"findings":null,"limits":[]}`, `{"findings":[],"limits":[]} {}`, `{"findings":[],"limits":[]}`} {
		t.Setenv("WORKFLOW_TEST_RESPONSE", response)
		_, err := evaluator.EvaluateWorkflow(context.Background(), workflowdomain.Evidence{Task: "Review current work", Passages: map[string]string{"current-task": "Review current work"}})
		valid := response == `{"findings":[],"limits":[]}`
		if (err == nil) != valid {
			t.Fatalf("response %s: error %v", response, err)
		}
	}
	content, err := os.ReadFile(request)
	if err != nil || !strings.Contains(string(content), "Editable review instruction.") || !strings.Contains(string(content), "Review current work") {
		t.Fatalf("installed instruction/task absent from actual process request: %s, %v", content, err)
	}
	actual, err := os.ReadFile(arguments)
	if err != nil {
		t.Fatal(err)
	}
	for _, argument := range []string{"--no-daemon\nexec\n", "--sandbox\nread-only\n", "--disable\nhooks\n", "--ignore-user-config\n", "--model\nexplicit-model\n"} {
		if !strings.Contains(string(actual), argument) {
			t.Fatalf("nested process lacks %q: %s", argument, actual)
		}
	}
}

func TestAssessmentParserRejectsUnavailableResponseShapes(t *testing.T) {
	for _, response := range []string{`{}`, `null`, `{"findings":null,"limits":[]}`, `{"findings":[],"limits":null}`, `{"findings":[],"limits":[]}{"findings":[],"limits":[]}`, `{"findings":{},"limits":[]}`, `{"findings":[],"limits":{}}`, `{"findings":[],"limits":[],"approval":true}`} {
		t.Run(response, func(t *testing.T) {
			if _, err := parseAssessment([]byte(response)); err == nil {
				t.Fatal("accepted invalid model response")
			}
		})
	}
	for _, response := range []string{`{"findings":[],"limits":[]}`, `{"findings":[{"evidence":"task-actions","quote":"observed","departure":"required workflow departed","correction":"record basis"}],"limits":["only supplied evidence reviewed"]}`} {
		if _, err := parseAssessment([]byte(response)); err != nil {
			t.Fatalf("rejected valid JSON shape: %v", err)
		}
	}
}
