package application

import (
	"fmt"
	"slices"
	"sort"

	"github.com/wixregiga/arclint/internal/domain/baseline"
	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// ObservationSource is the port through which the use case requests
// normalized Observations for the configured languages and exactly
// the fact classes the Rules' Enforcement declares, without selecting
// parser technology. referencesFor, when not nil, selects the files
// the reference facts (calls, resolved calls, field types) are needed
// for; a language holding none of them may be observed without them.
// Nil needs them for every file. namedTests are the test files the
// domain recording names under verified_by, read even when the scan
// leaves them out.
type ObservationSource interface {
	Observe(languages []rule.Language, scan rule.Scan, facts []rule.Fact, referencesFor func(path string) bool, namedTests []string) (conformance.Observations, error)
}

// BaselineSource loads the repository's committed Baseline. Absence is
// a normal result, not an error.
type BaselineSource interface {
	Load() (baseline.Snapshot, bool, error)
}

// AssessConformanceRequest is the plain request value.
type AssessConformanceRequest struct {
	// SkipBaseline evaluates without subtracting the committed Baseline.
	SkipBaseline bool
	// Only narrows the check to the Rules matching at least one
	// selector: an exact qualified id, an id prefix, or a path.Match
	// pattern such as "arclint:domain/*". Empty selects every
	// configured Rule.
	Only []string
	// Exclude removes the Rules matching any selector; exclusion wins
	// over selection.
	Exclude []string
}

// AssessConformance coordinates observations, configured Rules, the
// Baseline, and the Conformance Check.
type AssessConformance struct {
	rules        rule.Repository
	observations ObservationSource
	baselines    BaselineSource
	extensions   conformance.ExtensionEvaluator
	knowledge    vocab.Repository
}

// NewAssessConformance requires the repository, observation, baseline,
// and domain-model ports. The Extension mechanism may be nil: extension
// Rules then evaluate unsupported, honestly, rather than being skipped.
func NewAssessConformance(rules rule.Repository, observations ObservationSource,
	baselines BaselineSource, extensions conformance.ExtensionEvaluator,
	knowledge vocab.Repository,
) (AssessConformance, error) {
	if rules == nil {
		return AssessConformance{}, fmt.Errorf("assess conformance: missing rule repository")
	}
	if observations == nil {
		return AssessConformance{}, fmt.Errorf("assess conformance: missing observation source")
	}
	if baselines == nil {
		return AssessConformance{}, fmt.Errorf("assess conformance: missing baseline source")
	}
	if knowledge == nil {
		return AssessConformance{}, fmt.Errorf("assess conformance: missing domain model repository")
	}
	return AssessConformance{
		rules: rules, observations: observations,
		baselines: baselines, extensions: extensions, knowledge: knowledge,
	}, nil
}

// Execute runs one Conformance Check and applies the committed
// Baseline afterwards, surfacing stale Baseline entries as a coverage
// Diagnostic.
func (uc AssessConformance) Execute(req AssessConformanceRequest) (conformance.Assessment, error) {
	run, err := uc.run(req, nil)
	if err != nil {
		return conformance.Assessment{}, err
	}
	return run.assessment, nil
}

// assessmentRun is one Conformance Check with what it read: the
// configured Rules, the recorded language and the observations, so a
// use case composing the check reads the same repository state instead
// of observing it again.
type assessmentRun struct {
	assessment conformance.Assessment
	configured rule.Configured
	knowledge  vocab.UbiquitousLanguage
	// found is false when the project records no domain file yet.
	found        bool
	observations conformance.Observations
}

// run is one Conformance Check that also observes the extra fact
// classes, and returns what the check read with the Assessment.
func (uc AssessConformance) run(req AssessConformanceRequest, extra []rule.Fact) (assessmentRun, error) {
	cfg, err := uc.rules.ConfiguredRules()
	if err != nil {
		return assessmentRun{}, fmt.Errorf("load configured rules: %w", err)
	}
	rules := cfg.Rules
	if len(req.Only) > 0 || len(req.Exclude) > 0 {
		if rules, err = selectRules(rules, req.Only, req.Exclude); err != nil {
			return assessmentRun{}, err
		}
	}
	// Missing domain model is an empty Ubiquitous Language; load
	// failure is a configuration error, same class as an unreadable
	// rules.arclint.yaml.
	knowledge, found, err := uc.knowledge.RecordedLanguage()
	if err != nil {
		return assessmentRun{}, fmt.Errorf("load domain model: %w", err)
	}
	if !found {
		knowledge = vocab.UbiquitousLanguage{Recording: uc.knowledge.Recording()}
	}
	// The language names the test files its verified_by entries ask to
	// be read, whatever the scan leaves out; the Rules judging those
	// tests decide which of them may be read.
	facts := requiredFacts(rules)
	added := false
	for _, f := range extra {
		if !slices.Contains(facts, f) {
			facts = append(facts, f)
			added = true
		}
	}
	if added {
		// requiredFacts returns its classes sorted; keep them so.
		slices.Sort(facts)
	}
	observations, err := uc.observations.Observe(cfg.Languages, cfg.Scan, facts, nil, conformance.NamedTestsToRead(rules, knowledge))
	if err != nil {
		return assessmentRun{}, fmt.Errorf("observe repository: %w", err)
	}
	out := assessmentRun{configured: cfg, knowledge: knowledge, found: found, observations: observations}
	assessment, err := conformance.Run(conformance.Request{
		Rules:          rules,
		Zones:          cfg.Zones,
		Observations:   observations,
		UnknownImports: cfg.Scan.UnknownImports,
		Extensions:     uc.extensions,
		Knowledge:      knowledge,
	})
	if err != nil {
		return assessmentRun{}, fmt.Errorf("conformance check: %w", err)
	}
	out.assessment = assessment
	if req.SkipBaseline {
		return out, nil
	}
	snapshot, present, err := uc.baselines.Load()
	if err != nil {
		return assessmentRun{}, fmt.Errorf("load baseline: %w", err)
	}
	if !present {
		return out, nil
	}
	covered, stale, err := snapshot.Apply(assessment)
	if err != nil {
		return assessmentRun{}, fmt.Errorf("apply baseline: %w", err)
	}
	out.assessment = covered
	if len(stale) > 0 {
		note, err := conformance.NewCoverage("",
			fmt.Sprintf("baseline: %d adopted finding(s) no longer occur; refresh the baseline", staleCount(stale)))
		if err != nil {
			return assessmentRun{}, fmt.Errorf("baseline coverage note: %w", err)
		}
		noted, err := covered.WithDiagnostics(note)
		if err != nil {
			return assessmentRun{}, fmt.Errorf("baseline coverage note: %w", err)
		}
		out.assessment = noted
	}
	return out, nil
}

// selectRules narrows the configured Rules by selectors (exact
// qualified Rule ids, id prefixes, path.Match patterns, or the Pattern
// that distributed them) with exclusion winning over selection. Selection fails loudly, never
// evaluates vacuously: every selector must match at least one
// configured Rule, and the narrowed set may not be empty.
func selectRules(rules []rule.Rule, only, exclude []string) ([]rule.Rule, error) {
	selected := map[string]bool{}
	if len(only) == 0 {
		for _, r := range rules {
			selected[r.ID().Qualified()] = true
		}
	}
	for _, s := range only {
		hits, err := selectorHits(s, rules)
		if err != nil {
			return nil, err
		}
		if len(hits) == 0 {
			return nil, fmt.Errorf("rule selector %q matches no configured rule", s)
		}
		for _, id := range hits {
			selected[id] = true
		}
	}
	for _, s := range exclude {
		hits, err := selectorHits(s, rules)
		if err != nil {
			return nil, err
		}
		if len(hits) == 0 {
			return nil, fmt.Errorf("rule selector %q matches no configured rule", s)
		}
		for _, id := range hits {
			delete(selected, id)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("rule selection leaves no rule to evaluate")
	}
	out := make([]rule.Rule, 0, len(selected))
	for _, r := range rules {
		if selected[r.ID().Qualified()] {
			out = append(out, r)
		}
	}
	return out, nil
}

func staleCount(entries []baseline.Entry) int {
	total := 0
	for _, e := range entries {
		total += e.Count()
	}
	return total
}

// requiredFacts unions the fact classes every enabled Rule's
// Enforcement declares to guide collection. Producers may collect shared base
// facts; conformance.Facts enforces each Rule's access to the required classes.
func requiredFacts(rules []rule.Rule) []rule.Fact {
	seen := map[rule.Fact]bool{}
	var out []rule.Fact
	for _, r := range rules {
		if r.Disabled() {
			continue
		}
		for _, f := range r.Enforcement().Facts() {
			if !seen[f] {
				seen[f] = true
				out = append(out, f)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
