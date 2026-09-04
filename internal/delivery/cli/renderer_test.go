package cli

import (
	"errors"
	"io"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/distribution"
)

type emptyPatternSource struct{}

func (emptyPatternSource) Available() ([]distribution.Available, error) { return nil, nil }

type recordingRenderer struct {
	reports []Report
	err     error
}

func (r *recordingRenderer) Render(_ io.Writer, report Report) error {
	r.reports = append(r.reports, report)
	return r.err
}

func TestCommandEmitsReportThroughInjectedRenderer(t *testing.T) {
	list, err := application.NewListPatterns(nil, emptyPatternSource{})
	if err != nil {
		t.Fatal(err)
	}
	renderer := &recordingRenderer{}
	command := NewPatternsCommand(PatternCommands{List: list}, renderer)

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

func TestPatternSourceLabelSpellsEveryCarrier(t *testing.T) {
	cases := []struct {
		row  application.PatternSummary
		want string
	}{
		{application.PatternSummary{Source: distribution.SourceEmbedded}, "embedded"},
		{application.PatternSummary{Source: distribution.SourceEmbedded, Vendored: true}, "embedded, vendored"},
		{application.PatternSummary{Source: distribution.SourceEmbedded, Authored: true}, "embedded, authored"},
		{application.PatternSummary{Source: distribution.SourceLocal, Vendored: true}, "vendored"},
		{application.PatternSummary{Source: distribution.SourceLocal, Authored: true}, "authored"},
		{application.PatternSummary{Source: distribution.SourceRegistry}, "registry"},
	}
	for _, tc := range cases {
		if got := PatternSourceLabel(tc.row); got != tc.want {
			t.Errorf("PatternSourceLabel(%+v) = %q, want %q", tc.row, got, tc.want)
		}
	}
	if got := ShortDigest("sha256:0123456789abcdef"); got != "0123456789ab" {
		t.Errorf("ShortDigest = %q", got)
	}
	if got := ShortDigest(""); got != "" {
		t.Errorf("ShortDigest of nothing = %q", got)
	}
}
