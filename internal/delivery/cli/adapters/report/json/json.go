// Package jsonreport renders sealed CLI reports as JSON documents,
// preserving the historical field names and shapes for check and domain
// products. New machine formats are adapter-owned lowerCamel documents.
package jsonreport

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// renderer is the sealed JSON report adapter.
type renderer struct{}

// New returns a cli.Renderer that writes indented JSON documents.
func New() cli.Renderer { return renderer{} }

// Render writes one sealed Report as JSON to w.
func (renderer) Render(w io.Writer, r cli.Report) error {
	var doc any
	switch x := r.(type) {
	case cli.AssessmentReport:
		doc = checkDocs(x.Assessment)
	case cli.RuleListReport:
		doc = ruleSummaryDocs(x.Rules)
	case cli.RuleDetailReport:
		doc = ruleDetailDoc(x.Detail)
	case cli.RuleTestReport:
		doc = ruleTestDocs(x.Results)
	case cli.InitReport:
		doc = initDoc{Path: x.Path}
	case cli.ContextReport:
		doc = contextDoc(x.Context)
	case cli.DomainInitReport:
		doc = domainInitDoc{Source: x.Result.Source, Created: x.Result.Created}
	case cli.DomainMissingReport:
		doc = map[string]any{
			"found":  false,
			"source": vocab.UbiquitousLanguageFileName,
		}
	case cli.DomainOverviewReport:
		doc = overviewJSONDoc(x.Overview)
	case cli.DomainListReport:
		doc = listJSONDoc(x.Listing)
	case cli.DomainShowReport:
		doc = showJSONDoc(x.View)
	case cli.DomainExplainReport:
		if x.Single && len(x.Docs) == 1 {
			doc = explainJSONDoc(x.Docs[0])
		} else {
			items := make([]any, 0, len(x.Docs))
			for _, d := range x.Docs {
				items = append(items, explainJSONDoc(d))
			}
			doc = items
		}
	case cli.DomainDefineReport:
		doc = defineJSONDoc(x)
	case cli.DomainRemoveReport:
		doc = removeJSONDoc(x.Result)
	case cli.AgentsStatusReport:
		doc = artifactWriteDocs(x.Writes)
	case cli.BaselineCaptureReport:
		doc = baselineCaptureDoc{
			Findings: x.Result.Findings,
			Rules:    x.Result.Rules,
		}
	case cli.BaselineRefreshReport:
		doc = baselineRefreshDoc{
			Findings:     x.Result.Findings,
			Rules:        x.Result.Rules,
			RemovedStale: x.Result.RemovedStale,
		}
	case cli.PatternsReport:
		doc = patternDocs(x.Patterns)
	case cli.SDKInitReport:
		doc = sdkInitDoc{Paths: x.Paths}
	default:
		return fmt.Errorf("report: unknown type %T", r)
	}
	return writeJSON(w, doc)
}

func writeJSON(w io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	data = append(data, '\n')
	if err := out.WriteFull(w, data); err != nil {
		return fmt.Errorf("write json: %w", err)
	}
	return nil
}

// diagnosticDoc is the stable JSON shape of one Diagnostic in the
// target engine's output.
type diagnosticDoc struct {
	Kind        string `json:"kind"`
	RuleID      string `json:"ruleId,omitempty"`
	Pattern     string `json:"pattern,omitempty"`
	Path        string `json:"path,omitempty"`
	Line        int    `json:"line,omitempty"`
	Severity    string `json:"severity,omitempty"`
	Status      string `json:"status,omitempty"`
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
}

func checkDocs(a conformance.Assessment) []diagnosticDoc {
	diags := a.Diagnostics()
	docs := make([]diagnosticDoc, 0, len(diags))
	for _, d := range diags {
		docs = append(docs, diagnosticDoc{
			Kind:        string(d.Kind()),
			RuleID:      d.RuleID(),
			Pattern:     d.Pattern(),
			Path:        d.Path(),
			Line:        d.Line(),
			Severity:    string(d.Severity()),
			Status:      string(d.Status()),
			Message:     d.Message(),
			Remediation: d.Remediation(),
		})
	}
	return docs
}

// --- newly JSON-enabled simple result reports (lowerCamel) ---

type initDoc struct {
	Path string `json:"path"`
}

type domainInitDoc struct {
	Source  string `json:"source"`
	Created bool   `json:"created"`
}

type artifactWriteDoc struct {
	Changed bool   `json:"changed"`
	Path    string `json:"path"`
}

