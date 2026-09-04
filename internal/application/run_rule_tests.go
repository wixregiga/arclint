package application

import (
	"fmt"

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
// fixture's authored domain.arclint.yaml content into the
// recorded vocabulary that extension rules under test observe through
// ctx.domain().
type VocabularySource interface {
	ParseUbiquitousLanguage(content []byte) (vocab.UbiquitousLanguage, error)
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

// Passed reports whether the test ran and its complete result matched
// the expectation exactly.
func (r RuleTestResult) Passed() bool {
	return r.Err == "" && len(r.Missing) == 0 && len(r.Unexpected) == 0
}

// RunRuleTests evaluates every authored Rule Test: each fixture is
// observed with exactly the facts its Rule's Enforcement declares and
// checked with only that Rule under the repository's configured
// Modules and unknown-import policy.
type RunRuleTests struct {
	rules      rule.Repository
	tests      RuleTestSource
	fixtures   FixtureObserver
	extensions conformance.ExtensionEvaluator
	vocabulary VocabularySource
}

// NewRunRuleTests requires the repository, test source, fixture
// observer, and vocabulary source ports. The Extension mechanism may
// be nil: extension Rules then evaluate unsupported, honestly, rather
// than being skipped.
func NewRunRuleTests(rules rule.Repository, tests RuleTestSource,
	fixtures FixtureObserver, extensions conformance.ExtensionEvaluator,
	vocabulary VocabularySource,
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
	return RunRuleTests{
		rules: rules, tests: tests,
		fixtures: fixtures, extensions: extensions,
		vocabulary: vocabulary,
	}, nil
}

// Execute runs every Rule Test and returns one result per test in
// source order. A test naming no configured Rule, or whose
// conformance check fails, is a test failure carried in its result;
// the error return is reserved for infrastructure failures.
func (uc RunRuleTests) Execute() ([]RuleTestResult, error) {
	cfg, err := uc.rules.ConfiguredRules()
	if err != nil {
		return nil, fmt.Errorf("load configured rules: %w", err)
	}
	tests, err := uc.tests.Tests()
	if err != nil {
		return nil, fmt.Errorf("load rule tests: %w", err)
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
		observations, err := uc.fixtures.Observe(t.Files(), cfg.Languages, cfg.Scan, r.Enforcement().Facts())
		if err != nil {
			return nil, fmt.Errorf("rule test %q: observe fixture: %w", t.Name(), err)
		}
		// A fixture authors its recorded vocabulary as
		// domain.arclint.yaml at the tree root; extension rules
		// under test observe it through ctx.domain().
		knowledge, err := uc.fixtureVocabulary(t.Files())
		if err != nil {
			result.Err = fmt.Sprintf("fixture vocabulary: %v", err)
			results = append(results, result)
			continue
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
			Modules:        cfg.Modules,
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
		comparison := t.Compare(assessmentFindings(assessment))
		result.Missing = comparison.Missing
		result.Unexpected = comparison.Unexpected
		results = append(results, result)
	}
	return results, nil
}

// fixtureVocabulary parses the vocabulary a fixture authors as
// domain.arclint.yaml at its tree root; fixtures without one see
// an empty vocabulary.
func (uc RunRuleTests) fixtureVocabulary(files []rule.TestFile) (vocab.UbiquitousLanguage, error) {
	for _, f := range files {
		if f.Path == vocab.UbiquitousLanguageFileName {
			lang, err := uc.vocabulary.ParseUbiquitousLanguage([]byte(f.Content))
			if err != nil {
				// No prefix: the parser's message already names
				// the vocabulary file; Execute adds the
				// fixture-vocabulary context.
				return vocab.UbiquitousLanguage{}, fmt.Errorf("%w", err)
			}
			return lang, nil
		}
	}
	return vocab.UbiquitousLanguage{}, nil
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
			f.Kind = rule.FindingCoverage
		}
		out = append(out, f)
	}
	return out
}
