// Package conformance holds the Conformance Check (the deterministic
// domain operation evaluating applicable enabled Rules over supplied
// Observations) and its immutable results: Rule Evaluations,
// Diagnostics, Violations, and the complete Conformance Assessment.
// The check never reports conformance when evaluation was unsupported,
// undetermined, or failed.
package conformance

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// Request carries everything one Conformance Check evaluates: the
// configured Rules (with their Exclusions, Suppressions, and
// Disablements attached), the declared Modules, the Observations, the
// unknown-import policy, the Extension enforcement mechanism, and the
// project's recorded domain model for read-only extension access.
type Request struct {
	Rules          []rule.Rule
	Modules        []rule.Module
	Observations   Observations
	UnknownImports rule.UnknownImportPolicy
	// Extensions supplies Extension enforcement; nil makes every
	// extension-rule subject evaluate unsupported, honestly.
	Extensions ExtensionEvaluator
	// Knowledge is the project's recorded domain model; empty when the
	// project records none. Extensions may read it; declaring knowledge
	// never creates a diagnostic by itself.
	Knowledge vocab.UbiquitousLanguage
}

// Run evaluates every applicable enabled Rule or records why it could
// not, applies Exclusions before evaluation and Suppressions after,
// and produces one complete Conformance Assessment with deterministic
// ordering. It never writes or refactors application source code.
func Run(req Request) (Assessment, error) {
	policy := req.UnknownImports
	if policy == "" {
		policy = rule.UnknownImportsWarn
	}
	switch policy {
	case rule.UnknownImportsError, rule.UnknownImportsWarn, rule.UnknownImportsIgnore:
	default:
		return Assessment{}, fmt.Errorf("conformance: unknown_imports policy %q invalid", policy)
	}

	mem, err := newMembership(req.Modules, req.Observations)
	if err != nil {
		return Assessment{}, err
	}
	rules, err := validRules(req.Rules, mem)
	if err != nil {
		return Assessment{}, err
	}

	var evals []Evaluation
	var diags []Diagnostic
	var applied []string
	for _, r := range rules {
		if d, ok := r.Disablement(); ok {
			diag, err := NewCoverage(r.ID().Qualified(),
				fmt.Sprintf("rule %s is disabled: %s; not evaluated", r.ID(), d.Reason()))
			if err != nil {
				return Assessment{}, err
			}
			diags = append(diags, diag)
			continue
		}
		applied = append(applied, r.ID().Qualified())
		var es []Evaluation
		var ruleDiags []Diagnostic
		switch {
		case !r.Enforcement().CanEvaluate():
			es, err = evaluateUnsupported(r, mem)
		case r.Type() == rule.TypeConsumes:
			es, err = evaluateConsumes(r, mem, req.Observations)
		case r.Type() == rule.TypeStructure:
			es, err = evaluateStructure(r, mem)
		case r.Type() == rule.TypeNaming:
			es, err = evaluateNaming(r, mem)
		case r.Type() == rule.TypeContent:
			es, err = evaluateContent(r, mem, req.Observations)
		case r.Type() == rule.TypeExtension:
			es, ruleDiags, err = evaluateExtensionRule(r, mem, req.Observations, req.Extensions, req.Modules, req.Knowledge)
		case r.Type() == rule.TypeLayers, r.Type() == rule.TypeProtected, r.Type() == rule.TypeIndependence, r.Type() == rule.TypeAcyclic:
			es, err = evaluateGraph(r, mem, req.Observations)
		case r.Type() == rule.TypeInvariants:
			es, err = evaluateInvariants(r, mem, req.Observations, req.Knowledge)
		default:
			es, err = evaluateUnsupported(r, mem)
		}
		if err != nil {
			return Assessment{}, err
		}
		evals = append(evals, es...)
		diags = append(diags, ruleDiags...)
	}

	opDiags, err := observationDiagnostics(req.Observations, policy)
	if err != nil {
		return Assessment{}, err
	}
	diags = append(diags, opDiags...)
	summaries, err := coverageSummaries(evals)
	if err != nil {
		return Assessment{}, err
	}
	diags = append(diags, summaries...)
	return NewAssessment(evals, diags, applied)
}

