package application_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/baseline"
	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// fixture builds one configured repository: module "m" with a
// snake_case naming Rule, observed with the given file paths.
func fixture(t *testing.T, paths ...string) (rule.Configured, conformance.Observations) {
	t.Helper()
	glob, err := rule.NewGlob("m/**")
	if err != nil {
		t.Fatalf("NewGlob: %v", err)
	}
	module, err := rule.NewModule("m", "test module", []rule.Glob{glob})
	if err != nil {
		t.Fatalf("NewModule: %v", err)
	}
	snake, err := rule.NewCaseSpec("snake_case")
	if err != nil {
		t.Fatalf("NewCaseSpec: %v", err)
	}
	scope, err := rule.ModuleApplicability([]rule.ModuleName{"m"})
	if err != nil {
		t.Fatalf("ModuleApplicability: %v", err)
	}
	r, err := rule.New(rule.Spec{
		ID:            "t:m/snake",
		Type:          rule.TypeNaming,
		Params:        rule.NamingParams{Case: snake},
		Applicability: scope,
	})
	if err != nil {
		t.Fatalf("rule.New: %v", err)
	}
	files := make([]conformance.ObservedFile, 0, len(paths))
	for _, p := range paths {
		files = append(files, conformance.ObservedFile{Path: p})
	}
	obs, err := conformance.NewObservations(files, nil)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}
	cfg := rule.Configured{
		Rules:     []rule.Rule{r},
		Modules:   []rule.Module{module},
		Languages: []rule.Language{rule.LanguageGo},
	}
	return cfg, obs
}

type fakeRepository struct {
	cfg rule.Configured
}

func (f fakeRepository) ConfiguredRules() (rule.Configured, error) { return f.cfg, nil }

type fakeObservations struct {
	obs       conformance.Observations
	languages []rule.Language
	facts     []rule.Fact
}

func (f *fakeObservations) Observe(languages []rule.Language, scan rule.Scan, facts []rule.Fact) (conformance.Observations, error) {
	f.languages = languages
	f.facts = facts
	return f.obs, nil
}

type fakeBaselineSource struct {
	snapshot baseline.Snapshot
	present  bool
	loads    int
}

func (f *fakeBaselineSource) Load() (baseline.Snapshot, bool, error) {
	f.loads++
	return f.snapshot, f.present, nil
}

type fakeBaselineOutput struct {
	written []baseline.Snapshot
}

func (f *fakeBaselineOutput) Write(s baseline.Snapshot) error {
	f.written = append(f.written, s)
	return nil
}

// fakeKnowledge is an in-memory vocab.Repository for application tests.
type fakeKnowledge struct {
	lang  vocab.UbiquitousLanguage
	found bool
	err   error
	saves int
	last  vocab.UbiquitousLanguage
}

func (f *fakeKnowledge) RecordedLanguage() (vocab.UbiquitousLanguage, bool, error) {
	if f.err != nil {
		return vocab.UbiquitousLanguage{}, false, f.err
	}
	return f.lang, f.found, nil
}

func (f *fakeKnowledge) Record(lang vocab.UbiquitousLanguage) error {
	f.saves++
	f.last = lang
	f.lang = lang
	f.found = true
	return nil
}

func emptyKnowledge() *fakeKnowledge { return &fakeKnowledge{} }

func newAssess(t *testing.T, cfg rule.Configured, obs conformance.Observations,
	baselines *fakeBaselineSource,
) application.AssessConformance {
	t.Helper()
	uc, err := application.NewAssessConformance(fakeRepository{cfg}, &fakeObservations{obs: obs}, baselines, nil, emptyKnowledge())
	if err != nil {
		t.Fatalf("NewAssessConformance: %v", err)
	}
	return uc
}