func artifactWriteDocs(writes []cli.ArtifactWrite) []artifactWriteDoc {
	out := make([]artifactWriteDoc, 0, len(writes))
	for _, w := range writes {
		out = append(out, artifactWriteDoc{Changed: w.Changed, Path: w.Path})
	}
	return out
}

type baselineCaptureDoc struct {
	Findings int `json:"findings"`
	Rules    int `json:"rules"`
}

type baselineRefreshDoc struct {
	Findings     int `json:"findings"`
	Rules        int `json:"rules"`
	RemovedStale int `json:"removedStale"`
}

type patternDoc struct {
	Namespace  string   `json:"namespace"`
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	Rules      int      `json:"rules"`
	Extensions int      `json:"extensions"`
	Coverage   []string `json:"coverage,omitempty"`
}

func patternDocs(rows []application.PatternSummary) []patternDoc {
	out := make([]patternDoc, 0, len(rows))
	for _, row := range rows {
		out = append(out, patternDoc{
			Namespace:  row.Namespace,
			Name:       row.Name,
			Version:    row.Version,
			Rules:      row.Rules,
			Extensions: row.Extensions,
			Coverage:   append([]string(nil), row.Coverage...),
		})
	}
	return out
}

type sdkInitDoc struct {
	Paths []string `json:"paths"`
}

// --- rules (lowerCamel) ---

type ruleSummaryDoc struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	Severity       string `json:"severity"`
	Claim          string `json:"claim"`
	Assurance      string `json:"assurance"`
	Provenance     string `json:"provenance,omitempty"`
	Disabled       bool   `json:"disabled,omitempty"`
	DisabledReason string `json:"disabledReason,omitempty"`
}

func ruleSummaryDocOf(s application.RuleSummary) ruleSummaryDoc {
	return ruleSummaryDoc{
		ID:             s.ID,
		Type:           s.Type,
		Severity:       s.Severity,
		Claim:          s.Claim,
		Assurance:      s.Assurance,
		Provenance:     s.Provenance,
		Disabled:       s.Disabled,
		DisabledReason: s.DisabledReason,
	}
}

func ruleSummaryDocs(rows []application.RuleSummary) []ruleSummaryDoc {
	out := make([]ruleSummaryDoc, 0, len(rows))
	for _, row := range rows {
		out = append(out, ruleSummaryDocOf(row))
	}
	return out
}

type policyNoteDoc struct {
	Selectors []string `json:"selectors"`
	Reason    string   `json:"reason"`
}

type ruleDetailDocT struct {
	Summary          ruleSummaryDoc  `json:"summary"`
	Evidence         string          `json:"evidence,omitempty"`
	Languages        []string        `json:"languages,omitempty"`
	Facts            []string        `json:"facts,omitempty"`
	Limitations      []string        `json:"limitations,omitempty"`
	EntireRepository bool            `json:"entireRepository,omitempty"`
	Modules          []string        `json:"modules,omitempty"`
	Files            []string        `json:"files,omitempty"`
	Exclusions       []policyNoteDoc `json:"exclusions,omitempty"`
	Suppressions     []policyNoteDoc `json:"suppressions,omitempty"`
	Schema           string          `json:"schema,omitempty"`
}

func ruleDetailDoc(d application.RuleDetail) ruleDetailDocT {
	doc := ruleDetailDocT{
		Summary:          ruleSummaryDocOf(d.Summary),
		Evidence:         d.Evidence,
		Languages:        append([]string(nil), d.Languages...),
		Facts:            append([]string(nil), d.Facts...),
		Limitations:      append([]string(nil), d.Limitations...),
		EntireRepository: d.EntireRepository,
		Modules:          append([]string(nil), d.Modules...),
		Files:            append([]string(nil), d.Files...),
		Schema:           d.Schema,
	}
	for _, e := range d.Exclusions {
		doc.Exclusions = append(doc.Exclusions, policyNoteDoc{
			Selectors: append([]string(nil), e.Selectors...),
			Reason:    e.Reason,
		})
	}
	for _, s := range d.Suppressions {
		doc.Suppressions = append(doc.Suppressions, policyNoteDoc{
			Selectors: append([]string(nil), s.Selectors...),
			Reason:    s.Reason,
		})
	}
	return doc
}

type findingDoc struct {
	Kind    string `json:"kind"`
	Path    string `json:"path,omitempty"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message,omitempty"`
}

