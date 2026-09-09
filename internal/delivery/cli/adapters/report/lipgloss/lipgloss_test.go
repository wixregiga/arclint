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
	"github.com/wixregiga/arclint/internal/domain/vocab"
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
	if out != "wrote rules.arclint.yaml\nnext: declare your zones, then run `arclint check .`\n" {
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
			Proposition: "constraint text", Assurance: "exact", Provenance: "ns/n@1",
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
	want := "arclint:demo  [structure/error/exact]  constraint text  from ns/n@1\n"
	if out != want {
		t.Fatalf("stripped grammar = %q, want %q", out, want)
	}
}

func TestLipglossRuleListMutesTheBuiltInOrigin(t *testing.T) {
	var buf bytes.Buffer
	err := ansiRenderer().Render(&buf, cli.RuleListReport{
		Rules: []application.RuleSummary{{
			ID: "aggregate/root-declared", Type: "domain", Severity: "error",
			Proposition: "composed", Assurance: "exact", BuiltIn: true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := buf.String()
	if !strings.Contains(raw, "\x1b[2m  built in from the DDD meta-model\x1b[0m") {
		t.Fatalf("built-in origin is not muted: %q", raw)
	}
	want := "aggregate/root-declared  [domain/error/exact]  composed  built in from the DDD meta-model\n"
	if out := stripANSI(raw); out != want {
		t.Fatalf("stripped grammar = %q, want %q", out, want)
	}
}

func TestLipglossRuleDetailSeparatesConstraintAndOptionalRationale(t *testing.T) {
	for _, rationale := range []string{"", "Keep technology outside the domain."} {
		var buf bytes.Buffer
		err := ansiRenderer().Render(&buf, cli.RuleDetailReport{Detail: application.RuleDetail{
			Summary: application.RuleSummary{ID: "r1", Proposition: "dependencies point inward", Rationale: rationale},
		}})
		if err != nil {
			t.Fatal(err)
		}
		text := stripANSI(buf.String())
		if !strings.Contains(text, "constraint:  dependencies point inward\n") {
			t.Fatalf("constraint missing: %q", text)
		}
		if strings.Contains(text, "rationale:") != (rationale != "") {
			t.Fatalf("rationale presence changed: %q", text)
		}
		if rationale != "" && !strings.Contains(text, rationale) {
			t.Fatalf("authored rationale missing: %q", text)
		}
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

// Explain keeps the plain grammar: bold section headers, sources muted
// between the meaning and the questions.
func TestLipglossDomainExplainPrintsSources(t *testing.T) {
	var buf bytes.Buffer
	doc := vocab.ConceptAggregate.Doc()
	if len(doc.Sources) == 0 {
		t.Fatal("aggregate cites no source in the meta-model")
	}
	err := ansiRenderer().Render(&buf, cli.DomainExplainReport{Docs: []vocab.ConceptDoc{doc}, Single: true})
	if err != nil {
		t.Fatal(err)
	}
	out := stripANSI(buf.String())
	sources := strings.Index(out, "\nSources:\n\n")
	ask := strings.Index(out, "\nAsk:\n\n")
	if sources < 0 || ask < 0 || sources > ask {
		t.Fatalf("Sources must come before Ask:\n%s", out)
	}
	for _, s := range doc.Sources {
		if !strings.Contains(out, "  "+s.String()+"\n") {
			t.Fatalf("missing source %q:\n%s", s, out)
		}
	}
}
