package conformance

import (
	"fmt"
	"path"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// evaluateUnsupported records the honest outcome for a Rule whose
// Enforcement this build cannot perform: every selected subject
// evaluates unsupported, excluded subjects evaluate not-applicable.
func evaluateUnsupported(r rule.Rule, mem membership) ([]Evaluation, error) {
	selected, excluded := partitionFiles(r, mem)
	var out []Evaluation
	for _, f := range selected {
		subject, err := rule.FileSubject(f)
		if err != nil {
			return nil, fmt.Errorf("unsupported: %w", err)
		}
		e, err := simpleEvaluation(r, subject, OutcomeUnsupported)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return appendNotApplicable(out, r, excluded)
}

// evaluateNaming judges every selected file's stem against the Rule's
// case vocabulary.
func evaluateNaming(r rule.Rule, mem membership) ([]Evaluation, error) {
	p, ok := r.Params().(rule.NamingParams)
	if !ok {
		return nil, fmt.Errorf("rule %s: naming rule with %T params", r.ID(), r.Params())
	}
	selected, excluded := partitionFiles(r, mem)
	var out []Evaluation
	for _, f := range selected {
		subject, err := rule.FileSubject(f)
		if err != nil {
			return nil, fmt.Errorf("naming: %w", err)
		}
		var vs []Violation
		if !p.Case.Matches(fileStem(f)) {
			v, err := newViolation(r, subject, f, 0,
				fmt.Sprintf("file name %q violates naming rule %s", path.Base(f), p.Case),
				fmt.Sprintf("rename the file so its stem matches %s", p.Case))
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

// evaluateStructure judges each selected Module: every require glob
// must match a member file, no member file may match a forbid glob.
func evaluateStructure(r rule.Rule, mem membership) ([]Evaluation, error) {
	p, ok := r.Params().(rule.StructureParams)
	if !ok {
		return nil, fmt.Errorf("rule %s: structure rule with %T params", r.ID(), r.Params())
	}
	var out []Evaluation
	for _, name := range sortedModules(r.Applicability().Modules()) {
		subject, err := rule.ModuleSubject(name)
		if err != nil {
			return nil, fmt.Errorf("structure: %w", err)
		}
		if r.Applicability().ExcludedModule(name) {
			e, err := simpleEvaluation(r, subject, OutcomeNotApplicable)
			if err != nil {
				return nil, err
			}
			out = append(out, e)
			continue
		}
		members := mem.moduleFiles[name]
		var vs []Violation
		for _, req := range p.Require {
			found := false
			for _, f := range members {
				if req.Match(f) {
					found = true
					break
				}
			}
			if !found {
				v, err := newViolation(r, subject, staticPrefix(req.String()), 0,
					fmt.Sprintf("Module %q is missing a required file matching %q", name, req),
					fmt.Sprintf("create a file matching %q", req))
				if err != nil {
					return nil, err
				}
				vs = append(vs, v)
			}
		}
		for _, forbid := range p.Forbid {
			for _, f := range members {
				if !forbid.Match(f) {
					continue
				}
				fileSubject, err := rule.FileSubject(f)
				if err != nil {
					return nil, fmt.Errorf("structure: %w", err)
				}
				v, err := NewViolation(ViolationSpec{
					Rule:        r.ID(),
					Subject:     fileSubject,
					Outcome:     violationOutcome(r.Enforcement().Assurance()),
					Severity:    r.Severity(),
					Assurance:   r.Enforcement().Assurance(),
					Evidence:    r.Enforcement().Evidence(),
					Message:     fmt.Sprintf("path forbidden by structure rule %q of Module %q", forbid, name),
					Remediation: "remove or relocate the file",
				})
				if err != nil {
					return nil, err
				}
				vs = append(vs, v)
			}
		}
		e, err := completeEvaluation(r, subject, vs)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// appendNotApplicable records the not-applicable outcome for subjects
// an Exclusion removed: excluded is a decision, never silence.
func appendNotApplicable(out []Evaluation, r rule.Rule, excluded []string) ([]Evaluation, error) {
	for _, f := range excluded {
		subject, err := rule.FileSubject(f)
		if err != nil {
			return nil, fmt.Errorf("excluded: %w", err)
		}
		e, err := simpleEvaluation(r, subject, OutcomeNotApplicable)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func sortedModules(names []rule.ModuleName) []rule.ModuleName {
	out := append([]rule.ModuleName(nil), names...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func fileStem(p string) string {
	base := path.Base(p)
	return strings.TrimSuffix(base, path.Ext(base))
}
