package plain

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

func TestPlainInitBytes(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.InitReport{Path: "rules.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	want := "wrote rules.yaml\nnext: declare your modules, then run `arclint check .`\n"
	if buf.String() != want {
		t.Fatalf("bytes = %q, want %q", buf.String(), want)
	}
}

func TestPlainBaselineCaptureBytes(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.BaselineCaptureReport{
		Result: application.CaptureBaselineResult{Findings: 3, Rules: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "baseline captured: 3 finding(s) across 2 applied rule(s)\n"
	if buf.String() != want {
		t.Fatalf("bytes = %q, want %q", buf.String(), want)
	}
}

func TestPlainAgentsStatusBytes(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.AgentsStatusReport{
		Writes: []cli.ArtifactWrite{
			{Changed: true, Path: "AGENTS.md"},
			{Changed: false, Path: "SKILL.md"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "wrote AGENTS.md\nSKILL.md already current\n"
	if buf.String() != want {
		t.Fatalf("bytes = %q, want %q", buf.String(), want)
	}
}

func TestPlainDomainMissingBytes(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.DomainOverviewReport{
		Overview: application.DomainOverview{Found: false, Source: vocab.UbiquitousLanguageFileName},
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "No recorded Ubiquitous Language found") {
		t.Fatalf("missing guidance absent: %q", out)
	}
	if !strings.Contains(out, "arclint domain init") {
		t.Fatalf("init guidance absent: %q", out)
	}
}

func TestPlainShortWrite(t *testing.T) {
	err := New().Render(&shortWriter{n: 3}, cli.InitReport{Path: "rules.yaml"})
	if err == nil {
		t.Fatal("expected short-write error")
	}
	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("err = %v, want ErrShortWrite", err)
	}
}

// shortWriter accepts at most n bytes then returns n < len with nil error.
type shortWriter struct {
	n int
}

func (s *shortWriter) Write(p []byte) (int, error) {
	if s.n <= 0 {
		return 0, nil
	}
	if len(p) > s.n {
		n := s.n
		s.n = 0
		return n, nil
	}
	s.n -= len(p)
	return len(p), nil
}
