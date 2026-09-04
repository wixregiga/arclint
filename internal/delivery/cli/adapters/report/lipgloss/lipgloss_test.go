package lipgloss

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
)

func ansiRenderer() cli.Renderer {
	return NewWithRendererSetup(func(r *lipgloss.Renderer) {
		r.SetColorProfile(termenv.ANSI)
	})
}

func TestLipglossInitPreservesGrammar(t *testing.T) {
	var buf bytes.Buffer
	err := ansiRenderer().Render(&buf, cli.InitReport{Path: "rules.arclint.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	raw := buf.String()
	if !strings.Contains(raw, "\x1b[") {
		t.Fatalf("expected ANSI styling, got %q", raw)
	}
	out := stripANSI(raw)
	if !strings.Contains(out, "rules.arclint.yaml") {
		t.Fatalf("path missing: %q", out)
	}
	if !strings.Contains(out, "arclint check .") {
		t.Fatalf("next-step grammar changed: %q", out)
	}
	if out != "wrote rules.arclint.yaml\nnext: declare your modules, then run `arclint check .`\n" {
		t.Fatalf("stripped grammar = %q", out)
	}
}

func TestLipglossBaselineGrammarAndBoldCounts(t *testing.T) {
	var buf bytes.Buffer
	err := ansiRenderer().Render(&buf, cli.BaselineCaptureReport{
		Result: application.CaptureBaselineResult{Findings: 1, Rules: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := buf.String()
	if !strings.Contains(raw, "\x1b[") {
		t.Fatalf("expected ANSI styling for counts: %q", raw)
	}
	if !strings.Contains(raw, "\x1b[1m1\x1b[0m") {
		t.Fatalf("summary count is not bold: %q", raw)
	}
	out := stripANSI(raw)
	if !strings.Contains(out, "baseline captured:") {
		t.Fatalf("grammar changed: %q", out)
	}
	if !strings.Contains(out, "1 finding(s) across 1 applied rule(s)") {
		t.Fatalf("counts grammar changed: %q", out)
	}
}

func TestLipglossRuleListMutesIDAndColorsSeverity(t *testing.T) {
	var buf bytes.Buffer
	err := ansiRenderer().Render(&buf, cli.RuleListReport{
		Rules: []application.RuleSummary{{
			ID: "arclint:demo", Type: "structure", Severity: "error",
			Claim: "claim text", Assurance: "exact", Provenance: "ns/n@1",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := buf.String()
	if !strings.Contains(raw, "\x1b[") {
		t.Fatalf("expected ANSI styling: %q", raw)
	}
	if !strings.Contains(raw, "\x1b[2marclint:demo\x1b[0m") {
		t.Fatalf("rule ID is not muted: %q", raw)
	}
	if !strings.Contains(raw, "\x1b[31merror\x1b[0m") {
		t.Fatalf("error severity is not colored: %q", raw)
	}
	if !strings.Contains(raw, "\x1b[2m  from ns/n@1\x1b[0m") {
		t.Fatalf("provenance is not muted: %q", raw)
	}
	out := stripANSI(raw)
	want := "arclint:demo  [structure/error/exact]  claim text  from ns/n@1\n"
	if out != want {
		t.Fatalf("stripped grammar = %q, want %q", out, want)
	}
}

func TestLipglossShortWrite(t *testing.T) {
	err := ansiRenderer().Render(&shortWriter{n: 1}, cli.SDKInitReport{Paths: []string{"a.d.ts"}})
	if err == nil {
		t.Fatal("expected short-write error")
	}
	if err != io.ErrShortWrite {
		t.Fatalf("err = %v, want ErrShortWrite", err)
	}
}

// shortWriter accepts at most n bytes then returns n < len with nil error.
type shortWriter struct{ n int }

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

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for i := range len(s) {
		c := s[i]
		if c == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				inEsc = false
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