func TestAssessConformanceRequestsDeclaredFacts(t *testing.T) {
	cfg, obs := fixture(t, "m/ok.go")
	observations := &fakeObservations{obs: obs}
	uc, err := application.NewAssessConformance(fakeRepository{cfg}, observations, &fakeBaselineSource{}, nil, emptyKnowledge())
	if err != nil {
		t.Fatalf("NewAssessConformance: %v", err)
	}
	if _, err := uc.Execute(application.AssessConformanceRequest{}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// The fixture's naming Rule declares file_tree facts and nothing
	// else: observation must be asked for exactly that.
	if len(observations.facts) != 1 || observations.facts[0] != rule.FactFileTree {
		t.Errorf("requested facts = %v, want exactly [file_tree]", observations.facts)
	}
}

func TestListRulesSummarizes(t *testing.T) {
	cfg, _ := fixture(t, "m/ok.go")
	disablement, err := rule.NewDisablement("retired")
	if err != nil {
		t.Fatalf("NewDisablement: %v", err)
	}
	cfg.Rules[0] = cfg.Rules[0].Disable(disablement)
	uc, err := application.NewListRules(fakeRepository{cfg})
	if err != nil {
		t.Fatalf("NewListRules: %v", err)
	}
	rows, err := uc.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.ID != "t:m/snake" || row.Type != "naming" || row.Severity != "error" {
		t.Errorf("row = %+v", row)
	}
	if !strings.Contains(row.Claim, "snake_case") {
		t.Errorf("claim %q lacks the case vocabulary", row.Claim)
	}
	if !row.Disabled || row.DisabledReason != "retired" {
		t.Errorf("disablement not surfaced: %+v", row)
	}
}

func TestAssessConformanceAppliesBaseline(t *testing.T) {
	cfg, obs := fixture(t, "m/BadName.go", "m/ok.go")

	baselines := &fakeBaselineSource{}
	uc := newAssess(t, cfg, obs, baselines)
	first, err := uc.Execute(application.AssessConformanceRequest{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(first.ActiveViolations()) != 1 || !first.HasErrors() {
		t.Fatalf("expected one gating violation, got %d", len(first.ActiveViolations()))
	}

	snapshot, err := baseline.Capture(first, "")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	covered := newAssess(t, cfg, obs, &fakeBaselineSource{snapshot: snapshot, present: true})
	second, err := covered.Execute(application.AssessConformanceRequest{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(second.ActiveViolations()) != 0 || len(second.BaselinedViolations()) != 1 || second.HasErrors() {
		t.Errorf("baseline not applied: active %d baselined %d",
			len(second.ActiveViolations()), len(second.BaselinedViolations()))
	}

	if _, err := uc.Execute(application.AssessConformanceRequest{SkipBaseline: true}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if baselines.loads != 1 {
		t.Errorf("baseline loads = %d; SkipBaseline must not consult the source", baselines.loads)
	}
}

func TestAssessConformanceSurfacesStaleBaseline(t *testing.T) {
	cfg, obs := fixture(t, "m/BadName.go", "m/ok.go")
	before, err := newAssess(t, cfg, obs, &fakeBaselineSource{}).
		Execute(application.AssessConformanceRequest{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	snapshot, err := baseline.Capture(before, "")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	fixedCfg, fixedObs := fixture(t, "m/good_name.go", "m/ok.go")
	after, err := newAssess(t, fixedCfg, fixedObs, &fakeBaselineSource{snapshot: snapshot, present: true}).
		Execute(application.AssessConformanceRequest{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	found := false
	for _, d := range after.Diagnostics() {
		if d.Kind() == conformance.DiagnosticCoverage && strings.Contains(d.Message(), "no longer occur") {
			found = true
		}
	}
	if !found {
		t.Errorf("stale baseline entries must surface as a coverage diagnostic")
	}
}

func TestCaptureBaselinePersistsSnapshot(t *testing.T) {
	cfg, obs := fixture(t, "m/BadName.go", "m/ok.go")
	output := &fakeBaselineOutput{}
	assess := newAssess(t, cfg, obs, &fakeBaselineSource{})
	uc, err := application.NewCaptureBaseline(assess, output)
	if err != nil {
		t.Fatalf("NewCaptureBaseline: %v", err)
	}
	result, err := uc.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Findings != 1 || result.Rules != 1 {
		t.Errorf("result = %+v, want 1 finding from 1 rule", result)
	}
	if len(output.written) != 1 {
		t.Fatalf("snapshots written = %d, want 1", len(output.written))
	}
	if got := len(output.written[0].Entries()); got != 1 {
		t.Errorf("persisted entries = %d, want 1", got)
	}
}

func TestShowRule(t *testing.T) {
	cfg, _ := fixture(t, "m/ok.go")
	glob, err := rule.NewGlob("m/legacy/**")
	if err != nil {
		t.Fatalf("NewGlob: %v", err)
	}
	exclusion, err := rule.NewExclusion([]rule.Glob{glob}, nil, "adopted as-is")
	if err != nil {
		t.Fatalf("NewExclusion: %v", err)
	}
	cfg.Rules[0] = cfg.Rules[0].Exclude(exclusion)

	show, err := application.NewShowRule(fakeRepository{cfg})
	if err != nil {
		t.Fatalf("NewShowRule: %v", err)
	}
	detail, err := show.Execute("t:m/snake")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if detail.Summary.ID != "t:m/snake" || len(detail.Modules) != 1 || detail.Modules[0] != "m" {
		t.Errorf("detail = %+v", detail)
	}
	if len(detail.Exclusions) != 1 || detail.Exclusions[0].Reason != "adopted as-is" {
		t.Errorf("exclusions = %+v", detail.Exclusions)
	}
	if !strings.Contains(detail.Schema, "rule type naming") {
		t.Errorf("schema description missing: %q", detail.Schema)
	}
	if _, err := show.Execute("t:m/ghost"); err == nil {
		t.Errorf("unknown rule id must be an error")
	}
}

func TestRefreshBaselineDropsStaleEntries(t *testing.T) {
	brokenCfg, brokenObs := fixture(t, "m/BadName.go", "m/ok.go")
	before, err := newAssess(t, brokenCfg, brokenObs, &fakeBaselineSource{}).
		Execute(application.AssessConformanceRequest{SkipBaseline: true})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	snapshot, err := baseline.Capture(before, "")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}

	fixedCfg, fixedObs := fixture(t, "m/good_name.go", "m/ok.go")
	source := &fakeBaselineSource{snapshot: snapshot, present: true}
	output := &fakeBaselineOutput{}
	refresh, err := application.NewRefreshBaseline(
		newAssess(t, fixedCfg, fixedObs, source), source, output)
	if err != nil {
		t.Fatalf("NewRefreshBaseline: %v", err)
	}
	result, err := refresh.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.RemovedStale != 1 || result.Findings != 0 {
		t.Errorf("result = %+v, want the stale finding dropped and none re-adopted", result)
	}
	if len(output.written) != 1 || len(output.written[0].Entries()) != 0 {
		t.Errorf("replacement snapshot = %+v, want empty", output.written)
	}

	missing, err := application.NewRefreshBaseline(
		newAssess(t, fixedCfg, fixedObs, &fakeBaselineSource{}), &fakeBaselineSource{}, output)
	if err != nil {
		t.Fatalf("NewRefreshBaseline: %v", err)
	}
	if _, err := missing.Execute(); err == nil {
		t.Errorf("refreshing a nonexistent baseline must be an error, never an implicit capture")
	}
}

type fakePatternSource struct {
	patterns []rule.Pattern
}

func (f fakePatternSource) Patterns() ([]rule.Pattern, error) { return f.patterns, nil }

func TestListPatternsSummarizes(t *testing.T) {
	cfg, _ := fixture(t, "m/ok.go")
	p, err := rule.NewPattern("arclint", "ddd-flat", "1.2.3", cfg.Rules, nil, []rule.Language{rule.LanguageGo})
	if err != nil {
		t.Fatalf("NewPattern: %v", err)
	}
	uc, err := application.NewListPatterns(fakePatternSource{[]rule.Pattern{p}})
	if err != nil {
		t.Fatalf("NewListPatterns: %v", err)
	}
	rows, err := uc.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := application.PatternSummary{
		Namespace: "arclint", Name: "ddd-flat",
		Version: "1.2.3", Rules: 1, Extensions: 0, Coverage: []string{"go"},
	}
	if len(rows) != 1 || rows[0].Namespace != want.Namespace || rows[0].Name != want.Name ||
		rows[0].Version != want.Version || rows[0].Rules != want.Rules ||
		rows[0].Extensions != want.Extensions ||
		len(rows[0].Coverage) != 1 || rows[0].Coverage[0] != "go" {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}
