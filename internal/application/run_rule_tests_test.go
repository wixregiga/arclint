package application_test

import (
	"errors"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// ruleTestConfig builds one configured repository for Rule Test runs:
// zone "m" with a snake_case naming Rule and one Suppression
// covering m/BadTwo.go.
func ruleTestConfig(t *testing.T) rule.Configured {
	t.Helper()
	glob, err := rule.NewGlob("m/**")
	if err != nil {
		t.Fatalf("NewGlob: %v", err)
	}
	zone, err := rule.NewZone("m", "test zone", []rule.Glob{glob})
	if err != nil {
		t.Fatalf("NewZone: %v", err)
	}
	snake, err := rule.NewCaseSpec("snake_case")
	if err != nil {
		t.Fatalf("NewCaseSpec: %v", err)
	}
	scope, err := rule.ZoneScope([]rule.ZoneName{"m"})
	if err != nil {
		t.Fatalf("ZoneScope: %v", err)
	}
	r, err := rule.New(rule.Spec{
		ID:     "t/p:m/snake",
		Type:   rule.TypeNaming,
		Params: rule.NamingParams{Case: snake},
		Scope:  scope,
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
		Zones:     []rule.Zone{zone},
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

// fakeFixtureObserver turns each fixture file into one observed file at
// its cleaned path, where the real observer materializes it, recording
// what the use case requested.
type fakeFixtureObserver struct {
	languages  []rule.Language
	facts      []rule.Fact
	namedTests []string
}

func (f *fakeFixtureObserver) Observe(files []rule.TestFile, languages []rule.Language,
	scan rule.Scan, facts []rule.Fact, namedTests []string,
) (conformance.Observations, error) {
	f.languages = languages
	f.facts = facts
	f.namedTests = namedTests
	observed := make([]conformance.ObservedFile, 0, len(files))
	for _, file := range files {
		observed = append(observed, conformance.ObservedFile{Path: path.Clean(file.Path)})
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
	lang      vocab.UbiquitousLanguage
	err       error
	content   []byte
	recording vocab.DomainRecording
	parsed    bool
}

func (f *fakeVocabularySource) ParseUbiquitousLanguage(content []byte, recording vocab.DomainRecording) (vocab.UbiquitousLanguage, error) {
	f.content = content
	f.recording = recording
	f.parsed = true
	return f.lang, f.err
}

// fakeRecordingSource names the DomainRecording of whatever ruleset
// content it is given, and keeps that content.
type fakeRecordingSource struct {
	recording vocab.DomainRecording
	err       error
	content   []byte
}

func (f *fakeRecordingSource) ParseRulesetRecording(content []byte) (vocab.DomainRecording, error) {
	f.content = content
	return f.recording, f.err
}

func TestNewRunRuleTestsRejectsMissingPorts(t *testing.T) {
	repo := ruleTestRepository{}
	source := fakeRuleTestSource{}
	observer := &fakeFixtureObserver{}
	vocabulary := &fakeVocabularySource{}
	recordings := &fakeRecordingSource{}
	if _, err := application.NewRunRuleTests(nil, source, observer, nil, vocabulary, recordings); err == nil {
		t.Errorf("nil repository accepted")
	}
	if _, err := application.NewRunRuleTests(repo, nil, observer, nil, vocabulary, recordings); err == nil {
		t.Errorf("nil test source accepted")
	}
	if _, err := application.NewRunRuleTests(repo, source, nil, nil, vocabulary, recordings); err == nil {
		t.Errorf("nil fixture observer accepted")
	}
	if _, err := application.NewRunRuleTests(repo, source, observer, nil, nil, recordings); err == nil {
		t.Errorf("nil vocabulary source accepted")
	}
	if _, err := application.NewRunRuleTests(repo, source, observer, nil, vocabulary, nil); err == nil {
		t.Errorf("nil ruleset recording source accepted")
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
		mustRuleTest(t, "unknown-rule", "t/p:m/none", files, nil),
		mustRuleTest(t, "mapping", "t/p:m/snake", files, []rule.ExpectedFinding{
			{Kind: rule.FindingCoverage, Path: "m", Message: "never produced"},
		}),
		mustRuleTest(t, "conforming", "t/p:m/snake",
			[]rule.TestFile{{Path: "m/all_good.go"}}, nil),
	}}
	observer := &fakeFixtureObserver{}
	uc, err := application.NewRunRuleTests(ruleTestRepository{cfg}, source, observer, nil, &fakeVocabularySource{}, &fakeRecordingSource{})
	if err != nil {
		t.Fatalf("NewRunRuleTests: %v", err)
	}
	run, err := uc.Execute()
	results := run.Results
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("results = %d, want one per test", len(results))
	}

	unknown := results[0]
	if unknown.Name != "unknown-rule" || unknown.RuleID != "t/p:m/none" {
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

func (f failingExtensions) Evaluate(string, map[string]any, rule.TimeLimit, conformance.Facts,
	vocab.UbiquitousLanguage,
) (conformance.ExtensionRun, error) {
	return conformance.ExtensionRun{}, f.err
}

func TestRunRuleTestsContainsConformanceErrorAndContinues(t *testing.T) {
	cfg := ruleTestConfig(t)
	extScope, err := rule.ZoneScope([]rule.ZoneName{"m"})
	if err != nil {
		t.Fatalf("ZoneScope: %v", err)
	}
	extRule, err := rule.New(rule.Spec{
		ID:     "t/p:m/ext",
		Type:   rule.TypeExtension,
		Params: rule.ExtensionParams{Uses: "broken-ext"},
		Scope:  extScope,
	})
	if err != nil {
		t.Fatalf("extension rule.New: %v", err)
	}
	cfg.Rules = append(cfg.Rules, extRule)

	files := []rule.TestFile{{Path: "m/all_good.go"}}
	source := fakeRuleTestSource{tests: []rule.Test{
		mustRuleTest(t, "extension-crash", "t/p:m/ext", files, nil),
		mustRuleTest(t, "still-runs", "t/p:m/snake", files, nil),
	}}
	uc, err := application.NewRunRuleTests(
		ruleTestRepository{cfg},
		source,
		&fakeFixtureObserver{},
		failingExtensions{err: errors.New("extension blew up")},
		&fakeVocabularySource{},
		&fakeRecordingSource{},
	)
	if err != nil {
		t.Fatalf("NewRunRuleTests: %v", err)
	}
	run, err := uc.Execute()
	results := run.Results
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

// A Rule its Extension stopped has no result to compare: the stop is
// that test's failure, never an empty result that passes.
func TestRunRuleTestsReportsStoppedExtensionAsTheTestsError(t *testing.T) {
	cfg := ruleTestConfig(t)
	extScope, err := rule.ZoneScope([]rule.ZoneName{"m"})
	if err != nil {
		t.Fatalf("ZoneScope: %v", err)
	}
	extRule, err := rule.New(rule.Spec{
		ID: "t/p:m/ext", Type: rule.TypeExtension,
		Params: rule.ExtensionParams{Uses: "spin"}, Scope: extScope,
	})
	if err != nil {
		t.Fatalf("extension rule.New: %v", err)
	}
	cfg.Rules = append(cfg.Rules, extRule)
	failure, err := conformance.NewExtensionFailure(conformance.ExtensionFailureSpec{
		Source: ".arclint/extensions/spin.ts", Phase: conformance.PhaseCheck,
		Limit: 5 * time.Second, Elapsed: 5002 * time.Millisecond, TimedOut: true, File: "m/all_good.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	files := []rule.TestFile{{Path: "m/all_good.go"}}
	uc, err := application.NewRunRuleTests(
		ruleTestRepository{cfg},
		fakeRuleTestSource{tests: []rule.Test{mustRuleTest(t, "spins", "t/p:m/ext", files, nil)}},
		&fakeFixtureObserver{},
		failingExtensions{err: failure},
		&fakeVocabularySource{},
		&fakeRecordingSource{},
	)
	if err != nil {
		t.Fatalf("NewRunRuleTests: %v", err)
	}
	run, err := uc.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := `not evaluated: rule t/p:m/ext: extension "spin" (.arclint/extensions/spin.ts) reached its 5s time limit in check and was stopped after 5.002s, last reading m/all_good.go`
	if len(run.Results) != 1 || run.Results[0].Passed() || run.Results[0].Err != want {
		t.Fatalf("results = %+v, want one failed test with Err %q", run.Results, want)
	}
}

// slowExtensions finishes with no findings after most of its limit.
type slowExtensions struct{ elapsed time.Duration }

func (s slowExtensions) Evaluate(string, map[string]any, rule.TimeLimit, conformance.Facts,
	vocab.UbiquitousLanguage,
) (conformance.ExtensionRun, error) {
	return conformance.ExtensionRun{Elapsed: s.elapsed, Source: ".arclint/extensions/slow.ts"}, nil
}

// How long a fixture's check ran is not a finding: a clean check that
// used most of its limit still matches an empty expectation.
func TestRunRuleTestsIgnoresTheNearTimeLimitNote(t *testing.T) {
	cfg := ruleTestConfig(t)
	extScope, err := rule.ZoneScope([]rule.ZoneName{"m"})
	if err != nil {
		t.Fatalf("ZoneScope: %v", err)
	}
	extRule, err := rule.New(rule.Spec{
		ID: "t/p:m/ext", Type: rule.TypeExtension,
		Params: rule.ExtensionParams{Uses: "slow"}, Scope: extScope,
	})
	if err != nil {
		t.Fatalf("extension rule.New: %v", err)
	}
	cfg.Rules = append(cfg.Rules, extRule)
	uc, err := application.NewRunRuleTests(
		ruleTestRepository{cfg},
		fakeRuleTestSource{tests: []rule.Test{mustRuleTest(t, "slow-but-clean", "t/p:m/ext", []rule.TestFile{{Path: "m/all_good.go"}}, nil)}},
		&fakeFixtureObserver{},
		slowExtensions{elapsed: 4 * time.Second},
		&fakeVocabularySource{},
		&fakeRecordingSource{},
	)
	if err != nil {
		t.Fatalf("NewRunRuleTests: %v", err)
	}
	run, err := uc.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(run.Results) != 1 || !run.Results[0].Passed() {
		t.Fatalf("results = %+v, want a pass despite 4s of the 5s limit", run.Results)
	}
}

// recordingExtensions captures the vocabulary each evaluation received
// so tests can prove fixture knowledge reaches ctx.domain().
type recordingExtensions struct {
	knowledge vocab.UbiquitousLanguage
}

func (r *recordingExtensions) Evaluate(_ string, _ map[string]any, _ rule.TimeLimit, _ conformance.Facts,
	knowledge vocab.UbiquitousLanguage,
) (conformance.ExtensionRun, error) {
	r.knowledge = knowledge
	return conformance.ExtensionRun{}, nil
}

func TestRunRuleTestsFeedsFixtureVocabularyToExtensions(t *testing.T) {
	cfg := ruleTestConfig(t)
	extScope, err := rule.ZoneScope([]rule.ZoneName{"m"})
	if err != nil {
		t.Fatalf("ZoneScope: %v", err)
	}
	extRule, err := rule.New(rule.Spec{
		ID:     "t/p:m/ext",
		Type:   rule.TypeExtension,
		Params: rule.ExtensionParams{Uses: "domain-probe"},
		Scope:  extScope,
	})
	if err != nil {
		t.Fatalf("extension rule.New: %v", err)
	}
	cfg.Rules = append(cfg.Rules, extRule)

	const authored = "version: 2\nproject: shop\ncontexts:\n  ordering:\n    definition: Orders.\n    aggregates:\n      Order:\n        definition: A deal.\n        identity: OrderID\n"
	recorded := vocab.UbiquitousLanguage{Project: "shop", Contexts: []vocab.BoundedContext{{
		Name:       "ordering",
		Definition: "Orders.",
		Aggregates: []vocab.Aggregate{{Name: "Order", Definition: "A deal.", Identity: "OrderID"}},
	}}}
	files := []rule.TestFile{
		{Path: "m/all_good.go"},
		{Path: vocab.DefaultDomainRecordingPath, Content: authored},
	}
	source := fakeRuleTestSource{tests: []rule.Test{
		mustRuleTest(t, "sees-vocabulary", "t/p:m/ext", files, nil),
	}}
	vocabulary := &fakeVocabularySource{lang: recorded}
	recorder := &recordingExtensions{}
	uc, err := application.NewRunRuleTests(
		ruleTestRepository{cfg}, source, &fakeFixtureObserver{}, recorder, vocabulary, &fakeRecordingSource{})
	if err != nil {
		t.Fatalf("NewRunRuleTests: %v", err)
	}
	run, err := uc.Execute()
	results := run.Results
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(results) != 1 || !results[0].Passed() {
		t.Fatalf("results = %+v, want one pass", results)
	}
	if string(vocabulary.content) != authored {
		t.Errorf("parsed content = %q, want the authored fixture bytes", vocabulary.content)
	}
	if vocabulary.recording != mustDefaultRecording(t) {
		t.Errorf("parsed as %+v, want the default DomainRecording for a fixture without a ruleset", vocabulary.recording)
	}
	if len(recorder.knowledge.Contexts) != 1 ||
		len(recorder.knowledge.Contexts[0].Aggregates) != 1 ||
		recorder.knowledge.Contexts[0].Aggregates[0].Name != "Order" {
		t.Errorf("evaluator knowledge = %+v, want the parsed Order aggregate", recorder.knowledge)
	}
}

func TestRunRuleTestsContainsFixtureVocabularyError(t *testing.T) {
	cfg := ruleTestConfig(t)
	files := []rule.TestFile{
		{Path: "m/all_good.go"},
		{Path: vocab.DefaultDomainRecordingPath, Content: "version: 2\n"},
	}
	source := fakeRuleTestSource{tests: []rule.Test{
		mustRuleTest(t, "broken-vocabulary", "t/p:m/snake", files, nil),
		mustRuleTest(t, "still-runs", "t/p:m/snake", []rule.TestFile{{Path: "m/all_good.go"}}, nil),
	}}
	vocabulary := &fakeVocabularySource{err: errors.New("unsupported version 2")}
	uc, err := application.NewRunRuleTests(
		ruleTestRepository{cfg}, source, &fakeFixtureObserver{}, nil, vocabulary, &fakeRecordingSource{})
	if err != nil {
		t.Fatalf("NewRunRuleTests: %v", err)
	}
	run, err := uc.Execute()
	results := run.Results
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

// extensionCase builds the configured repository and test source for one
// extension rule probing a fixture of the given files.
func extensionCase(t *testing.T, files []rule.TestFile) (ruleTestRepository, fakeRuleTestSource) {
	t.Helper()
	cfg := ruleTestConfig(t)
	scope, err := rule.ZoneScope([]rule.ZoneName{"m"})
	if err != nil {
		t.Fatalf("ZoneScope: %v", err)
	}
	extRule, err := rule.New(rule.Spec{
		ID:     "t/p:m/ext",
		Type:   rule.TypeExtension,
		Params: rule.ExtensionParams{Uses: "domain-probe"},
		Scope:  scope,
	})
	if err != nil {
		t.Fatalf("extension rule.New: %v", err)
	}
	cfg.Rules = append(cfg.Rules, extRule)
	return ruleTestRepository{cfg}, fakeRuleTestSource{tests: []rule.Test{
		mustRuleTest(t, "probe", "t/p:m/ext", files, nil),
	}}
}

func TestRunRuleTestsResolvesTheFixtureDomainFileFromTheFixtureRuleset(t *testing.T) {
	const ruleset = "domain:\n  file: docs/model.arclint.yaml\n"
	const authored = "version: 1\nproject: shop\n"
	const ignored = "version: 1\nproject: ignored\n"
	recording, err := vocab.NewDomainRecording("docs/model.arclint.yaml")
	if err != nil {
		t.Fatalf("NewDomainRecording: %v", err)
	}
	repo, source := extensionCase(t, []rule.TestFile{
		{Path: "m/all_good.go"},
		{Path: rule.RulesetFileName, Content: ruleset},
		{Path: "docs/model.arclint.yaml", Content: authored},
		{Path: vocab.DefaultDomainRecordingPath, Content: ignored},
	})
	vocabulary := &fakeVocabularySource{lang: vocab.UbiquitousLanguage{Project: "shop", Recording: recording}}
	recordings := &fakeRecordingSource{recording: recording}
	recorder := &recordingExtensions{}
	uc, err := application.NewRunRuleTests(repo, source, &fakeFixtureObserver{}, recorder, vocabulary, recordings)
	if err != nil {
		t.Fatalf("NewRunRuleTests: %v", err)
	}
	run, err := uc.Execute()
	results := run.Results
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(results) != 1 || !results[0].Passed() {
		t.Fatalf("results = %+v, want one pass", results)
	}
	if string(recordings.content) != ruleset {
		t.Errorf("recording source read %q, want the fixture ruleset", recordings.content)
	}
	if string(vocabulary.content) != authored || vocabulary.recording != recording {
		t.Errorf("vocabulary parsed %q as %+v, want the file the fixture ruleset names", vocabulary.content, vocabulary.recording)
	}
	if recorder.knowledge.Recording != recording {
		t.Errorf("evaluator knowledge recording = %+v, want %+v", recorder.knowledge.Recording, recording)
	}
}

func TestRunRuleTestsGivesAFixtureWithoutADomainFileTheRecordingItWouldUse(t *testing.T) {
	recording, err := vocab.NewDomainRecording("docs/model.arclint.yaml")
	if err != nil {
		t.Fatalf("NewDomainRecording: %v", err)
	}
	repo, source := extensionCase(t, []rule.TestFile{
		{Path: "m/all_good.go"},
		{Path: rule.RulesetFileName, Content: "domain:\n  file: docs/model.arclint.yaml\n"},
		{Path: vocab.DefaultDomainRecordingPath, Content: "version: 1\nproject: ignored\n"},
	})
	vocabulary := &fakeVocabularySource{}
	recorder := &recordingExtensions{}
	uc, err := application.NewRunRuleTests(repo, source, &fakeFixtureObserver{}, recorder, vocabulary, &fakeRecordingSource{recording: recording})
	if err != nil {
		t.Fatalf("NewRunRuleTests: %v", err)
	}
	run, err := uc.Execute()
	results := run.Results
	if err != nil || len(results) != 1 || !results[0].Passed() {
		t.Fatalf("Execute = %+v, %v; want one pass", results, err)
	}
	if vocabulary.parsed {
		t.Errorf("the default domain file was parsed although the fixture ruleset names another")
	}
	if !recorder.knowledge.Empty() || recorder.knowledge.Recording != recording {
		t.Errorf("evaluator knowledge = %+v, want an empty language carrying %+v", recorder.knowledge, recording)
	}
}

func TestRunRuleTestsContainsAFixtureRulesetRecordingError(t *testing.T) {
	repo, source := extensionCase(t, []rule.TestFile{
		{Path: "m/all_good.go"},
		{Path: rule.RulesetFileName, Content: "domain:\n  file: notes.txt\n"},
	})
	recordings := &fakeRecordingSource{err: errors.New("rules.arclint.yaml: domain.file: line 2: DomainRecording format-named-by-extension")}
	uc, err := application.NewRunRuleTests(repo, source, &fakeFixtureObserver{}, &recordingExtensions{}, &fakeVocabularySource{}, recordings)
	if err != nil {
		t.Fatalf("NewRunRuleTests: %v", err)
	}
	run, err := uc.Execute()
	results := run.Results
	if err != nil {
		t.Fatalf("Execute must contain a fixture ruleset failure: %v", err)
	}
	if len(results) != 1 || results[0].Passed() || !strings.Contains(results[0].Err, "format-named-by-extension") {
		t.Errorf("results = %+v, want a contained fixture error naming the broken invariant", results)
	}
}

func TestRunRuleTestsDiscoversTheFixtureDomainFileAmongTheDefaultNames(t *testing.T) {
	const yaml, yml, cml = "domain.arclint.yaml", "domain.arclint.yml", "domain.arclint.cml"
	for _, tc := range []struct {
		name  string
		carry []string
		want  string
	}{
		{"only the CML file", []string{cml}, cml},
		{"only the .yml file", []string{yml}, yml},
		{"the YAML file beside the CML file", []string{cml, yaml}, yaml},
		{"the .yml file beside the CML file", []string{cml, yml}, yml},
		{"all three", []string{cml, yml, yaml}, yaml},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := []rule.TestFile{{Path: "m/all_good.go"}}
			for _, name := range tc.carry {
				files = append(files, rule.TestFile{Path: name, Content: "authored in " + name})
			}
			repo, source := extensionCase(t, files)
			vocabulary := &fakeVocabularySource{}
			recorder := &recordingExtensions{}
			uc, err := application.NewRunRuleTests(repo, source, &fakeFixtureObserver{}, recorder, vocabulary, &fakeRecordingSource{})
			if err != nil {
				t.Fatalf("NewRunRuleTests: %v", err)
			}
			run, err := uc.Execute()
			results := run.Results
			if err != nil || len(results) != 1 || !results[0].Passed() {
				t.Fatalf("Execute = %+v, %v; want one pass", results, err)
			}
			if vocabulary.recording.Path() != tc.want || string(vocabulary.content) != "authored in "+tc.want {
				t.Errorf("parsed %q as %q, want the file %s", vocabulary.content, vocabulary.recording.Path(), tc.want)
			}
		})
	}
}

func TestRunRuleTestsFixtureWithOnlyANamedFileIgnoresTheDefaultNames(t *testing.T) {
	recording, err := vocab.NewDomainRecording("docs/model.cml")
	if err != nil {
		t.Fatal(err)
	}
	repo, source := extensionCase(t, []rule.TestFile{
		{Path: "m/all_good.go"},
		{Path: rule.RulesetFileName, Content: "domain:\n  file: docs/model.cml\n"},
		{Path: "domain.arclint.yaml", Content: "ignored"},
		{Path: "domain.arclint.cml", Content: "ignored"},
	})
	vocabulary := &fakeVocabularySource{}
	recorder := &recordingExtensions{}
	uc, err := application.NewRunRuleTests(repo, source, &fakeFixtureObserver{}, recorder, vocabulary, &fakeRecordingSource{recording: recording})
	if err != nil {
		t.Fatalf("NewRunRuleTests: %v", err)
	}
	run, err := uc.Execute()
	results := run.Results
	if err != nil || len(results) != 1 || !results[0].Passed() {
		t.Fatalf("Execute = %+v, %v; want one pass", results, err)
	}
	if vocabulary.parsed {
		t.Errorf("a default-named file was parsed although the fixture ruleset names docs/model.cml, which the fixture does not carry")
	}
	if !recorder.knowledge.Empty() || recorder.knowledge.Recording != recording {
		t.Errorf("evaluator knowledge = %+v, want an empty language carrying %+v", recorder.knowledge, recording)
	}
}

func mustDefaultRecording(t *testing.T) vocab.DomainRecording {
	t.Helper()
	recording, err := vocab.DefaultDomainRecording()
	if err != nil {
		t.Fatal(err)
	}
	return recording
}

// A fixture may spell its domain file the way its ruleset does, before
// the path is cleaned; the file is still found under its cleaned path.
func TestRunRuleTestsFindsAFixtureDomainFileSpelledUncleaned(t *testing.T) {
	const ruleset = "domain:\n  file: docs/../model.yaml\n"
	const authored = "version: 1\nproject: shop\n"
	recording, err := vocab.NewDomainRecording("model.yaml")
	if err != nil {
		t.Fatalf("NewDomainRecording: %v", err)
	}
	repo, source := extensionCase(t, []rule.TestFile{
		{Path: "m/all_good.go"},
		{Path: rule.RulesetFileName, Content: ruleset},
		{Path: "docs/../model.yaml", Content: authored},
	})
	vocabulary := &fakeVocabularySource{lang: vocab.UbiquitousLanguage{Project: "shop", Recording: recording}}
	recordings := &fakeRecordingSource{recording: recording}
	uc, err := application.NewRunRuleTests(repo, source, &fakeFixtureObserver{}, &recordingExtensions{}, vocabulary, recordings)
	if err != nil {
		t.Fatalf("NewRunRuleTests: %v", err)
	}
	if _, err := uc.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if string(vocabulary.content) != authored || vocabulary.recording != recording {
		t.Errorf("vocabulary parsed %q as %+v, want the fixture's model.yaml", vocabulary.content, vocabulary.recording)
	}
}

// A fixture may spell its ruleset with a leading "./"; its domain.file
// setting still decides the recording.
func TestRunRuleTestsReadsAFixtureRulesetSpelledUncleaned(t *testing.T) {
	const ruleset = "domain:\n  file: docs/model.arclint.yaml\n"
	const authored = "version: 1\nproject: shop\n"
	recording, err := vocab.NewDomainRecording("docs/model.arclint.yaml")
	if err != nil {
		t.Fatalf("NewDomainRecording: %v", err)
	}
	repo, source := extensionCase(t, []rule.TestFile{
		{Path: "m/all_good.go"},
		{Path: "./" + rule.RulesetFileName, Content: ruleset},
		{Path: "docs/model.arclint.yaml", Content: authored},
	})
	vocabulary := &fakeVocabularySource{lang: vocab.UbiquitousLanguage{Project: "shop", Recording: recording}}
	recordings := &fakeRecordingSource{recording: recording}
	uc, err := application.NewRunRuleTests(repo, source, &fakeFixtureObserver{}, &recordingExtensions{}, vocabulary, recordings)
	if err != nil {
		t.Fatalf("NewRunRuleTests: %v", err)
	}
	if _, err := uc.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if string(recordings.content) != ruleset || string(vocabulary.content) != authored {
		t.Errorf("read ruleset %q and vocabulary %q, want the fixture's ruleset and the file it names", recordings.content, vocabulary.content)
	}
}
