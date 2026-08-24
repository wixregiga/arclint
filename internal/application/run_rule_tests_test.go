package application_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// ruleTestConfig builds one configured repository for Rule Test runs:
// module "m" with a snake_case naming Rule and one Suppression
// covering m/BadTwo.go.
func ruleTestConfig(t *testing.T) rule.Configured {
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
	suppressed, err := rule.NewGlob("m/BadTwo.go")
	if err != nil {
		t.Fatalf("NewGlob: %v", err)
	}
	suppression, err := rule.NewSuppression([]rule.Glob{suppressed}, "known debt")
	if err != nil {
		t.Fatalf("NewSuppression: %v", err)
	}
	return rule.Configured{
		Rules:     []rule.Rule{r.Suppress(suppression)},
		Modules:   []rule.Module{module},
		Languages: []rule.Language{rule.LanguageGo},
	}
}

type ruleTestRepository struct {
	cfg rule.Configured
}

func (f ruleTestRepository) ConfiguredRules() (rule.Configured, error) { return f.cfg, nil }

type fakeRuleTestSource struct {
	tests []rule.Test
}

func (f fakeRuleTestSource) Tests() ([]rule.Test, error) { return f.tests, nil }

// fakeFixtureObserver turns each fixture file into one observed file,
// recording what the use case requested.
type fakeFixtureObserver struct {
	languages []rule.Language
	facts     []rule.Fact
}

func (f *fakeFixtureObserver) Observe(files []rule.TestFile, languages []rule.Language,
	scan rule.Scan, facts []rule.Fact,
) (conformance.Observations, error) {
	f.languages = languages
	f.facts = facts
	observed := make([]conformance.ObservedFile, 0, len(files))
	for _, file := range files {
		observed = append(observed, conformance.ObservedFile{Path: file.Path})
	}
	return conformance.NewObservations(observed, nil)
}

func mustRuleTest(t *testing.T, name, ruleID string, files []rule.TestFile,
	expect []rule.ExpectedFinding,
) rule.Test {
	t.Helper()
	rt, err := rule.NewTest(name, ruleID, files, expect)
	if err != nil {
		t.Fatalf("NewRuleTest %q: %v", name, err)
	}
	return rt
}

// fakeVocabularySource returns a configured vocabulary and records the
// content it was asked to parse.
type fakeVocabularySource struct {
	lang    vocab.UbiquitousLanguage
	err     error
	content []byte
}

func (f *fakeVocabularySource) ParseUbiquitousLanguage(content []byte) (vocab.UbiquitousLanguage, error) {
	f.content = content
	return f.lang, f.err
}

func TestNewRunRuleTestsRejectsMissingPorts(t *testing.T) {
	repo := ruleTestRepository{}
	source := fakeRuleTestSource{}
	observer := &fakeFixtureObserver{}
	vocabulary := &fakeVocabularySource{}
	if _, err := application.NewRunRuleTests(nil, source, observer, nil, vocabulary); err == nil {
		t.Errorf("nil repository accepted")
	}
	if _, err := application.NewRunRuleTests(repo, nil, observer, nil, vocabulary); err == nil {
		t.Errorf("nil test source accepted")
	}
	if _, err := application.NewRunRuleTests(repo, source, nil, nil, vocabulary); err == nil {
		t.Errorf("nil fixture observer accepted")
	}
	if _, err := application.NewRunRuleTests(repo, source, observer, nil, nil); err == nil {
		t.Errorf("nil vocabulary source accepted")
	}
}

