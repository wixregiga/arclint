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
func evaluateExtensionRule(r rule.Rule, mem membership, obs Observations,
	evaluator ExtensionEvaluator, modules []rule.Module,
) ([]Evaluation, error) {
	params, ok := r.Params().(rule.ExtensionParams)
	if !ok {
		return nil, fmt.Errorf("rule %s: extension rule with %T params", r.ID(), r.Params())
	}
	if evaluator == nil {
		return evaluateUnsupported(r, mem)
	}
	selected, excluded := partitionFiles(r, mem)
	findings, err := evaluator.Evaluate(params.Uses, params.With, selected, modules, obs)
	if err != nil {
		return nil, fmt.Errorf("rule %s: %v", r.ID(), err)
	}

	inScope := map[string]bool{}
	for _, f := range selected {
		inScope[f] = true
	}
	byPath := map[string][]ExtensionFinding{}
	for _, f := range findings {
		if !inScope[f.Path] {
			return nil, fmt.Errorf("rule %s: extension %q reported %q, which is outside the rule's applicability",
				r.ID(), params.Uses, f.Path)
		}
		byPath[f.Path] = append(byPath[f.Path], f)
	}

	var out []Evaluation
	for _, path := range selected {
		subject, err := rule.FileSubject(path)
		if err != nil {
			return nil, fmt.Errorf("extension: %w", err)
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
				return nil, err
			}
			vs = append(vs, v)
		}
		e, err := completeEvaluation(r, subject, vs)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return appendNotApplicable(out, r, excluded)
}
