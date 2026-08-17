package application

import (
	"fmt"
	"strings"
)

// ExplainRule explains one Rule in human- and agent-readable form:
// the same facts for both audiences — what the Rule requires, where it
// applies, how it is enforced, and what a violation means.
type ExplainRule struct {
	show ShowRule
}

// NewExplainRule requires the show use case it projects from.
func NewExplainRule(show ShowRule) (ExplainRule, error) {
	if show == (ShowRule{}) {
		return ExplainRule{}, fmt.Errorf("explain rule: missing show use case")
	}
	return ExplainRule{show: show}, nil
}

// Execute renders the explanation.
func (uc ExplainRule) Execute(id string) (string, error) {
	d, err := uc.show.Execute(id)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "rule %s (%s, severity %s)\n", d.Summary.ID, d.Summary.Type, d.Summary.Severity)
	fmt.Fprintf(&b, "claim: %s\n", d.Summary.Claim)

	switch {
	case d.EntireRepository:
		b.WriteString("applies to: the entire repository\n")
	case len(d.Modules) > 0:
		fmt.Fprintf(&b, "applies to: Module(s) %s", strings.Join(d.Modules, ", "))
		if len(d.Files) > 0 {
			fmt.Fprintf(&b, ", files matching %s", strings.Join(d.Files, ", "))
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "enforcement: %s; assurance %s", d.Evidence, d.Summary.Assurance)
	if len(d.Languages) > 0 {
		fmt.Fprintf(&b, "; languages %s", strings.Join(d.Languages, ", "))
	}
	if len(d.Facts) > 0 {
		fmt.Fprintf(&b, "; requires %s", strings.Join(d.Facts, ", "))
	}
	b.WriteString("\n")
	for _, l := range d.Limitations {
		fmt.Fprintf(&b, "limitation: %s\n", l)
	}

	if d.Summary.Severity == "error" {
		b.WriteString("when violated: fails the gate\n")
	} else {
		fmt.Fprintf(&b, "when violated: reported as %s without failing the gate\n", d.Summary.Severity)
	}

	if d.Summary.Provenance != "" {
		fmt.Fprintf(&b, "provenance: distributed by %s\n", d.Summary.Provenance)
	} else {
		b.WriteString("provenance: repository-local\n")
	}
	for _, e := range d.Exclusions {
		fmt.Fprintf(&b, "excluded: %s (%s)\n", strings.Join(e.Selectors, ", "), e.Reason)
	}
	for _, s := range d.Suppressions {
		fmt.Fprintf(&b, "suppressed: %s (%s)\n", strings.Join(s.Selectors, ", "), s.Reason)
	}
	if d.Summary.Disabled {
		fmt.Fprintf(&b, "disabled: %s\n", d.Summary.DisabledReason)
	}
	b.WriteString("\naccepted configuration:\n")
	b.WriteString(d.Schema)
	return b.String(), nil
}
