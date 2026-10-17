package conformance

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// evaluateWordLimit judges the built-in word-limit Rule over the
// recorded domain. Its one Subject is the DomainRecording's file, and
// every recorded text over the WordLimit of its kind is one Violation
// anchored at the line the text's holder is written on. A language
// that was not read from a file has nothing to judge, and an Exclusion
// of the file leaves the Rule not applicable.
func evaluateWordLimit(r rule.Rule, knowledge vocab.UbiquitousLanguage) ([]Evaluation, error) {
	if knowledge.Recording.IsZero() {
		return nil, nil
	}
	file := knowledge.Recording.Path()
	subject, err := rule.FileSubject(file)
	if err != nil {
		return nil, fmt.Errorf("rule %s: %w", r.ID(), err)
	}
	if r.Scope().ExcludedFile(file) {
		e, err := simpleEvaluation(r, subject, OutcomeNotApplicable)
		if err != nil {
			return nil, err
		}
		return []Evaluation{e}, nil
	}
	var vs []Violation
	for _, o := range knowledge.OverlongTexts() {
		v, err := newViolation(r, subject, file, o.Line, o.Problem(), o.Remedy())
		if err != nil {
			return nil, fmt.Errorf("rule %s: %w", r.ID(), err)
		}
		vs = append(vs, v)
	}
	e, err := completeEvaluation(r, subject, vs)
	if err != nil {
		return nil, err
	}
	return []Evaluation{e}, nil
}
