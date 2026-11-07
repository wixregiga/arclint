package conformance

import (
	"fmt"
	"slices"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// EvidenceAttributionSpec is the input to EvidenceAttribution
// construction: one assessment, the recorded language it judged, the
// declarations located in the same observations, and the observed
// files that failed to parse.
type EvidenceAttributionSpec struct {
	Knowledge     vocab.UbiquitousLanguage
	Assessment    Assessment
	Carriers      Carriers
	ParseFailures []string
}

// EvidenceAttribution is what one assessment shows about the whole
// recorded language: the EntryEvidence of every entry, and the
// findings and diagnostics no entry can be given, kept apart as
// unattributed.
type EvidenceAttribution struct {
	entries       []EntryEvidence
	unattributed  []Violation
	diagnostics   []Diagnostic
	notEvaluated  []string
	parseFailures []string
}

// findingKey identifies one finding of the assessment the way
// Assessment.RelabelViolations does.
type findingKey struct {
	fingerprint, path string
	line              int
}

func keyOf(v Violation) findingKey {
	return findingKey{fingerprint: v.Fingerprint(), path: v.Path(), line: v.Line()}
}

// NewEvidenceAttribution attributes every finding of the assessment to
// the recorded entries its structure ties it to, and keeps the rest as
// unattributed. A question records no code and gets no evidence; a
// finding on its recorded line is unattributed.
func NewEvidenceAttribution(s EvidenceAttributionSpec) (EvidenceAttribution, error) {
	b := attributionBuilder{
		spec:          s,
		recording:     s.Knowledge.Recording.Path(),
		coverage:      map[string]ContractCoverage{},
		contextChecks: map[string][]Partiality{},
		fileChecks:    map[string][]Partiality{},
	}
	b.indexAssessment()
	for _, ctx := range s.Knowledge.Contexts {
		if err := b.addContext(ctx); err != nil {
			return EvidenceAttribution{}, err
		}
	}
	return b.build()
}

type attributionBuilder struct {
	spec      EvidenceAttributionSpec
	recording string
	coverage  map[string]ContractCoverage
	// global is the partiality every entry shares: Rules an Extension
	// stopped and files that failed to parse.
	global        []Partiality
	contextChecks map[string][]Partiality
	fileChecks    map[string][]Partiality
	specs         []EntryEvidenceSpec
}

func contractID(context string, kind vocab.Concept, owner, key string) string {
	return context + "\x1f" + string(kind) + "\x1f" + owner + "\x1f" + key
}

// indexAssessment reads what the assessment could not decide, and the
// coverage of every recorded contract.
func (b *attributionBuilder) indexAssessment() {
	a := b.spec.Assessment
	for _, id := range a.NotEvaluatedRules() {
		b.global = append(b.global, Partiality{Kind: PartialAssessmentIncomplete, Rule: id})
	}
	if len(b.spec.ParseFailures) > 0 {
		failed := slices.Clone(b.spec.ParseFailures)
		slices.Sort(failed)
		b.global = append(b.global, Partiality{Kind: PartialParseFailed, Paths: slices.Compact(failed)})
	}
	for _, c := range a.Contracts() {
		b.coverage[contractID(c.Context(), c.Kind(), c.Owner(), c.Key())] = c
	}
	for _, e := range a.Evaluations() {
		switch e.Outcome() {
		case OutcomeUndetermined, OutcomeUnsupported, OutcomeFailed:
		default:
			continue
		}
		p := Partiality{Kind: PartialCheckUndecided, Rule: e.Rule().Qualified(), Outcome: string(e.Outcome())}
		switch e.Subject().Kind() {
		case rule.SubjectContext:
			b.contextChecks[e.Subject().Identity()] = appendPartial(b.contextChecks[e.Subject().Identity()], p)
		case rule.SubjectFile:
			b.fileChecks[e.Subject().Identity()] = appendPartial(b.fileChecks[e.Subject().Identity()], p)
		case rule.SubjectFolder, rule.SubjectZone:
		}
	}
}

func appendPartial(ps []Partiality, p Partiality) []Partiality {
	for _, have := range ps {
		if have.Kind == p.Kind && have.Rule == p.Rule && have.Outcome == p.Outcome {
			return ps
		}
	}
	return append(ps, p)
}

func (b *attributionBuilder) addContext(ctx vocab.BoundedContext) error {
	name := ctx.Name
	b.add(EntryEvidenceSpec{Entry: EntryAddress{Concept: vocab.ConceptBoundedContext, Context: name, Name: name}, RecordedLine: ctx.Line, Declarations: DeclarationNotApplicable})
	for _, agg := range ctx.Aggregates {
		var contracts []ContractEvidence
		for _, inv := range agg.Invariants {
			c, err := b.invariant(name, agg.Name, vocab.ConceptAggregate, inv.Key)
			if err != nil {
				return err
			}
			contracts = append(contracts, c)
		}
		for _, as := range agg.Assertions {
			c, err := b.assertion(name, agg.Name, vocab.ConceptAggregate, as.Key)
			if err != nil {
				return err
			}
			contracts = append(contracts, c)
		}
		b.addTerm(name, vocab.ConceptAggregate, "", agg.Name, agg.Line, contracts)
		for _, ent := range agg.Entities {
			b.addTerm(name, vocab.ConceptEntity, agg.Name, ent.Name, ent.Line, nil)
		}
		for i, inv := range agg.Invariants {
			b.addContract(name, vocab.ConceptInvariant, agg.Name, inv.Key, inv.Line, contracts[i])
		}
		for i, as := range agg.Assertions {
			b.addContract(name, vocab.ConceptAssertion, agg.Name, as.Key, as.Line, contracts[len(agg.Invariants)+i])
		}
	}
	for _, vo := range ctx.ValueObjects {
		var contracts []ContractEvidence
		for _, inv := range vo.Invariants {
			c, err := b.invariant(name, vo.Name, vocab.ConceptValueObject, inv.Key)
			if err != nil {
				return err
			}
			contracts = append(contracts, c)
		}
		b.addTerm(name, vocab.ConceptValueObject, "", vo.Name, vo.Line, contracts)
		for i, inv := range vo.Invariants {
			b.addContract(name, vocab.ConceptInvariant, vo.Name, inv.Key, inv.Line, contracts[i])
		}
	}
	for _, ev := range ctx.Events {
		b.addTerm(name, vocab.ConceptDomainEvent, "", ev.Name, ev.Line, nil)
	}
	for _, svc := range ctx.Services {
		var contracts []ContractEvidence
		for _, as := range svc.Assertions {
			c, err := b.assertion(name, svc.Name, vocab.ConceptDomainService, as.Key)
			if err != nil {
				return err
			}
			contracts = append(contracts, c)
		}
		b.addTerm(name, vocab.ConceptDomainService, "", svc.Name, svc.Line, contracts)
		for i, as := range svc.Assertions {
			b.addContract(name, vocab.ConceptAssertion, svc.Name, as.Key, as.Line, contracts[i])
		}
	}
	for _, spec := range ctx.Specifications {
		b.addTerm(name, vocab.ConceptSpecification, "", spec.Name, spec.Line, nil)
	}
	return nil
}

// invariant is one invariant with the declaration carrying it, located
// as the built-in rules locate it: a value object's constructor, an
// aggregate root's ensure method.
func (b *attributionBuilder) invariant(ctx, owner string, ownerConcept vocab.Concept, key string) (ContractEvidence, error) {
	c := ContractEvidence{Kind: vocab.ConceptInvariant, Owner: owner, OwnerConcept: ownerConcept, Key: key, CodeRead: b.spec.Carriers.ReadsCode(ctx)}
	if ownerConcept == vocab.ConceptValueObject {
		c.Carrier, c.Located = b.spec.Carriers.Constructor(ctx, owner)
	} else {
		at, found, err := b.spec.Carriers.Invariant(ctx, owner, key)
		if err != nil {
			return ContractEvidence{}, fmt.Errorf("evidence: invariant %s of %s: %w", key, owner, err)
		}
		c.Carrier, c.Located = at, found
	}
	c.Coverage, c.Covered = b.coverage[contractID(ctx, vocab.ConceptInvariant, owner, key)]
	return c, nil
}

// assertion is one assertion with its checking method on the owner.
func (b *attributionBuilder) assertion(ctx, owner string, ownerConcept vocab.Concept, key string) (ContractEvidence, error) {
	at, found, err := b.spec.Carriers.Assertion(ctx, owner, key)
	if err != nil {
		return ContractEvidence{}, fmt.Errorf("evidence: assertion %s of %s: %w", key, owner, err)
	}
	c := ContractEvidence{Kind: vocab.ConceptAssertion, Owner: owner, OwnerConcept: ownerConcept, Key: key, Carrier: at, Located: found, CodeRead: b.spec.Carriers.ReadsCode(ctx)}
	c.Coverage, c.Covered = b.coverage[contractID(ctx, vocab.ConceptAssertion, owner, key)]
	return c, nil
}

// addTerm adds an entry a type declaration spells.
func (b *attributionBuilder) addTerm(ctx string, concept vocab.Concept, owner, name string, line int, contracts []ContractEvidence) {
	locations := b.spec.Carriers.Declarations(ctx, name)
	state := DeclarationMissing
	switch {
	case len(locations) == 1:
		state = DeclarationLocated
	case len(locations) > 1:
		state = DeclarationAmbiguous
	case !b.spec.Carriers.ReadsCode(ctx):
		state = DeclarationNotObserved
	}
	b.add(EntryEvidenceSpec{
		Entry:        EntryAddress{Concept: concept, Context: ctx, Owner: owner, Name: name},
		RecordedLine: line, Declarations: state, Locations: locations, Contracts: contracts,
	})
}

// addContract adds an invariant or an assertion entry: its evidence is
// the contract it is.
func (b *attributionBuilder) addContract(ctx string, concept vocab.Concept, owner, key string, line int, contract ContractEvidence) {
	b.add(EntryEvidenceSpec{
		Entry:        EntryAddress{Concept: concept, Context: ctx, Owner: owner, Name: key},
		RecordedLine: line, Declarations: DeclarationNotApplicable, Contracts: []ContractEvidence{contract},
	})
}

// add completes an entry's spec with the partiality it shares: the
// whole assessment's, its context's undecided checks, and the
// undecided checks of the files declaring it.
func (b *attributionBuilder) add(s EntryEvidenceSpec) {
	s.Recording = b.recording
	s.Partial = slices.Clone(b.global)
	if !b.spec.Carriers.ReadsCode(s.Entry.Context) && s.Declarations == DeclarationNotApplicable {
		s.Partial = append(s.Partial, Partiality{Kind: PartialNotObserved})
	}
	for _, p := range b.contextChecks[s.Entry.Context] {
		s.Partial = appendPartial(s.Partial, p)
	}
	for _, l := range s.Locations {
		for _, p := range b.fileChecks[l.Path] {
			s.Partial = appendPartial(s.Partial, p)
		}
	}
	b.specs = append(b.specs, s)
}

// build attributes every finding and constructs the evidence.
func (b *attributionBuilder) build() (EvidenceAttribution, error) {
	byLine := map[int][]int{}
	byFile := map[string][]int{}
	byContext := map[string]int{}
	byContract := map[findingKey][]int{}
	for i, s := range b.specs {
		if s.RecordedLine > 0 {
			byLine[s.RecordedLine] = append(byLine[s.RecordedLine], i)
		}
		for _, l := range s.Locations {
			byFile[l.Path] = append(byFile[l.Path], i)
		}
		if s.Entry.Concept == vocab.ConceptBoundedContext {
			byContext[s.Entry.Context] = i
		}
		for _, c := range s.Contracts {
			if !c.Covered {
				continue
			}
			for _, check := range c.Coverage.Checks() {
				for _, held := range check.Violations() {
					k := keyOf(held)
					if !slices.Contains(byContract[k], i) {
						byContract[k] = append(byContract[k], i)
					}
				}
			}
		}
	}
	out := EvidenceAttribution{notEvaluated: b.spec.Assessment.NotEvaluatedRules()}
	for _, d := range b.spec.Assessment.Diagnostics() {
		if d.Kind() != DiagnosticViolation {
			out.diagnostics = append(out.diagnostics, d)
		}
	}
	for _, p := range b.global {
		if p.Kind == PartialParseFailed {
			out.parseFailures = slices.Clone(p.Paths)
		}
	}
	findings := make([][]AttributedFinding, len(b.specs))
	violations := b.spec.Assessment.Violations()
	for _, v := range violations {
		bases := map[int][]EvidenceBasis{}
		tie := func(i int, basis EvidenceBasis) {
			if !slices.Contains(bases[i], basis) {
				bases[i] = append(bases[i], basis)
			}
		}
		for _, i := range byContract[keyOf(v)] {
			tie(i, BasisContract)
		}
		if v.Path() == b.recording && v.Line() > 0 {
			for _, i := range byLine[v.Line()] {
				tie(i, BasisRecordedLine)
			}
		}
		if v.Subject().Kind() == rule.SubjectContext {
			if i, ok := byContext[v.Subject().Identity()]; ok {
				tie(i, BasisBoundedContext)
			}
		}
		for _, i := range byFile[v.Path()] {
			tie(i, BasisDeclaringFile)
		}
		if len(bases) == 0 {
			out.unattributed = append(out.unattributed, v)
			continue
		}
		for i := range b.specs {
			if bs, ok := bases[i]; ok {
				findings[i] = append(findings[i], AttributedFinding{Violation: v, Bases: bs})
			}
		}
	}
	for i, s := range b.specs {
		s.Findings = findings[i]
		e, err := NewEntryEvidence(s)
		if err != nil {
			return EvidenceAttribution{}, err
		}
		out.entries = append(out.entries, e)
	}
	if err := out.ensureAccountsForEveryFinding(violations); err != nil {
		return EvidenceAttribution{}, err
	}
	return out, nil
}

// ensureAccountsForEveryFinding refuses an attribution that drops a
// finding: each one is held by an entry's evidence or is unattributed.
func (a EvidenceAttribution) ensureAccountsForEveryFinding(violations []Violation) error {
	held := map[findingKey]bool{}
	for _, e := range a.entries {
		for _, f := range e.findings {
			held[keyOf(f.Violation)] = true
		}
	}
	for _, v := range a.unattributed {
		held[keyOf(v)] = true
	}
	for _, v := range violations {
		if !held[keyOf(v)] {
			return fmt.Errorf("evidence attribution: finding %s at %s:%d is neither attributed nor unattributed", v.Rule().Qualified(), v.Path(), v.Line())
		}
	}
	return nil
}

// Entries is the evidence of every recorded entry, in recording order.
func (a EvidenceAttribution) Entries() []EntryEvidence { return slices.Clone(a.entries) }

// Unattributed are the findings no entry's structure ties them to.
func (a EvidenceAttribution) Unattributed() []Violation { return slices.Clone(a.unattributed) }

// Diagnostics are the assessment's operational, coverage and
// not-evaluated diagnostics; none belongs to one entry.
func (a EvidenceAttribution) Diagnostics() []Diagnostic { return slices.Clone(a.diagnostics) }

// NotEvaluatedRules are the Rules an Extension stopped.
func (a EvidenceAttribution) NotEvaluatedRules() []string { return slices.Clone(a.notEvaluated) }

// ParseFailures are the observed files that failed to parse.
func (a EvidenceAttribution) ParseFailures() []string { return slices.Clone(a.parseFailures) }

// Complete reports whether the assessment decided everything it ran:
// no Rule was stopped and no file failed to parse.
func (a EvidenceAttribution) Complete() bool {
	return len(a.notEvaluated) == 0 && len(a.parseFailures) == 0
}