func TestRunRuleTestsMapsAssessmentToFindings(t *testing.T) {
	cfg := ruleTestConfig(t)
	files := []rule.TestFile{
		{Path: "m/BadName.go"},
		{Path: "m/BadTwo.go"},
		{Path: "m/all_good.go"},
	}
	source := fakeRuleTestSource{tests: []rule.Test{
		mustRuleTest(t, "unknown-rule", "t:m/none", files, nil),
		mustRuleTest(t, "mapping", "t:m/snake", files, []rule.ExpectedFinding{
			{Kind: rule.FindingCoverage, Path: "m", Message: "never produced"},
		}),
		mustRuleTest(t, "conforming", "t:m/snake",
			[]rule.TestFile{{Path: "m/all_good.go"}}, nil),
	}}
	observer := &fakeFixtureObserver{}
	uc, err := application.NewRunRuleTests(ruleTestRepository{cfg}, source, observer, nil, &fakeVocabularySource{})
	if err != nil {
		t.Fatalf("NewRunRuleTests: %v", err)
	}
	results, err := uc.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("results = %d, want one per test", len(results))
	}

	unknown := results[0]
	if unknown.Name != "unknown-rule" || unknown.RuleID != "t:m/none" {
		t.Errorf("unknown identity = %q %q", unknown.Name, unknown.RuleID)
	}
	if unknown.Passed() || !strings.Contains(unknown.Err, "not configured") {
		t.Errorf("unknown rule must fail as a test failure, got Err=%q", unknown.Err)
	}

	mapping := results[1]
	if mapping.Passed() || mapping.Err != "" {
		t.Fatalf("mapping result = %+v, want a failed comparison without a test error", mapping)
	}
	if len(mapping.Missing) != 1 || mapping.Missing[0].Message != "never produced" {
		t.Errorf("Missing = %v, want the never-produced coverage expectation", mapping.Missing)
	}
	if len(mapping.Unexpected) != 2 {
		t.Fatalf("Unexpected = %v, want the active and the suppressed violation", mapping.Unexpected)
	}
	active, suppressed := mapping.Unexpected[0], mapping.Unexpected[1]
	if active.Kind != rule.FindingViolation || active.Path != "m/BadName.go" {
		t.Errorf("active mapping = %+v, want kind violation at m/BadName.go", active)
	}
	if suppressed.Kind != rule.FindingSuppressed || suppressed.Path != "m/BadTwo.go" {
		t.Errorf("suppressed mapping = %+v, want kind suppressed at m/BadTwo.go", suppressed)
	}

	conforming := results[2]
	if !conforming.Passed() {
		t.Errorf("conforming result = %+v, want a pass", conforming)
	}

	// The fixture is observed with the ruleset's languages and exactly
	// the facts the one Rule's Enforcement declares.
	if len(observer.languages) != 1 || observer.languages[0] != rule.LanguageGo {
		t.Errorf("languages = %v, want the configured [go]", observer.languages)
	}
	if len(observer.facts) != 1 || observer.facts[0] != rule.FactFileTree {
		t.Errorf("facts = %v, want exactly [file_tree]", observer.facts)
	}
}

// failingExtensions forces conformance.Run to return an error so the
// use case can prove evaluator failures stay on one RuleTestResult.
type failingExtensions struct {
	err error
}

func (f failingExtensions) Evaluate(string, map[string]any, []string,
	[]rule.Module, conformance.Observations, vocab.UbiquitousLanguage,
) ([]conformance.ExtensionFinding, error) {
	return nil, f.err
}

func TestRunRuleTestsContainsConformanceErrorAndContinues(t *testing.T) {
	cfg := ruleTestConfig(t)
	extScope, err := rule.ModuleApplicability([]rule.ModuleName{"m"})
	if err != nil {
		t.Fatalf("ModuleApplicability: %v", err)
	}
	extRule, err := rule.New(rule.Spec{
		ID:            "t:m/ext",
		Type:          rule.TypeExtension,
		Params:        rule.ExtensionParams{Uses: "broken-ext"},
		Applicability: extScope,
	})
	if err != nil {
		t.Fatalf("extension rule.New: %v", err)
	}
	cfg.Rules = append(cfg.Rules, extRule)

	files := []rule.TestFile{{Path: "m/all_good.go"}}
	source := fakeRuleTestSource{tests: []rule.Test{
		mustRuleTest(t, "extension-crash", "t:m/ext", files, nil),
		mustRuleTest(t, "still-runs", "t:m/snake", files, nil),
	}}
	uc, err := application.NewRunRuleTests(
		ruleTestRepository{cfg},
		source,
		&fakeFixtureObserver{},
		failingExtensions{err: errors.New("extension blew up")},
		&fakeVocabularySource{},
	)
	if err != nil {
		t.Fatalf("NewRunRuleTests: %v", err)
	}
	results, err := uc.Execute()
	if err != nil {
		t.Fatalf("Execute must not treat a conformance error as infrastructure: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want both tests in source order", len(results))
	}

	first, second := results[0], results[1]
	if first.Name != "extension-crash" || second.Name != "still-runs" {
		t.Fatalf("order = %q then %q, want extension-crash then still-runs",
			first.Name, second.Name)
	}
	if first.Passed() || first.Err == "" {
		t.Fatalf("first = %+v, want a contained test-level error", first)
	}
	if !strings.Contains(first.Err, "extension blew up") {
		t.Errorf("first.Err = %q, want the evaluator failure", first.Err)
	}
	if !strings.Contains(first.Err, "conformance check") {
		t.Errorf("first.Err = %q, want conformance-check context", first.Err)
	}
	if !second.Passed() {
		t.Errorf("second = %+v, want a pass after the contained failure", second)
	}
}

