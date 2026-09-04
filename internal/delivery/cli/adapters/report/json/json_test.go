package jsonreport

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/domain/distribution"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

func TestJSONInitShape(t *testing.T) {
	var buf bytes.Buffer
	if err := New().Render(&buf, cli.InitReport{Path: "rules.yaml"}); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["path"] != "rules.yaml" {
		t.Fatalf("path = %v", doc["path"])
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Fatal("missing trailing newline")
	}
}

func TestJSONDomainOverviewMissing(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.DomainOverviewReport{
		Overview: application.DomainOverview{Found: false, Source: vocab.UbiquitousLanguageFileName},
	})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["found"] != false {
		t.Fatalf("found = %v", doc["found"])
	}
	if doc["source"] != vocab.UbiquitousLanguageFileName {
		t.Fatalf("source = %v", doc["source"])
	}
	counts, ok := doc["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts missing: %v", doc)
	}
	for _, key := range []string{"contexts", "entities", "aggregates", "valueObjects", "invariants", "assertions", "specifications", "events", "relations"} {
		if _, ok := counts[key]; !ok {
			t.Fatalf("counts missing %s: %v", key, counts)
		}
	}
}

func TestJSONDomainShowKeys(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.DomainShowReport{
		View: application.DomainDefinitionView{
			Concept:    vocab.ConceptEntity,
			Context:    "Ordering",
			Definition: vocab.Definition{Name: "Order", Definition: "A request"},
			Aggregate:  true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["type"] != "entity" || doc["name"] != "Order" {
		t.Fatalf("doc = %v", doc)
	}
	if doc["aggregate"] != true {
		t.Fatalf("aggregate = %v", doc["aggregate"])
	}
}

func TestJSONRuleListLowerCamel(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.RuleListReport{
		Rules: []application.RuleSummary{{
			ID: "arclint:x", Type: "structure", Severity: "error",
			Claim: "c", Assurance: "exact", Provenance: "ns/n@1",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var docs []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &docs); err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("len = %d", len(docs))
	}
	d := docs[0]
	for _, key := range []string{"id", "type", "severity", "claim", "assurance", "provenance"} {
		if _, ok := d[key]; !ok {
			t.Fatalf("missing %s: %v", key, d)
		}
	}
	if _, ok := d["ID"]; ok {
		t.Fatal("must not emit PascalCase ID")
	}
}

func TestJSONRuleDetailLowerCamel(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.RuleDetailReport{
		Detail: application.RuleDetail{
			Summary:    application.RuleSummary{ID: "r1", Type: "layers", Severity: "warning", Claim: "c", Assurance: "exact"},
			Evidence:   "static",
			Modules:    []string{"app"},
			Exclusions: []application.PolicyNote{{Selectors: []string{"x"}, Reason: "y"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	sum, ok := doc["summary"].(map[string]any)
	if !ok || sum["id"] != "r1" {
		t.Fatalf("summary = %v", doc["summary"])
	}
	if doc["evidence"] != "static" {
		t.Fatalf("evidence = %v", doc["evidence"])
	}
}

func TestJSONRuleTestLowerCamel(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.RuleTestReport{
		Results: []application.RuleTestResult{{
			Name: "t1", RuleID: "r1",
			Missing: []rule.ExpectedFinding{{Kind: rule.FindingViolation, Path: "a.go", Line: 2, Message: "m"}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var docs []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &docs); err != nil {
		t.Fatal(err)
	}
	if docs[0]["name"] != "t1" || docs[0]["ruleId"] != "r1" || docs[0]["passed"] != false {
		t.Fatalf("doc = %v", docs[0])
	}
	missing, ok := docs[0]["missing"].([]any)
	if !ok || len(missing) != 1 {
		t.Fatalf("missing = %v", docs[0]["missing"])
	}
}

func TestJSONBaselineCaptureLowerCamel(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.BaselineCaptureReport{
		Result: application.CaptureBaselineResult{Findings: 3, Rules: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["findings"] != float64(3) || doc["rules"] != float64(2) {
		t.Fatalf("doc = %v", doc)
	}
}

func TestJSONBaselineRefreshLowerCamel(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.BaselineRefreshReport{
		Result: application.RefreshBaselineResult{Findings: 1, Rules: 1, RemovedStale: 4},
	})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["removedStale"] != float64(4) {
		t.Fatalf("doc = %v", doc)
	}
}

func TestJSONAgentsStatusLowerCamel(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.AgentsStatusReport{
		Writes: []cli.ArtifactWrite{{Changed: true, Path: "AGENTS.md"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var docs []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &docs); err != nil {
		t.Fatal(err)
	}
	if docs[0]["changed"] != true || docs[0]["path"] != "AGENTS.md" {
		t.Fatalf("doc = %v", docs[0])
	}
}

func TestJSONPatternsLowerCamel(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.PatternsReport{
		Registry: "https://patterns.example.com",
		Patterns: []application.PatternSummary{{
			Namespace: "arclint", Name: "vertical", Version: "1", Source: distribution.SourceEmbedded, Vendored: true,
			Digest: "sha256:abc", Documentation: "Vertical slices.", Rules: 5, Extensions: 2, Coverage: []string{"go"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Registry string           `json:"registry"`
		Patterns []map[string]any `json:"patterns"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Registry != "https://patterns.example.com" || len(doc.Patterns) != 1 {
		t.Fatalf("doc = %+v", doc)
	}
	row := doc.Patterns[0]
	if row["reference"] != "arclint/vertical@1" || row["namespace"] != "arclint" || row["rules"] != float64(5) ||
		row["source"] != "embedded" || row["vendored"] != true || row["authored"] != false || row["digest"] != "sha256:abc" || row["documentation"] != "Vertical slices." {
		t.Fatalf("row = %v", row)
	}
	if !strings.Contains(buf.String(), `"extensions": 2`) {
		t.Fatalf("keys must be lowerCamel:\n%s", buf.String())
	}
}

func TestJSONPatternVendorInstallExportLowerCamel(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.PatternVendorReport{Result: application.VendorPatternResult{
		Reference: "acme/layers@1.0.0", Digest: "sha256:abc", Source: distribution.SourceRegistry, Path: ".arclint/patterns/acme/layers", Replaced: "0.9.0",
	}})
	if err != nil {
		t.Fatal(err)
	}
	var vendor map[string]any
	if err := json.Unmarshal(buf.Bytes(), &vendor); err != nil {
		t.Fatal(err)
	}
	if vendor["reference"] != "acme/layers@1.0.0" || vendor["path"] != ".arclint/patterns/acme/layers" || vendor["replaced"] != "0.9.0" || vendor["unchanged"] != false {
		t.Fatalf("vendor doc = %v", vendor)
	}

	buf.Reset()
	err = New().Render(&buf, cli.PatternInstallReport{Result: application.InstallPatternResult{
		Reference: "acme/layers@1.0.0", Digest: "sha256:abc", Source: distribution.SourceEmbedded,
		RulesetPath: "rules.yaml", RulesetReplaced: "0.9.0",
		Bound:   []application.BoundModule{{Module: "domain", Paths: []string{"src/domain/**"}}},
		Adopted: []string{"domain"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var install map[string]any
	if err := json.Unmarshal(buf.Bytes(), &install); err != nil {
		t.Fatal(err)
	}
	if install["rulesetPath"] != "rules.yaml" || install["rulesetReplaced"] != "0.9.0" || install["rulesetCreated"] != false {
		t.Fatalf("install doc = %v", install)
	}
	bound, _ := install["bound"].([]any)
	unbound, _ := install["unbound"].([]any)
	if len(bound) != 1 || bound[0].(map[string]any)["module"] != "domain" || unbound == nil || len(unbound) != 0 {
		t.Fatalf("install bindings = %v / %v", install["bound"], install["unbound"])
	}
	if _, present := install["vendoredPath"]; present {
		t.Fatalf("an offline install carries no vendoredPath: %v", install)
	}

	buf.Reset()
	err = New().Render(&buf, cli.PatternExportReport{Result: application.ExportPatternResult{
		Reference: "acme/layers@1.0.0", Digest: "sha256:abc", VersionDir: "registry/acme/layers/1.0.0", IndexPath: "registry/index.json", Replaced: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	var export map[string]any
	if err := json.Unmarshal(buf.Bytes(), &export); err != nil {
		t.Fatal(err)
	}
	if export["versionDir"] != "registry/acme/layers/1.0.0" || export["indexPath"] != "registry/index.json" || export["replaced"] != true {
		t.Fatalf("export doc = %v", export)
	}
}

func TestJSONSDKInitLowerCamel(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.SDKInitReport{Paths: []string{"a.d.ts"}})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	paths, ok := doc["paths"].([]any)
	if !ok || len(paths) != 1 || paths[0] != "a.d.ts" {
		t.Fatalf("doc = %v", doc)
	}
}

func TestJSONDomainInitLowerCamel(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.DomainInitReport{
		Result: application.InitDomainResult{Source: "ubiquitous-language.yaml", Created: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["source"] != "ubiquitous-language.yaml" || doc["created"] != true {
		t.Fatalf("doc = %v", doc)
	}
}

func TestJSONContextPreservesEstablishedKeys(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.ContextReport{
		Context: application.ArchitecturalContext{
			Scope:     "repository",
			Languages: []string{"go"},
			RuleCount: 1,
			Domain: &application.DomainKnowledge{
				Source: "ubiquitous-language.yaml",
				Counts: vocab.Counts{Entities: 1},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["Scope"] != "repository" || doc["RuleCount"] != float64(1) {
		t.Fatalf("top-level established keys lost: %v", doc)
	}
	domain, ok := doc["domain"].(map[string]any)
	if !ok {
		t.Fatalf("domain missing: %v", doc)
	}
	counts, ok := domain["counts"].(map[string]any)
	if !ok || counts["Entities"] != float64(1) {
		t.Fatalf("domain counts established keys lost: %v", domain)
	}
}

func TestJSONShortWrite(t *testing.T) {
	err := New().Render(&shortWriter{n: 2}, cli.InitReport{Path: "rules.yaml"})
	if err == nil {
		t.Fatal("expected short-write error")
	}
	if !errors.Is(err, io.ErrShortWrite) {
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
