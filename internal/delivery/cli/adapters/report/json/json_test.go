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
	if err := New().Render(&buf, cli.InitReport{Path: "rules.arclint.yaml"}); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["path"] != "rules.arclint.yaml" {
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
	for _, key := range []string{"contexts", "aggregates", "entities", "valueObjects", "invariants", "assertions", "specifications", "events", "services", "questions", "relations"} {
		if _, ok := counts[key]; !ok {
			t.Fatalf("counts missing %s: %v", key, counts)
		}
	}
}

func TestJSONDomainShowKeys(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.DomainShowReport{
		View: application.DomainEntryView{
			Concept: vocab.ConceptEntity,
			Context: "ordering",
			Owner:   "Order",
			Name:    "OrderLine",
			Entity:  vocab.Entity{Name: "OrderLine", Definition: "One ticket type on an Order.", Identity: "LineID"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["type"] != "entity" || doc["name"] != "OrderLine" || doc["context"] != "ordering" || doc["owner"] != "Order" {
		t.Fatalf("doc = %v", doc)
	}
	if doc["definition"] != "One ticket type on an Order." || doc["identity"] != "LineID" {
		t.Fatalf("entity properties = %v", doc)
	}
}

// An aggregate carries its whole consistency unit: identity, entities,
// invariants, assertions, and the declared repository and factory.
func TestJSONDomainShowAggregate(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.DomainShowReport{
		View: application.DomainEntryView{
			Concept: vocab.ConceptAggregate,
			Context: "catalog",
			Name:    "Event",
			Aggregate: vocab.Aggregate{
				Name: "Event", Definition: "A scheduled performance.", Identity: "EventID",
				Entities:   []vocab.Entity{{Name: "Organizer", Definition: "Who runs it."}},
				Invariants: []vocab.Invariant{{Key: "published-frozen", Statement: "A published Event never changes."}},
				Assertions: []vocab.Assertion{{Key: "capacity-fits", On: "Publish", Statement: "Capacity fits the venue."}},
				Repository: "EventRepository",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Type     string `json:"type"`
		Identity string `json:"identity"`
		Entities []struct {
			Name string `json:"name"`
		}
		Invariants []struct {
			Key       string `json:"key"`
			Statement string `json:"statement"`
		}
		Assertions []struct {
			Key string `json:"key"`
			On  string `json:"on"`
		}
		Repository string  `json:"repository"`
		Factory    *string `json:"factory"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Type != "aggregate" || doc.Identity != "EventID" || doc.Repository != "EventRepository" || doc.Factory != nil {
		t.Fatalf("doc = %+v", doc)
	}
	if len(doc.Entities) != 1 || doc.Entities[0].Name != "Organizer" {
		t.Fatalf("entities = %+v", doc.Entities)
	}
	if len(doc.Invariants) != 1 || doc.Invariants[0].Key != "published-frozen" {
		t.Fatalf("invariants = %+v", doc.Invariants)
	}
	if len(doc.Assertions) != 1 || doc.Assertions[0].On != "Publish" {
		t.Fatalf("assertions = %+v", doc.Assertions)
	}
}

// A define names the outcome, the entry, and each changed property with
// the value it now holds; a cleared list is an empty list, not null.
func TestJSONDomainDefineCarriesValues(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.DomainDefineReport{
		Result: application.DomainDefineResult{
			Outcome: vocab.OutcomeUpdated, Concept: vocab.ConceptValueObject, Context: "catalog", Name: "Price",
			Changed: []string{"definition", "aliases"},
		},
		Change: vocab.Change{SetDefinition: true, Definition: "Money asked for a seat.", SetAliases: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Result  string   `json:"result"`
		Type    string   `json:"type"`
		Name    string   `json:"name"`
		Context string   `json:"context"`
		Changed []string `json:"changed"`
		Values  struct {
			Definition string   `json:"definition"`
			Aliases    []string `json:"aliases"`
		} `json:"values"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Result != "updated" || doc.Type != "value_object" || doc.Name != "Price" || doc.Context != "catalog" {
		t.Fatalf("doc = %+v", doc)
	}
	if len(doc.Changed) != 2 || doc.Values.Definition != "Money asked for a seat." {
		t.Fatalf("changed = %+v values = %+v", doc.Changed, doc.Values)
	}
	if doc.Values.Aliases == nil || len(doc.Values.Aliases) != 0 {
		t.Fatalf("cleared aliases = %#v, want an empty list", doc.Values.Aliases)
	}
}

func TestJSONDomainRemoveCarriesConsequences(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.DomainRemoveReport{
		Result: application.DomainRemoveResult{
			Concept: vocab.ConceptAggregate, Context: "catalog", Name: "Event",
			Also: []string{"entity Organizer removed with it", "event EventPublished no longer names what raises it"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Type    string   `json:"type"`
		Name    string   `json:"name"`
		Result  string   `json:"result"`
		Context string   `json:"context"`
		Also    []string `json:"also"`
		Changed bool     `json:"sourceFilesChanged"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Type != "aggregate" || doc.Name != "Event" || doc.Result != "removed" || doc.Context != "catalog" || doc.Changed {
		t.Fatalf("doc = %+v", doc)
	}
	if len(doc.Also) != 2 {
		t.Fatalf("also = %v", doc.Also)
	}
}

func TestJSONRuleListLowerCamel(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.RuleListReport{
		Rules: []application.RuleSummary{{
			ID: "arclint:x", Type: "structure", Severity: "error",
			Proposition: "c", Assurance: "exact", Provenance: "ns/n@1",
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
	for _, key := range []string{"id", "type", "severity", "proposition", "assurance", "provenance"} {
		if _, ok := d[key]; !ok {
			t.Fatalf("missing %s: %v", key, d)
		}
	}
	if _, ok := d["ID"]; ok {
		t.Fatal("must not emit PascalCase ID")
	}
	for _, absent := range []string{"claim", "rationale"} {
		if _, ok := d[absent]; ok {
			t.Fatalf("unexpected %s field: %v", absent, d)
		}
	}
	if _, ok := d["builtIn"]; ok {
		t.Fatalf("a distributed Rule must not carry builtIn: %v", d)
	}
}

func TestJSONRuleListMarksBuiltIns(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.RuleListReport{
		Rules: []application.RuleSummary{{
			ID: "aggregate/root-declared", Type: "domain", Severity: "error",
			Proposition: "c", Assurance: "exact", BuiltIn: true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var docs []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &docs); err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 || docs[0]["builtIn"] != true {
		t.Fatalf("docs = %v, want builtIn true", docs)
	}
	if _, ok := docs[0]["provenance"]; ok {
		t.Fatalf("a built-in Rule has no Pattern provenance: %v", docs[0])
	}
}

func TestJSONRuleDetailLowerCamel(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.RuleDetailReport{
		Detail: application.RuleDetail{
			Summary:    application.RuleSummary{ID: "r1", Type: "layers", Severity: "warning", Proposition: "dependencies point inward: app, then domain", Rationale: "Keep technology outside the domain.", Assurance: "exact"},
			Evidence:   "static",
			Zones:      []string{"app"},
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
	if sum["proposition"] != "dependencies point inward: app, then domain" || sum["rationale"] != "Keep technology outside the domain." {
		t.Fatalf("summary = %v", sum)
	}
	if _, ok := doc["asserts"]; ok {
		t.Fatalf("legacy asserts field leaked: %v", doc)
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
	err := New().Render(&buf, cli.ArtifactStatusReport{
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
		RulesetPath: "rules.arclint.yaml", RulesetReplaced: "0.9.0",
		Bound:   []application.BoundZone{{Zone: "domain", Paths: []string{"src/domain/**"}}},
		Adopted: []string{"domain"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var install map[string]any
	if err := json.Unmarshal(buf.Bytes(), &install); err != nil {
		t.Fatal(err)
	}
	if install["rulesetPath"] != "rules.arclint.yaml" || install["rulesetReplaced"] != "0.9.0" || install["rulesetCreated"] != false {
		t.Fatalf("install doc = %v", install)
	}
	bound, _ := install["bound"].([]any)
	unbound, _ := install["unbound"].([]any)
	if len(bound) != 1 || bound[0].(map[string]any)["zone"] != "domain" || unbound == nil || len(unbound) != 0 {
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
		Result: application.InitDomainResult{Source: "domain.arclint.yaml", Project: "boxoffice", Created: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["source"] != "domain.arclint.yaml" || doc["project"] != "boxoffice" || doc["created"] != true {
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
				Source: "domain.arclint.yaml",
				Counts: vocab.Counts{Aggregates: 1},
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
	if !ok || counts["aggregates"] != float64(1) {
		t.Fatalf("domain counts must be lowerCamel like the overview: %v", domain)
	}
}

func TestJSONContextSeparatesPropositionAndOptionalRationale(t *testing.T) {
	for _, rationale := range []string{"", "Keep filenames predictable."} {
		var buf bytes.Buffer
		err := New().Render(&buf, cli.ContextReport{
			Context: application.ArchitecturalContext{Rules: []application.AppliedRule{{
				Summary: application.RuleSummary{ID: "r1", Proposition: "file names use snake_case", Rationale: rationale},
			}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Rules []struct{ Summary map[string]any }
		}
		if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
			t.Fatal(err)
		}
		if len(doc.Rules) != 1 {
			t.Fatalf("rules = %+v", doc.Rules)
		}
		summary := doc.Rules[0].Summary
		if summary["Proposition"] != "file names use snake_case" {
			t.Fatalf("canonical proposition lost: %v", summary)
		}
		text, present := summary["Rationale"]
		if present != (rationale != "") || (present && text != rationale) {
			t.Fatalf("authored rationale changed: %v", summary)
		}
		if _, present := summary["Claim"]; present {
			t.Fatalf("legacy Claim field leaked: %v", summary)
		}
	}
}

func TestJSONContextCarriesScopeAndAnchors(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.ContextReport{
		Context: application.ArchitecturalContext{
			Scope: "m/event.go",
			Domain: &application.DomainKnowledge{
				Source:  "domain.arclint.yaml",
				Counts:  vocab.Counts{Contexts: 2, Entities: 1, Aggregates: 1, ValueObjects: 2, Invariants: 3},
				Scoped:  true,
				Shown:   vocab.Counts{Contexts: 1, Entities: 1, Aggregates: 1, ValueObjects: 1, Invariants: 2},
				Located: true,
				Contexts: []application.DomainContextKnowledge{{
					Name:         "catalog",
					Aggregates:   []application.DomainAggregateRef{{Name: "Event", Identity: "EventID", Entities: []string{"Organizer"}}},
					ValueObjects: []string{"Price"},
					Invariants: []application.DomainInvariantRef{
						{Key: "published-frozen", Statement: "A published Event never changes.", Owner: "Event", OwnerConcept: vocab.ConceptAggregate, Source: "event/event.go:90", Anchor: application.AnchorFound},
						{Key: "one-venue", Statement: "An Event has one Venue.", Owner: "Event", OwnerConcept: vocab.ConceptAggregate, Anchor: application.AnchorMissing},
					},
					Specifications: []application.DomainSpecificationRef{{Name: "LateOrder", Anchor: application.AnchorMissing}},
				}},
				Unanchored: []application.UnanchoredContract{
					{Kind: application.ContractInvariant, Context: "catalog", Owner: "Event", Key: "one-venue", Statement: "An Event has one Venue.", Expected: "method OneVenue on Event"},
					{Kind: application.ContractSpecification, Context: "catalog", Name: "LateOrder", Expected: "satisfaction method on LateOrder"},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Domain struct {
			Scoped  bool `json:"scoped"`
			Located bool `json:"located"`
			Counts  struct {
				Invariants int `json:"invariants"`
			} `json:"counts"`
			Shown struct {
				Contexts   int `json:"contexts"`
				Invariants int `json:"invariants"`
			} `json:"shown"`
			Contexts []struct {
				Aggregates []struct {
					Name     string   `json:"name"`
					Identity string   `json:"identity"`
					Entities []string `json:"entities"`
				} `json:"aggregates"`
				Invariants []struct {
					Key          string `json:"key"`
					Owner        string `json:"owner"`
					OwnerConcept string `json:"ownerConcept"`
					Source       string `json:"source"`
					Anchor       string `json:"anchor"`
				} `json:"invariants"`
				Specifications []struct {
					Name   string `json:"name"`
					Anchor string `json:"anchor"`
				} `json:"specifications"`
			} `json:"contexts"`
			Unanchored []struct {
				Kind      string `json:"kind"`
				Context   string `json:"context"`
				Owner     string `json:"owner"`
				Key       string `json:"key"`
				Name      string `json:"name"`
				Statement string `json:"statement"`
				Expected  string `json:"expected"`
			} `json:"unanchored"`
		} `json:"domain"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, buf.String())
	}
	d := doc.Domain
	if !d.Scoped || !d.Located {
		t.Fatalf("scoped/located lost: %+v", d)
	}
	if d.Counts.Invariants != 3 || d.Shown.Contexts != 1 || d.Shown.Invariants != 2 {
		t.Fatalf("counts/shown = %+v / %+v", d.Counts, d.Shown)
	}
	if len(d.Contexts) != 1 || len(d.Contexts[0].Invariants) != 2 {
		t.Fatalf("contexts = %+v", d.Contexts)
	}
	if agg := d.Contexts[0].Aggregates; len(agg) != 1 || agg[0].Identity != "EventID" || len(agg[0].Entities) != 1 || agg[0].Entities[0] != "Organizer" {
		t.Fatalf("aggregates = %+v", agg)
	}
	inv := d.Contexts[0].Invariants
	if inv[0].Key != "published-frozen" || inv[0].Anchor != "found" || inv[0].Source != "event/event.go:90" || inv[0].OwnerConcept != "aggregate" {
		t.Fatalf("found invariant = %+v", inv[0])
	}
	if inv[1].Key != "one-venue" || inv[1].Anchor != "missing" || inv[1].Source != "" {
		t.Fatalf("missing invariant = %+v", inv[1])
	}
	if spec := d.Contexts[0].Specifications; len(spec) != 1 || spec[0].Anchor != "missing" {
		t.Fatalf("specifications = %+v", spec)
	}
	if len(d.Unanchored) != 2 {
		t.Fatalf("unanchored = %+v", d.Unanchored)
	}
	if u := d.Unanchored[0]; u.Kind != "invariant" || u.Context != "catalog" || u.Owner != "Event" || u.Key != "one-venue" || u.Statement == "" || u.Expected != "method OneVenue on Event" {
		t.Fatalf("unanchored[0] = %+v", u)
	}
	if u := d.Unanchored[1]; u.Kind != "specification" || u.Name != "LateOrder" || u.Owner != "" || u.Key != "" || u.Expected != "satisfaction method on LateOrder" {
		t.Fatalf("unanchored[1] = %+v", u)
	}
	raw := buf.String()
	for _, absent := range []string{`"owner":""`, `"key":""`, `"source":""`} {
		if strings.Contains(raw, absent) {
			t.Fatalf("empty %s should be omitted:\n%s", absent, raw)
		}
	}
}

func TestJSONContextOmitsScopeMarksForWholeListings(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, cli.ContextReport{
		Context: application.ArchitecturalContext{
			Scope:  "repository",
			Domain: &application.DomainKnowledge{Source: "domain.arclint.yaml", Counts: vocab.Counts{Contexts: 1}, Shown: vocab.Counts{Contexts: 1}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := buf.String()
	if strings.Contains(raw, `"scoped"`) || strings.Contains(raw, `"unanchored"`) {
		t.Fatalf("a whole, unlocated listing must carry neither scoped nor unanchored:\n%s", raw)
	}
	if !strings.Contains(raw, `"located": false`) {
		t.Fatalf("located must always be stated:\n%s", raw)
	}
}

func TestJSONShortWrite(t *testing.T) {
	err := New().Render(&shortWriter{n: 2}, cli.InitReport{Path: "rules.arclint.yaml"})
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

// Explain carries each source as an object naming the work and the
// parts that narrow it, beside the readable citation.
func TestJSONDomainExplainCarriesSources(t *testing.T) {
	var buf bytes.Buffer
	concept := vocab.ConceptAggregate.Doc()
	if len(concept.Sources) == 0 {
		t.Fatal("aggregate cites no source in the meta-model")
	}
	err := New().Render(&buf, cli.DomainExplainReport{Docs: []vocab.ConceptDoc{concept}, Single: true})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	sources, ok := doc["sources"].([]any)
	if !ok || len(sources) != len(concept.Sources) {
		t.Fatalf("sources = %v, want %d objects", doc["sources"], len(concept.Sources))
	}
	first, ok := sources[0].(map[string]any)
	if !ok {
		t.Fatalf("sources[0] = %v", sources[0])
	}
	want := concept.Sources[0]
	if first["work"] != want.Work.Key || first["title"] != want.Work.Title || first["citation"] != want.String() {
		t.Fatalf("sources[0] = %v, want work %q title %q citation %q", first, want.Work.Key, want.Work.Title, want.String())
	}
	if want.Page != "" && first["page"] != want.Page {
		t.Fatalf("page = %v, want %q", first["page"], want.Page)
	}
}

func TestJSONContextIncludesOnlyRequestedObservedDependencies(t *testing.T) {
	for _, requested := range []bool{false, true} {
		ctx := application.ArchitecturalContext{Scope: "repository"}
		if requested {
			ctx.Dependencies = &application.ObservedDependencies{
				Files: []application.DependencyFile{}, Edges: []application.DependencyImport{{SourcePath: "main.go", TargetPath: "internal/model", TargetKind: "directory", SourceZones: []string{"composition"}, TargetZones: []string{"domain", "model"}}},
				Coverage:    application.DependencyCoverage{Scope: "repository", Complete: false},
				Diagnostics: []application.DependencyDiagnostic{{Code: "IMPORTS_UNAVAILABLE", Path: "bad.go", Message: "syntax error"}},
			}
		}
		var buf bytes.Buffer
		if err := New().Render(&buf, cli.ContextReport{Context: ctx}); err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
			t.Fatal(err)
		}
		deps, present := doc["dependencies"]
		if present != requested {
			t.Fatalf("dependencies presence: %v", doc)
		}
		if requested {
			view := deps.(map[string]any)
			edge := view["edges"].([]any)[0].(map[string]any)
			if edge["targetKind"] != "directory" || edge["targetPath"] != "internal/model" || view["coverage"].(map[string]any)["complete"] != false {
				t.Fatalf("native precision or coverage lost: %v", view)
			}
		}
	}
}
