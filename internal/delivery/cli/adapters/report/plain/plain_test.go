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
	err := New().Render(&buf, cli.InitReport{Path: "rules.arclint.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	want := "wrote rules.arclint.yaml\nnext: declare your zones, then run `arclint check .`\n"
	if buf.String() != want {
		t.Fatalf("bytes = %q, want %q", buf.String(), want)
	}
}

// A listing says where a Rule comes from when it is not the
// repository's own: the Pattern that distributed it, or the meta-model
// arclint composes it from; a local Rule carries no note.
func TestPlainRuleListNamesTheOrigin(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.RuleListReport{
		Rules: []application.RuleSummary{
			{ID: "ns/n:x", Type: "structure", Severity: "error", Claim: "distributed", Assurance: "exact", Provenance: "ns/n@1"},
			{ID: "aggregate/root-declared", Type: "domain", Severity: "error", Claim: "composed", Assurance: "exact", BuiltIn: true},
			{ID: "local/own", Type: "content", Severity: "warning", Claim: "written here", Assurance: "exact"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "ns/n:x  [structure/error/exact]  distributed  from ns/n@1\n" +
		"aggregate/root-declared  [domain/error/exact]  composed  built in from the DDD meta-model\n" +
		"local/own  [content/warning/exact]  written here\n"
	if buf.String() != want {
		t.Fatalf("bytes = %q, want %q", buf.String(), want)
	}
}

func TestPlainRuleDetailOfABuiltInSaysHowToAdoptIt(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.RuleDetailReport{Detail: application.RuleDetail{
		Summary:          application.RuleSummary{ID: "aggregate/root-declared", Type: "domain", Severity: "error", Claim: "composed", Assurance: "exact", BuiltIn: true},
		EntireRepository: true,
		Schema:           "schema",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "origin:      built in from the DDD meta-model; adopt it with an override under its id\n") {
		t.Fatalf("detail = %q", buf.String())
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
	err := New().Render(&buf, cli.ArtifactStatusReport{
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
	err := New().Render(&shortWriter{n: 3}, cli.InitReport{Path: "rules.arclint.yaml"})
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

// Explain prints the meta-model's sources for the concept between the
// meaning and the questions, one reference per line.
func TestPlainDomainExplainPrintsSources(t *testing.T) {
	var buf bytes.Buffer
	doc := vocab.ConceptAggregate.Doc()
	if len(doc.Sources) == 0 {
		t.Fatal("aggregate cites no source in the meta-model")
	}
	err := New().Render(&buf, cli.DomainExplainReport{Docs: []vocab.ConceptDoc{doc}, Single: true})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
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
