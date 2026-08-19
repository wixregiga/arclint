package conformance

import (
	"fmt"
	"sort"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// evaluateExtensionRule delegates one Rule to its Extension through
// the evaluator port. The extension sees exactly the Rule's selected
// subjects; without a supplied mechanism every subject honestly
// evaluates unsupported. Heuristic assurance keeps the outcomes
// honest: findings are suspected violations, absence of findings is
// undetermined, never conformance.
//
// A finding that names a path outside the selected subjects is an
// Applicability breach: every finding from that Extension run is
// discarded as untrustworthy, each selected subject evaluates failed,
// excluded subjects stay not-applicable, and error-severity
// operational Diagnostics identify the breach. The check still returns
// a complete Assessment (no error) so other Rules keep reporting.
func evaluateExtensionRule(r rule.Rule, mem membership, obs Observations,
	evaluator ExtensionEvaluator, modules []rule.Module,
) ([]Evaluation, []Diagnostic, error) {
	params, ok := r.Params().(rule.ExtensionParams)
	if !ok {
		return nil, nil, fmt.Errorf("rule %s: extension rule with %T params", r.ID(), r.Params())
	}
	if evaluator == nil {
		es, err := evaluateUnsupported(r, mem)
		return es, nil, err
	}
	selected, excluded := partitionFiles(r, mem)
	findings, err := evaluator.Evaluate(params.Uses, params.With, selected, modules, obs)
	if err != nil {
		return nil, nil, fmt.Errorf("rule %s: %v", r.ID(), err)
	}

	inScope := map[string]bool{}
	for _, f := range selected {
		inScope[f] = true
	}
	var breaches []ExtensionFinding
	byPath := map[string][]ExtensionFinding{}
	for _, f := range findings {
		if !inScope[f.Path] {
			breaches = append(breaches, f)
			continue
		}
		byPath[f.Path] = append(byPath[f.Path], f)
	}
	if len(breaches) > 0 {
		return containExtensionApplicabilityBreach(r, params.Uses, selected, excluded, breaches)
	}

	var out []Evaluation
	for _, path := range selected {
		subject, err := rule.FileSubject(path)
		if err != nil {
			return nil, nil, fmt.Errorf("extension: %w", err)
		}
		reported := byPath[path]
		sort.SliceStable(reported, func(i, j int) bool {
			if reported[i].Line != reported[j].Line {
				return reported[i].Line < reported[j].Line
			}
			return reported[i].Message < reported[j].Message
		})
		var vs []Violation
		for _, f := range reported {
			v, err := newViolation(r, subject, path, f.Line, f.Message, f.Remediation)
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
	return out, nil, err
}

// containExtensionApplicabilityBreach discards untrustworthy Extension
// findings and records failed selected subjects plus operational
// Diagnostics. When nothing was selected, only the Diagnostics are
// returned — no fabricated Evaluation.
func containExtensionApplicabilityBreach(r rule.Rule, extension string,
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
		msg := fmt.Sprintf("rule %s: extension %q reported %q, which is outside the rule's applicability",
			r.ID(), extension, f.Path)
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
