package application

import (
	"fmt"
	"path"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// RuleTestSource is the port through which the use case loads the
// repository's authored Rule Tests, in deterministic order.
type RuleTestSource interface {
	Tests() ([]rule.Test, error)
}

// FixtureObserver is the port through which the use case observes one
// Rule Test fixture: it materializes the fixture files and produces
// normalized Observations for the requested languages, scan policy,
// and fact classes through the same analysis production uses.
type FixtureObserver interface {
	Observe(files []rule.TestFile, languages []rule.Language, scan rule.Scan, facts []rule.Fact) (conformance.Observations, error)
}

// VocabularySource is the port through which the use case turns a
// fixture's authored domain file content into the recorded vocabulary
// that extension rules under test observe through ctx.domain(). The
// DomainRecording names the file the content is the content of, and so
// the format it is written in.
type VocabularySource interface {
	ParseUbiquitousLanguage(content []byte, recording vocab.DomainRecording) (vocab.UbiquitousLanguage, error)
}

// RulesetRecordingSource is the port through which the use case reads
// the DomainRecording a fixture's own ruleset names under domain.file,
// the way a repository's ruleset names its own: the zero DomainRecording
// when it names none.
type RulesetRecordingSource interface {
	ParseRulesetRecording(content []byte) (vocab.DomainRecording, error)
}

// RuleTestResult is the outcome of one Rule Test: the comparison of
// the produced result against the complete expected one, or the
// test-level error that prevented a comparison.
type RuleTestResult struct {
	Name       string
	RuleID     string
	Missing    []rule.ExpectedFinding
	Unexpected []rule.Finding
	// Err reports a test-level failure, such as a Rule ID naming no
	// configured Rule or a conformance/evaluator failure; "" when the
	// test ran to a comparison.
	Err string
}

// RuleTestRun is the outcome of running every authored Rule Test: one
// result per test in source order, and the enabled Rules whose
// Assurance holds only for their pattern that no test exercises with a
// failing case.
type RuleTestRun struct {
	Results []RuleTestResult
	// WithoutFailingCase lists qualified Rule identities in
	// configuration order.
	WithoutFailingCase []string
}

// Passed reports whether every test passed and every pattern-bound Rule
// has a failing case.
func (r RuleTestRun) Passed() bool {
	if len(r.WithoutFailingCase) > 0 {
		return false
	}
	for _, result := range r.Results {
		if !result.Passed() {
			return false
		}
	}
	return true
}

// Passed reports whether the test ran and its complete result matched
// the expectation exactly.
func (r RuleTestResult) Passed() bool {
	return r.Err == "" && len(r.Missing) == 0 && len(r.Unexpected) == 0
}

// RunRuleTests evaluates every authored Rule Test: each fixture is
// observed with exactly the facts its Rule's Enforcement declares and
// checked with only that Rule under the repository's configured
// Zones and unknown-import policy.
type RunRuleTests struct {
	rules      rule.Repository
	tests      RuleTestSource
	fixtures   FixtureObserver
	extensions conformance.ExtensionEvaluator
	vocabulary VocabularySource
	recordings RulesetRecordingSource
}

// NewRunRuleTests requires the repository, test source, fixture
// observer, vocabulary source, and ruleset recording source ports. The
// Extension mechanism may be nil: extension Rules then evaluate
// unsupported, honestly, rather than being skipped.
func NewRunRuleTests(rules rule.Repository, tests RuleTestSource,
	fixtures FixtureObserver, extensions conformance.ExtensionEvaluator,
	vocabulary VocabularySource, recordings RulesetRecordingSource,
) (RunRuleTests, error) {
	if rules == nil {
		return RunRuleTests{}, fmt.Errorf("run rule tests: missing rule repository")
	}
	if tests == nil {
		return RunRuleTests{}, fmt.Errorf("run rule tests: missing rule test source")
	}
	if fixtures == nil {
		return RunRuleTests{}, fmt.Errorf("run rule tests: missing fixture observer")
	}
	if vocabulary == nil {
		return RunRuleTests{}, fmt.Errorf("run rule tests: missing vocabulary source")
	}
	if recordings == nil {
		return RunRuleTests{}, fmt.Errorf("run rule tests: missing ruleset recording source")
	}
	return RunRuleTests{
		rules: rules, tests: tests,
		fixtures: fixtures, extensions: extensions,
		vocabulary: vocabulary, recordings: recordings,
	}, nil
}

// Execute runs every Rule Test and returns one result per test in
// source order, together with the pattern-bound Rules that lack a
// failing case. A test naming no configured Rule, or whose conformance
// check fails, is a test failure carried in its result; the error
// return is reserved for infrastructure failures.
func (uc RunRuleTests) Execute() (RuleTestRun, error) {
	cfg, err := uc.rules.ConfiguredRules()
	if err != nil {
		return RuleTestRun{}, fmt.Errorf("load configured rules: %w", err)
	}
	tests, err := uc.tests.Tests()
	if err != nil {
		return RuleTestRun{}, fmt.Errorf("load rule tests: %w", err)
	}
	byID := make(map[string]rule.Rule, len(cfg.Rules))
	for _, r := range cfg.Rules {
		byID[r.ID().Qualified()] = r
	}
	results := make([]RuleTestResult, 0, len(tests))
	for _, t := range tests {
		result := RuleTestResult{Name: t.Name(), RuleID: t.RuleID()}
		r, ok := byID[t.RuleID()]
		if !ok {
			result.Err = fmt.Sprintf("rule %q is not configured", t.RuleID())
			results = append(results, result)
			continue
		}
		// A fixture authors its recorded vocabulary in the domain file
		// its own ruleset names, else in the first default-named file
		// it carries; extension rules under test observe it through
		// ctx.domain().
		knowledge, err := uc.fixtureVocabulary(t.Files())
		if err != nil {
			result.Err = fmt.Sprintf("fixture vocabulary: %v", err)
			results = append(results, result)
			continue
		}
		observations, err := uc.fixtures.Observe(t.Files(), cfg.Languages, cfg.Scan, r.Enforcement().Facts())
		if err != nil {
			return RuleTestRun{}, fmt.Errorf("rule test %q: observe fixture: %w", t.Name(), err)
		}
		// An expanded Rule is re-derived against the fixture's own
		// vocabulary, so the test exercises the fixture's recorded
		// terms rather than the repository's.
		if _, expanded := r.Expansion(); expanded {
			if r, err = r.Reexpand(knowledge); err != nil {
				result.Err = fmt.Sprintf("reexpand rule: %v", err)
				results = append(results, result)
				continue
			}
		}
		assessment, err := conformance.Run(conformance.Request{
			Rules:          []rule.Rule{r},
			Zones:          cfg.Zones,
			Observations:   observations,
			UnknownImports: cfg.Scan.UnknownImports,
			Extensions:     uc.extensions,
			Knowledge:      knowledge,
		})
		if err != nil {
			// Evaluator/extension crashes are this test's failure, not
			// infrastructure: later Rule Tests must still run.
			result.Err = fmt.Sprintf("conformance check: %v", err)
			results = append(results, result)
			continue
		}
		// A Rule its Extension stopped has no result to compare: the
		// stop is this test's failure, as an aborted check is.
		if stopped := notEvaluated(assessment); stopped != "" {
			result.Err = stopped
			results = append(results, result)
			continue
		}
		comparison := t.Compare(assessmentFindings(assessment))
		result.Missing = comparison.Missing
		result.Unexpected = comparison.Unexpected
		results = append(results, result)
	}
	run := RuleTestRun{Results: results}
	for _, id := range rule.WithoutFailingCase(cfg.Rules, tests) {
		run.WithoutFailingCase = append(run.WithoutFailingCase, id.Qualified())
	}
	return run, nil
}

// fixtureVocabulary parses the vocabulary a fixture authors in its
// domain file. The file is resolved like a repository's: the
// DomainRecording the fixture's own rules.arclint.yaml names under
// domain.file, else the first of the default names the fixture carries,
// else domain.arclint.yaml, at the tree root. A fixture without the
// file sees an empty vocabulary that carries the DomainRecording it
// would be written in.
func (uc RunRuleTests) fixtureVocabulary(files []rule.TestFile) (vocab.UbiquitousLanguage, error) {
	var named vocab.DomainRecording
	for _, f := range files {
		if path.Clean(f.Path) == rule.RulesetFileName {
			recording, err := uc.recordings.ParseRulesetRecording([]byte(f.Content))
			if err != nil {
				return vocab.UbiquitousLanguage{}, fmt.Errorf("%w", err)
			}
			named = recording
		}
	}
	// A fixture names its files as written; a recording holds its path
	// cleaned, so the two are compared cleaned.
	carries := func(candidate vocab.DomainRecording) (bool, error) {
		for _, f := range files {
			if path.Clean(f.Path) == candidate.Path() {
				return true, nil
			}
		}
		return false, nil
	}
	recording, _, err := vocab.ResolveDomainRecording(named, carries)
	if err != nil {
		return vocab.UbiquitousLanguage{}, fmt.Errorf("%w", err)
	}
	for _, f := range files {
		if path.Clean(f.Path) == recording.Path() {
			lang, err := uc.vocabulary.ParseUbiquitousLanguage([]byte(f.Content), recording)
			if err != nil {
				// No prefix: the parser's message already names
				// the vocabulary file; Execute adds the
				// fixture-vocabulary context.
				return vocab.UbiquitousLanguage{}, fmt.Errorf("%w", err)
			}
			return lang, nil
		}
	}
	return vocab.UbiquitousLanguage{Recording: recording}, nil
}

// assessmentFindings maps one Assessment to the neutral findings a
// Rule Test asserts against: active Violations surface as violation
// entries, suppressed Violations as suppressed entries, and the
// operational and coverage Diagnostics under their own kinds, each
// carrying its diagnostic anchor path, line, and message.
func assessmentFindings(a conformance.Assessment) []rule.Finding {
	diagnostics := a.Diagnostics()
	out := make([]rule.Finding, 0, len(diagnostics))
	for _, d := range diagnostics {
		f := rule.Finding{Path: d.Path(), Line: d.Line(), Message: d.Message()}
		switch d.Kind() {
		case conformance.DiagnosticViolation:
			switch d.Status() {
			case conformance.StatusActive:
				f.Kind = rule.FindingViolation
			case conformance.StatusSuppressed:
				f.Kind = rule.FindingSuppressed
			case conformance.StatusBaselined:
				// No Baseline participates in a Rule Test run.
				continue
			}
		case conformance.DiagnosticOperational:
			f.Kind = rule.FindingOperational
		case conformance.DiagnosticCoverage:
			if _, near := d.NearTimeLimit(); near {
				// How long a fixture ran is not a finding it can expect.
				continue
			}
			f.Kind = rule.FindingCoverage
		case conformance.DiagnosticNotEvaluated:
			// Execute reports a stopped Rule as the test's error.
			continue
		}
		out = append(out, f)
	}
	return out
}

// notEvaluated states why the Assessment's Rules were not evaluated,
// "" when every Rule was.
func notEvaluated(a conformance.Assessment) string {
	var reasons []string
	for _, d := range a.Diagnostics() {
		if d.Kind() == conformance.DiagnosticNotEvaluated {
			reasons = append(reasons, "not evaluated: "+d.Message())
		}
	}
	return strings.Join(reasons, "; ")
}