type ruleTestDoc struct {
	Name       string       `json:"name"`
	RuleID     string       `json:"ruleId"`
	Passed     bool         `json:"passed"`
	Missing    []findingDoc `json:"missing,omitempty"`
	Unexpected []findingDoc `json:"unexpected,omitempty"`
	Err        string       `json:"error,omitempty"`
}

func ruleTestDocs(results []application.RuleTestResult) []ruleTestDoc {
	out := make([]ruleTestDoc, 0, len(results))
	for _, r := range results {
		doc := ruleTestDoc{
			Name:   r.Name,
			RuleID: r.RuleID,
			Passed: r.Passed(),
			Err:    r.Err,
		}
		for _, m := range r.Missing {
			doc.Missing = append(doc.Missing, findingDoc{
				Kind:    string(m.Kind),
				Path:    m.Path,
				Line:    m.Line,
				Message: m.Message,
			})
		}
		for _, u := range r.Unexpected {
			doc.Unexpected = append(doc.Unexpected, findingDoc{
				Kind:    string(u.Kind),
				Path:    u.Path,
				Line:    u.Line,
				Message: u.Message,
			})
		}
		out = append(out, doc)
	}
	return out
}

// --- context: preserve established field names from prior direct marshal ---

// Established context JSON used Go field names for application structs
// that lacked tags (PascalCase), with nested domain already lowerCamel.
type contextJSON struct {
	Scope          string               `json:"Scope"`
	Languages      []string             `json:"Languages"`
	RuleCount      int                  `json:"RuleCount"`
	Modules        []modulePolicyJSON   `json:"Modules"`
	Rules          []appliedRuleJSON    `json:"Rules"`
	Paths          []pathBindingJSON    `json:"Paths"`
	Kinds          []kindInUseJSON      `json:"Kinds"`
	UnknownImports string               `json:"UnknownImports"`
	Domain         *domainKnowledgeJSON `json:"domain,omitempty"`
}

type pathBindingJSON struct {
	Path    string   `json:"Path"`
	Modules []string `json:"Modules"`
}

type kindInUseJSON struct {
	Kind    string `json:"Kind"`
	Meaning string `json:"Meaning"`
}

type modulePolicyJSON struct {
	Name               string   `json:"Name"`
	Description        string   `json:"Description"`
	Paths              []string `json:"Paths"`
	Internal           []string `json:"Internal"`
	InternalRestricted bool     `json:"InternalRestricted"`
	External           string   `json:"External"`
	Stdlib             string   `json:"Stdlib"`
}

type appliedRuleJSON struct {
	Summary ruleSummaryPassthrough `json:"Summary"`
	Reason  string                 `json:"Reason"`
	Via     []string               `json:"Via"`
}

// ruleSummaryPassthrough matches the established untagged RuleSummary marshal.
type ruleSummaryPassthrough struct {
	ID             string `json:"ID"`
	Type           string `json:"Type"`
	Severity       string `json:"Severity"`
	Claim          string `json:"Claim"`
	Assurance      string `json:"Assurance"`
	Provenance     string `json:"Provenance"`
	Disabled       bool   `json:"Disabled"`
	DisabledReason string `json:"DisabledReason"`
}

type domainKnowledgeJSON struct {
	Source    string                   `json:"source"`
	Counts    domainCountsPassthrough  `json:"counts"`
	Contexts  []domainContextKnowJSON  `json:"contexts,omitempty"`
	Relations []domainRelationKnowJSON `json:"relations,omitempty"`
}

// domainCountsPassthrough matches untagged vocab.Counts under context domain.
type domainCountsPassthrough struct {
	Contexts       int `json:"Contexts"`
	Entities       int `json:"Entities"`
	Aggregates     int `json:"Aggregates"`
	ValueObjects   int `json:"ValueObjects"`
	Invariants     int `json:"Invariants"`
	Assertions     int `json:"Assertions"`
	Specifications int `json:"Specifications"`
	Events         int `json:"Events"`
	Relations      int `json:"Relations"`
}

type domainEntityKnowJSON struct {
	Name      string `json:"name"`
	Aggregate bool   `json:"aggregate,omitempty"`
}

type domainInvariantKnowJSON struct {
	Statement string `json:"statement"`
	Owner     string `json:"owner"`
	ID        string `json:"id,omitempty"`
	Source    string `json:"source,omitempty"`
}

type domainAssertionKnowJSON struct {
	Statement string `json:"statement"`
	Owner     string `json:"owner"`
	ID        string `json:"id"`
	On        string `json:"on"`
	Source    string `json:"source,omitempty"`
}