// recordingExtensions captures the vocabulary each evaluation received
// so tests can prove fixture knowledge reaches ctx.domain().
type recordingExtensions struct {
	knowledge vocab.UbiquitousLanguage
}

func (r *recordingExtensions) Evaluate(_ string, _ map[string]any, _ []string,
	_ []rule.Module, _ conformance.Observations, knowledge vocab.UbiquitousLanguage,
) ([]conformance.ExtensionFinding, error) {
	r.knowledge = knowledge
	return nil, nil
}

func TestRunRuleTestsFeedsFixtureVocabularyToExtensions(t *testing.T) {
	cfg := ruleTestConfig(t)
	extScope, err := rule.ModuleApplicability([]rule.ModuleName{"m"})
	if err != nil {
		t.Fatalf("ModuleApplicability: %v", err)
	}
	extRule, err := rule.New(rule.Spec{
		ID:            "t:m/ext",
		Type:          rule.TypeExtension,
		Params:        rule.ExtensionParams{Uses: "domain-probe"},
		Applicability: extScope,
	})
	if err != nil {
		t.Fatalf("extension rule.New: %v", err)
	}
	cfg.Rules = append(cfg.Rules, extRule)

	const authored = "version: 1\ncontexts:\n  - name: Ordering\n    entities:\n      - name: Order\n"
	recorded := vocab.UbiquitousLanguage{Contexts: []vocab.BoundedContext{{
		Name:     "Ordering",
		Entities: []vocab.Entity{{Definition: vocab.Definition{Name: "Order"}}},
	}}}
	files := []rule.TestFile{
		{Path: "m/all_good.go"},
		{Path: vocab.UbiquitousLanguageFileName, Content: authored},
	}
	source := fakeRuleTestSource{tests: []rule.Test{
		mustRuleTest(t, "sees-vocabulary", "t:m/ext", files, nil),
	}}
	vocabulary := &fakeVocabularySource{lang: recorded}
	recorder := &recordingExtensions{}
	uc, err := application.NewRunRuleTests(
		ruleTestRepository{cfg}, source, &fakeFixtureObserver{}, recorder, vocabulary)
	if err != nil {
		t.Fatalf("NewRunRuleTests: %v", err)
	}
	results, err := uc.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(results) != 1 || !results[0].Passed() {
		t.Fatalf("results = %+v, want one pass", results)
	}
	if string(vocabulary.content) != authored {
		t.Errorf("parsed content = %q, want the authored fixture bytes", vocabulary.content)
	}
	if len(recorder.knowledge.Contexts) != 1 ||
		len(recorder.knowledge.Contexts[0].Entities) != 1 ||
		recorder.knowledge.Contexts[0].Entities[0].Name != "Order" {
		t.Errorf("evaluator knowledge = %+v, want the parsed Order entity", recorder.knowledge)
	}
}

func TestRunRuleTestsContainsFixtureVocabularyError(t *testing.T) {
	cfg := ruleTestConfig(t)
	files := []rule.TestFile{
		{Path: "m/all_good.go"},
		{Path: vocab.UbiquitousLanguageFileName, Content: "version: 2\n"},
	}
	source := fakeRuleTestSource{tests: []rule.Test{
		mustRuleTest(t, "broken-vocabulary", "t:m/snake", files, nil),
		mustRuleTest(t, "still-runs", "t:m/snake", []rule.TestFile{{Path: "m/all_good.go"}}, nil),
	}}
	vocabulary := &fakeVocabularySource{err: errors.New("unsupported version 2")}
	uc, err := application.NewRunRuleTests(
		ruleTestRepository{cfg}, source, &fakeFixtureObserver{}, nil, vocabulary)
	if err != nil {
		t.Fatalf("NewRunRuleTests: %v", err)
	}
	results, err := uc.Execute()
	if err != nil {
		t.Fatalf("Execute must contain a fixture vocabulary failure: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want both tests", len(results))
	}
	if results[0].Passed() || !strings.Contains(results[0].Err, "fixture vocabulary") {
		t.Errorf("first = %+v, want a contained fixture-vocabulary error", results[0])
	}
	if !results[1].Passed() {
		t.Errorf("second = %+v, want a pass after the contained failure", results[1])
	}
}
