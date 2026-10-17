package conformance

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// evaluateExtensionRule delegates one Rule to its Extension through
// the evaluator port. The extension sees exactly the Rule's selected
// subjects; without a supplied mechanism every subject honestly
// evaluates unsupported. Heuristic assurance keeps the outcomes
// honest: findings are suspected violations, absence of findings is
// undetermined, never conformance.
//
// A finding with neither a selected location nor supplied dependency evidence
// for an explicitly selected subject is a Scope breach: every finding from that Extension run is
// discarded as untrustworthy, each selected subject evaluates failed,
// excluded subjects stay not-applicable, and error-severity
// operational Diagnostics identify the breach. The check still returns
// a complete Assessment (no error) so other Rules keep reporting.
//
// An Extension that runs past its time limit or fails while running
// returns an ExtensionFailure: the Rule gets no Evaluation and one
// not-evaluated Diagnostic, and the check goes on with the other
// Rules. Any other error aborts the check as a configuration error. A
// check that ran longer than half its TimeLimit adds a coverage
// Diagnostic saying so: a run twice as slow would stop it.
func evaluateExtensionRule(r rule.Rule, facts Facts,
	evaluator ExtensionEvaluator, knowledge vocab.UbiquitousLanguage,
) ([]Evaluation, []Diagnostic, error) {
	params, ok := r.Params().(rule.ExtensionParams)
	if !ok {
		return nil, nil, fmt.Errorf("rule %s: extension rule with %T params", r.ID(), r.Params())
	}
	if evaluator == nil {
		es, err := evaluateUnsupported(r, facts)
		return es, nil, err
	}
	selected, excluded := facts.selectedFiles()
	limit := params.Limit()
	run, err := evaluator.Evaluate(params.Uses, params.With, limit, facts, knowledge)
	var failure ExtensionFailure
	if errors.As(err, &failure) {
		d, err := NewNotEvaluated(r.ID().Qualified(), params.Uses, failure)
		if err != nil {
			return nil, nil, err
		}
		return nil, []Diagnostic{d}, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("rule %s: %w", r.ID(), err)
	}

	var near []Diagnostic
	if limit.Near(run.Elapsed) {
		d, err := NewCoverage(r.ID().Qualified(), fmt.Sprintf(
			"rule %s: extension %q took %s of its %s time limit; a run twice as slow would not be evaluated, so make the check faster or raise time_limit",
			r.ID(), params.Uses, run.Elapsed.Round(time.Millisecond), limit))
		if err != nil {
			return nil, nil, err
		}
		near = append(near, d)
	}

	var breaches []ExtensionFinding
	byPath := map[string][]ExtensionFinding{}
	for _, f := range run.Findings {
		subject := f.SubjectPath
		if subject == "" {
			subject = f.Path
		}
		if !facts.AllowsFinding(subject, f.Path, f.Line) {
			breaches = append(breaches, f)
			continue
		}
		byPath[subject] = append(byPath[subject], f)
	}
	if len(breaches) > 0 {
		es, diags, err := containExtensionScopeBreach(r, params.Uses, selected, excluded, breaches)
		return es, append(diags, near...), err
	}

	var out []Evaluation
	for _, path := range selected {
		subject, err := rule.FileSubject(path)
		if err != nil {
			return nil, nil, fmt.Errorf("extension: %w", err)
		}
		reported := byPath[path]
		sort.SliceStable(reported, func(i, j int) bool {
			if reported[i].Path != reported[j].Path {
				return reported[i].Path < reported[j].Path
			}
			if reported[i].Line != reported[j].Line {
				return reported[i].Line < reported[j].Line
			}
			return reported[i].Message < reported[j].Message
		})
		var vs []Violation
		for _, f := range reported {
			v, err := newViolation(r, subject, f.Path, f.Line, f.Message, f.Remediation)
			if err != nil {
				return nil, nil, err
			}
			vs = append(vs, v)
		}
		e, err := completeEvaluation(r, subject, vs)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, e)
	}
	out, err = appendNotApplicable(out, r, excluded)
	return out, near, err
}

// containExtensionScopeBreach discards untrustworthy Extension
// findings and records failed selected subjects plus operational
// Diagnostics. When nothing was selected, only the Diagnostics are
// returned, no fabricated Evaluation.
func containExtensionScopeBreach(r rule.Rule, extension string,
	selected, excluded []string, breaches []ExtensionFinding,
) ([]Evaluation, []Diagnostic, error) {
	sort.SliceStable(breaches, func(i, j int) bool {
		a, b := breaches[i], breaches[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Message != b.Message {
			return a.Message < b.Message
		}
		return a.Remediation < b.Remediation
	})

	ruleID := r.ID().Qualified()
	var diags []Diagnostic
	for _, f := range breaches {
		msg := fmt.Sprintf("rule %s: extension %q reported %q, which is outside the rule's scope",
			r.ID(), extension, f.Path)
		if f.SubjectPath != "" {
			msg = fmt.Sprintf("rule %s: extension %q reported %s:%d for subject %q without permission from its supplied facts", r.ID(), extension, f.Path, f.Line, f.SubjectPath)
		}
		d, err := NewOperational(ruleID, f.Path, f.Line, rule.SeverityError, msg)
		if err != nil {
			return nil, nil, err
		}
		diags = append(diags, d)
	}

	var out []Evaluation
	for _, path := range selected {
		subject, err := rule.FileSubject(path)
		if err != nil {
			return nil, nil, fmt.Errorf("extension: %w", err)
		}
		e, err := simpleEvaluation(r, subject, OutcomeFailed)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, e)
	}
	out, err := appendNotApplicable(out, r, excluded)
	if err != nil {
		return nil, nil, err
	}
	return out, diags, nil
}