type domainSpecificationKnowJSON struct {
	Name   string `json:"name"`
	Source string `json:"source,omitempty"`
}

type domainRelationKnowJSON struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type domainContextKnowJSON struct {
	Name           string                        `json:"name"`
	Entities       []domainEntityKnowJSON        `json:"entities,omitempty"`
	ValueObjects   []string                      `json:"valueObjects,omitempty"`
	Invariants     []domainInvariantKnowJSON     `json:"invariants,omitempty"`
	Assertions     []domainAssertionKnowJSON     `json:"assertions,omitempty"`
	Specifications []domainSpecificationKnowJSON `json:"specifications,omitempty"`
	Events         []string                      `json:"events,omitempty"`
}

func contextDoc(c application.ArchitecturalContext) contextJSON {
	doc := contextJSON{
		Scope:          c.Scope,
		Languages:      append([]string(nil), c.Languages...),
		RuleCount:      c.RuleCount,
		UnknownImports: c.UnknownImports,
	}
	for _, m := range c.Modules {
		doc.Modules = append(doc.Modules, modulePolicyJSON{
			Name:               m.Name,
			Description:        m.Description,
			Paths:              append([]string(nil), m.Paths...),
			Internal:           append([]string(nil), m.Internal...),
			InternalRestricted: m.InternalRestricted,
			External:           m.External,
			Stdlib:             m.Stdlib,
		})
	}
	for _, r := range c.Rules {
		doc.Rules = append(doc.Rules, appliedRuleJSON{
			Summary: ruleSummaryPassthrough{
				ID:             r.Summary.ID,
				Type:           r.Summary.Type,
				Severity:       r.Summary.Severity,
				Claim:          r.Summary.Claim,
				Assurance:      r.Summary.Assurance,
				Provenance:     r.Summary.Provenance,
				Disabled:       r.Summary.Disabled,
				DisabledReason: r.Summary.DisabledReason,
			},
			Reason: r.Reason,
			Via:    append([]string(nil), r.Via...),
		})
	}
	for _, p := range c.Paths {
		doc.Paths = append(doc.Paths, pathBindingJSON{
			Path:    p.Path,
			Modules: append([]string(nil), p.Modules...),
		})
	}
	for _, k := range c.Kinds {
		doc.Kinds = append(doc.Kinds, kindInUseJSON{Kind: k.Kind, Meaning: k.Meaning})
	}
	if c.Domain != nil {
		d := c.Domain
		dk := &domainKnowledgeJSON{
			Source: d.Source,
			Counts: domainCountsPassthrough{
				Contexts:       d.Counts.Contexts,
				Entities:       d.Counts.Entities,
				Aggregates:     d.Counts.Aggregates,
				ValueObjects:   d.Counts.ValueObjects,
				Invariants:     d.Counts.Invariants,
				Assertions:     d.Counts.Assertions,
				Specifications: d.Counts.Specifications,
				Events:         d.Counts.Events,
				Relations:      d.Counts.Relations,
			},
		}
		for _, ctx := range d.Contexts {
			entry := domainContextKnowJSON{
				Name:         ctx.Name,
				ValueObjects: append([]string(nil), ctx.ValueObjects...),
				Events:       append([]string(nil), ctx.Events...),
			}
			for _, e := range ctx.Entities {
				entry.Entities = append(entry.Entities, domainEntityKnowJSON{Name: e.Name, Aggregate: e.Aggregate})
			}
			for _, inv := range ctx.Invariants {
				entry.Invariants = append(entry.Invariants, domainInvariantKnowJSON{
					Statement: inv.Statement, Owner: inv.Owner, ID: inv.ID, Source: inv.Source,
				})
			}
			for _, a := range ctx.Assertions {
				entry.Assertions = append(entry.Assertions, domainAssertionKnowJSON{
					Statement: a.Statement, Owner: a.Owner, ID: a.ID, On: a.On, Source: a.Source,
				})
			}
			for _, s := range ctx.Specifications {
				entry.Specifications = append(entry.Specifications, domainSpecificationKnowJSON{
					Name: s.Name, Source: s.Source,
				})
			}
			dk.Contexts = append(dk.Contexts, entry)
		}
		for _, rel := range d.Relations {
			dk.Relations = append(dk.Relations, domainRelationKnowJSON{From: rel.From, To: rel.To, Kind: rel.Kind})
		}
		doc.Domain = dk
	}
	return doc
}