// validRules re-proves every Rule, rejects duplicate identities, and
// rejects references to undeclared Modules: a Rule bound to nothing
// must fail loudly, never evaluate vacuously.
func validRules(rules []rule.Rule, mem membership) ([]rule.Rule, error) {
	seen := map[string]bool{}
	out := append([]rule.Rule(nil), rules...)
	for _, r := range out {
		if err := r.Validate(); err != nil {
			return nil, fmt.Errorf("conformance: %w", err)
		}
		q := r.ID().Qualified()
		if seen[q] {
			return nil, fmt.Errorf("conformance: duplicate rule id %q", q)
		}
		seen[q] = true
		for _, m := range r.ReferencedModules() {
			if _, ok := mem.modules[m]; !ok {
				return nil, fmt.Errorf("conformance: rule %s references undeclared module %q", r.ID(), m)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ID().Qualified() < out[j].ID().Qualified()
	})
	return out, nil
}

// membership resolves Module membership over the Observations once,
// for every evaluator.
type membership struct {
	names       []rule.ModuleName // sorted
	modules     map[rule.ModuleName]rule.Module
	files       []string                     // sorted repo-relative paths
	fileModules map[string][]rule.ModuleName // sorted per file
	moduleFiles map[rule.ModuleName][]string // path order
	dirModules  map[string][]rule.ModuleName // sorted per directory
}

func newMembership(modules []rule.Module, obs Observations) (membership, error) {
	mem := membership{
		modules:     map[rule.ModuleName]rule.Module{},
		fileModules: map[string][]rule.ModuleName{},
		moduleFiles: map[rule.ModuleName][]string{},
		dirModules:  map[string][]rule.ModuleName{},
	}
	for _, m := range modules {
		if _, ok := mem.modules[m.Name()]; ok {
			return membership{}, fmt.Errorf("conformance: duplicate module %q", m.Name())
		}
		mem.modules[m.Name()] = m
		mem.names = append(mem.names, m.Name())
	}
	sort.Slice(mem.names, func(i, j int) bool { return mem.names[i] < mem.names[j] })

	dirSets := map[string]map[rule.ModuleName]bool{}
	for _, f := range obs.Files() {
		mem.files = append(mem.files, f.Path)
		for _, name := range mem.names {
			if mem.modules[name].Contains(f.Path) {
				mem.fileModules[f.Path] = append(mem.fileModules[f.Path], name)
				mem.moduleFiles[name] = append(mem.moduleFiles[name], f.Path)
			}
		}
		if mods := mem.fileModules[f.Path]; len(mods) > 0 {
			d := path.Dir(f.Path)
			set := dirSets[d]
			if set == nil {
				set = map[rule.ModuleName]bool{}
				dirSets[d] = set
			}
			for _, m := range mods {
				set[m] = true
			}
		}
	}
	for d, set := range dirSets {
		names := make([]rule.ModuleName, 0, len(set))
		for m := range set {
			names = append(names, m)
		}
		sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
		mem.dirModules[d] = names
	}
	return mem, nil
}

// targetModules resolves the declared Modules an internal import lands
// in: file-granular targets through the file's own membership,
// package-granular targets through the directory union.
func (m membership) targetModules(imp Import) []rule.ModuleName {
	if imp.TargetFile != "" {
		return m.fileModules[imp.TargetFile]
	}
	if imp.TargetDir != "" {
		return m.dirModules[imp.TargetDir]
	}
	return nil
}

// partitionFiles splits the observed files into the Rule's selected
// Subjects and the subjects an Exclusion removed.
func partitionFiles(r rule.Rule, mem membership) (selected, excluded []string) {
	for _, f := range mem.files {
		if !r.Applicability().WouldSelectFile(f, mem.fileModules[f]) {
			continue
		}
		if r.Applicability().ExcludedFile(f) {
			excluded = append(excluded, f)
			continue
		}
		selected = append(selected, f)
	}
	return selected, excluded
}

// partitionModuleFiles is partitionFiles narrowed to one Module's
// member files, for evaluators that judge per Module.
func partitionModuleFiles(r rule.Rule, name rule.ModuleName, mem membership) (selected, excluded []string) {
	for _, f := range mem.moduleFiles[name] {
		if !r.Applicability().WouldSelectFile(f, mem.fileModules[f]) {
			continue
		}
		if r.Applicability().ExcludedFile(f) {
			excluded = append(excluded, f)
			continue
		}
		selected = append(selected, f)
	}
	return selected, excluded
}

// applySuppressions relabels Violations the Rule's Suppressions match.
// The Evaluation Outcome is unchanged: a suppressed Violation remains
// a Violation.
func applySuppressions(r rule.Rule, vs []Violation) ([]Violation, error) {
	for i, v := range vs {
		reason, ok := r.SuppressionFor(v.Path())
		if !ok {
			continue
		}
		suppressed, err := v.WithStatus(StatusSuppressed, reason)
		if err != nil {
			return nil, err
		}
		vs[i] = suppressed
	}
	return vs, nil
}

// violationOutcome maps evidence strength to the honest violation
// outcome: exact and partial evidence prove a break, weaker evidence
// only suspects one.
func violationOutcome(a rule.Assurance) Outcome {
	switch a {
	case rule.AssuranceExact, rule.AssurancePartial:
		return OutcomeViolates
	default:
		return OutcomeSuspectedViolation
	}
}

// completeEvaluation builds the Evaluation for one subject from its
// (possibly empty) violations, honoring what the Rule's Assurance can
// justify: no findings become conforms only when the evidence permits,
// undetermined otherwise.
func completeEvaluation(r rule.Rule, subject rule.Subject, vs []Violation) (Evaluation, error) {
	vs, err := applySuppressions(r, vs)
	if err != nil {
		return Evaluation{}, err
	}
	outcome := OutcomeConforms
	if len(vs) > 0 {
		outcome = violationOutcome(r.Enforcement().Assurance())
	} else if !r.Enforcement().Assurance().PermitsConformance() {
		outcome = OutcomeUndetermined
	}
	return NewEvaluation(r.ID(), subject, outcome, r.Enforcement(), vs)
}

// simpleEvaluation builds a violation-less Evaluation with an explicit
// outcome (unsupported, not_applicable, failed).
func simpleEvaluation(r rule.Rule, subject rule.Subject, outcome Outcome) (Evaluation, error) {
	return NewEvaluation(r.ID(), subject, outcome, r.Enforcement(), nil)
}

// newViolation builds one Violation for a Rule with the Rule's
// Severity and its Enforcement's evidence qualification.
func newViolation(r rule.Rule, subject rule.Subject, anchor string, line int, message, remediation string) (Violation, error) {
	return NewViolation(ViolationSpec{
		Rule:        r.ID(),
		Subject:     subject,
		Outcome:     violationOutcome(r.Enforcement().Assurance()),
		Severity:    r.Severity(),
		Assurance:   r.Enforcement().Assurance(),
		Evidence:    r.Enforcement().Evidence(),
		Message:     message,
		Remediation: remediation,
		Path:        anchor,
		Line:        line,
	})
}

// observationDiagnostics surfaces analysis failures and the
// unknown-import policy as operational Diagnostics.
func observationDiagnostics(obs Observations, policy rule.UnknownImportPolicy) ([]Diagnostic, error) {
	var out []Diagnostic
	for _, f := range obs.Files() {
		facts, ok := obs.FactsFor(f.Path)
		if !ok {
			continue
		}
		if facts.ParseFailure != "" {
			d, err := NewOperational("", f.Path, 0, rule.SeverityWarning,
				fmt.Sprintf("analysis failed, file skipped: %s", facts.ParseFailure))
			if err != nil {
				return nil, err
			}
			out = append(out, d)
			continue
		}
		if policy == rule.UnknownImportsIgnore {
			continue
		}
		severity := rule.SeverityWarning
		if policy == rule.UnknownImportsError {
			severity = rule.SeverityError
		}
		for _, imp := range facts.Imports {
			if imp.Class != ImportUnknown {
				continue
			}
			d, err := NewOperational("", f.Path, imp.Line, severity,
				fmt.Sprintf("import %q is neither stdlib, internal, nor declared in the dependency manifest", imp.Path))
			if err != nil {
				return nil, err
			}
			out = append(out, d)
		}
	}
	return out, nil
}

// coverageSummaries reports unsupported and failed evaluations per
// Rule, so silence never reads as conformance.
func coverageSummaries(evals []Evaluation) ([]Diagnostic, error) {
	type tally struct {
		unsupported, failed int
		limitations         []string
	}
	byRule := map[string]*tally{}
	var order []string
	for _, e := range evals {
		q := e.Rule().Qualified()
		t := byRule[q]
		if t == nil {
			t = &tally{}
			byRule[q] = t
			order = append(order, q)
		}
		switch e.Outcome() {
		case OutcomeUnsupported:
			t.unsupported++
			if t.limitations == nil {
				t.limitations = e.Limitations()
			}
		case OutcomeFailed:
			t.failed++
		case OutcomeConforms, OutcomeViolates, OutcomeSuspectedViolation,
			OutcomeUndetermined, OutcomeNotApplicable:
			// Coverage summarizes only the evaluation gaps.
		}
	}
	sort.Strings(order)
	var out []Diagnostic
	for _, q := range order {
		t := byRule[q]
		if t.unsupported > 0 {
			msg := fmt.Sprintf("rule %s: %d subject(s) could not be evaluated (unsupported)", q, t.unsupported)
			if len(t.limitations) > 0 {
				msg += ": " + strings.Join(t.limitations, "; ")
			}
			d, err := NewCoverage(q, msg)
			if err != nil {
				return nil, err
			}
			out = append(out, d)
		}
		if t.failed > 0 {
			d, err := NewCoverage(q,
				fmt.Sprintf("rule %s: %d subject(s) failed evaluation; see operational diagnostics", q, t.failed))
			if err != nil {
				return nil, err
			}
			out = append(out, d)
		}
	}
	return out, nil
}

// staticPrefix returns the longest glob-free directory prefix of a
// pattern, anchoring structure findings at a real path.
func staticPrefix(pattern string) string {
	segs := strings.Split(pattern, "/")
	var keep []string
	for _, s := range segs {
		if strings.ContainsAny(s, "*?[\\") {
			break
		}
		keep = append(keep, s)
	}
	if len(keep) == 0 {
		return "."
	}
	return path.Join(keep...)
}
