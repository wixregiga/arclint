package conformance

import (
	"fmt"
	"slices"
	"sort"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// evaluateExtensionRule delegates one Rule to its Extension through
// the evaluator port. The extension sees repository-wide evidence and
// the resolved Scope separately; without a mechanism every subject honestly
// evaluates unsupported. Heuristic assurance keeps the outcomes
// honest: findings are suspected violations, absence of findings is
// undetermined, never conformance.
//
// A finding that names a subject outside the selected files is a
// Scope breach: every finding from that Extension run is
// discarded as untrustworthy, each selected subject evaluates failed,
// excluded subjects stay not-applicable, and error-severity
// operational Diagnostics identify the breach. The check still returns
// a complete Assessment (no error) so other Rules keep reporting.
func evaluateExtensionRule(r rule.Rule, mem membership, obs Observations,
	evaluator ExtensionEvaluator, zones []rule.Zone, knowledge vocab.UbiquitousLanguage,
) ([]Evaluation, []Diagnostic, error) {
	params, ok := r.Params().(rule.ExtensionParams)
	if !ok {
		return nil, nil, fmt.Errorf("rule %s: extension rule with %T params", r.ID(), r.Params())
	}
	if evaluator == nil {
		es, err := evaluateUnsupported(r, mem)
		return es, nil, err
	}
	scope := resolveExtensionScope(r, mem)
	selected, excluded := scope.Files, scope.ExcludedFiles
	findings, err := evaluator.Evaluate(params.Uses, params.With, scope, zones, obs, knowledge)
	if err != nil {
		return nil, nil, fmt.Errorf("rule %s: %v", r.ID(), err)
	}

	inScope := map[string]bool{}
	for _, f := range selected {
		inScope[f] = true
	}
	observed := map[string]bool{}
	for _, file := range obs.Files() {
		observed[file.Path] = true
	}
	var breaches []ExtensionFinding
	byPath := map[string][]ExtensionFinding{}
	for _, f := range findings {
		if f.SubjectPath == "" {
			f.SubjectPath = f.Path
		}
		if !inScope[f.SubjectPath] || !observed[f.Path] {
			breaches = append(breaches, f)
			continue
		}
		byPath[f.SubjectPath] = append(byPath[f.SubjectPath], f)
	}
	if len(breaches) > 0 {
		return containExtensionScopeBreach(r, params.Uses, selected, excluded, breaches)
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
	return out, nil, err
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
		if a.SubjectPath != b.SubjectPath {
			return a.SubjectPath < b.SubjectPath
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
			r.ID(), extension, f.SubjectPath)
		if slices.Contains(selected, f.SubjectPath) {
			msg = fmt.Sprintf("rule %s: extension %q reported evidence %q for subject %q, which is outside the repository observations",
				r.ID(), extension, f.Path, f.SubjectPath)
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

// resolveExtensionScope applies exclusions to evaluation subjects only. Zone
// exclusions remove their member files even when Zones overlap.
func resolveExtensionScope(r rule.Rule, mem membership) ExtensionScope {
	scope := ExtensionScope{Exclusions: r.Scope().Exclusions()}
	for _, path := range mem.files {
		memberOf := mem.fileZones[path]
		if !r.Scope().WouldSelectFile(path, memberOf) {
			continue
		}
		excluded := r.Scope().ExcludedFile(path)
		for _, zone := range memberOf {
			excluded = excluded || r.Scope().ExcludedZone(zone)
		}
		if excluded {
			scope.ExcludedFiles = append(scope.ExcludedFiles, path)
		} else {
			scope.Files = append(scope.Files, path)
		}
	}
	return scope
}
