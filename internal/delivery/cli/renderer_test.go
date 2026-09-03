package cli

import (
	"errors"
	"io"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/pattern"
)

type emptyPatternSource struct{}

func (emptyPatternSource) Patterns() ([]pattern.Pattern, error) { return nil, nil }

type recordingRenderer struct {
	reports []Report
	err     error
}

func (r *recordingRenderer) Render(_ io.Writer, report Report) error {
	r.reports = append(r.reports, report)
	return r.err
}

func TestCommandEmitsReportThroughInjectedRenderer(t *testing.T) {
	list, err := application.NewListPatterns(emptyPatternSource{})
	if err != nil {
		t.Fatal(err)
	}
	renderer := &recordingRenderer{}
	command := NewPatternsCommand(list, renderer)

	if err := command.Run(Context{Stdout: io.Discard}); err != nil {
		t.Fatal(err)
	}
	if len(renderer.reports) != 1 {
		t.Fatalf("reports = %d, want 1", len(renderer.reports))
	}
	if _, ok := renderer.reports[0].(PatternsReport); !ok {
		t.Fatalf("report type = %T, want PatternsReport", renderer.reports[0])
	}

	writeErr := errors.New("write failed")
	renderer.err = writeErr
	if err := command.Run(Context{Stdout: io.Discard}); !errors.Is(err, writeErr) {
		t.Fatalf("error = %v, want wrapped renderer error", err)
	}
}
