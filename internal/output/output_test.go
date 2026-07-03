package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jofyi/arclint/internal/config"
	"github.com/jofyi/arclint/internal/rules"
)

func sampleViolations() []rules.Violation {
	line := 14
	return []rules.Violation{
		{
			RuleID:   "no-utils-dir",
			Category: config.CategoryStructure,
			Severity: config.SeverityError,
			Path:     "pkg/utils/x.go",
			Message:  "file matches forbidden pattern **/utils/**",
			FixHint:  "Move helpers next to the code that uses them.",
		},
		{
			RuleID:   "layered-architecture",
			Category: config.CategoryDependencies,
			Severity: config.SeverityError,
			Path:     "internal/domain/user.go",
			Line:     &line,
			Message:  "imports the infra layer",
			FixHint:  "",
		},
	}
}

func TestTextGroupedOutput(t *testing.T) {
	var buf bytes.Buffer
	Text(&buf, sampleViolations(), Summary{Total: 2, FilesScanned: 42, DurationMs: 7}, false)
	out := buf.String()

	for _, want := range []string{
		"structure (1)",
		"dependencies (1)",
		"no-utils-dir",
		"pkg/utils/x.go",
		"internal/domain/user.go:14",
		"fix: Move helpers next to the code that uses them.",
		"2 violations (1 structure, 1 dependencies) in 42 files, 7ms",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q\n%s", want, out)
		}
	}
}

func TestTextQuiet(t *testing.T) {
	var buf bytes.Buffer
	Text(&buf, sampleViolations(), Summary{Total: 2, FilesScanned: 42, DurationMs: 7}, true)
	out := buf.String()
	if strings.Contains(out, "structure (1)") || strings.Contains(out, "fix:") {
		t.Errorf("quiet output must drop headers and fix hints\n%s", out)
	}
	if !strings.Contains(out, "no-utils-dir") || !strings.Contains(out, "2 violations") {
		t.Errorf("quiet output must keep violations and summary\n%s", out)
	}

	buf.Reset()
	Text(&buf, nil, Summary{FilesScanned: 42, DurationMs: 3}, true)
	if buf.Len() != 0 {
		t.Errorf("clean quiet run must print nothing, got %q", buf.String())
	}
}

func TestTextClean(t *testing.T) {
	var buf bytes.Buffer
	Text(&buf, nil, Summary{FilesScanned: 412, DurationMs: 31}, false)
	if got := buf.String(); got != "0 violations in 412 files, 31ms\n" {
		t.Errorf("unexpected clean output %q", got)
	}
}

func TestJSONShape(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, sampleViolations(), Summary{Total: 2, FilesScanned: 42, DurationMs: 7}); err != nil {
		t.Fatal(err)
	}

	var got struct {
		Violations []map[string]any `json:"violations"`
		Summary    map[string]any   `json:"summary"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(got.Violations) != 2 {
		t.Fatalf("want 2 violations, got %d", len(got.Violations))
	}

	first := got.Violations[0]
	if first["ruleId"] != "no-utils-dir" || first["category"] != "structure" || first["severity"] != "error" {
		t.Errorf("unexpected first violation: %v", first)
	}
	if _, hasLine := first["line"]; hasLine {
		t.Error("line must be omitted when not line-anchored")
	}
	if _, hasHint := first["fixHint"]; !hasHint {
		t.Error("fixHint must always be present")
	}
	if got.Violations[1]["line"] != float64(14) {
		t.Errorf("second violation line: %v", got.Violations[1]["line"])
	}
	if got.Summary["total"] != float64(2) || got.Summary["filesScanned"] != float64(42) || got.Summary["durationMs"] != float64(7) {
		t.Errorf("unexpected summary: %v", got.Summary)
	}
}

func TestJSONEmptyArrayNotNull(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, nil, Summary{FilesScanned: 1}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"violations": []`) {
		t.Errorf("clean run must emit an empty array, got %s", buf.String())
	}
}
