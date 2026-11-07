package conformance

import (
	"fmt"
	"slices"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// EvidenceBasis names what ties a finding to a recorded entry. Each is
// read from the finding's structure: the contract coverage that holds
// it, its anchor in the domain recording, its subject, or the file it
// is anchored in. A finding's message text never ties it to an entry.
type EvidenceBasis string

const (
	// BasisContract means the coverage of one of the entry's recorded
	// invariants or assertions holds the finding: the entry is that
	// contract or its owner.
	BasisContract EvidenceBasis = "contract"
	// BasisRecordedLine means the finding is anchored in the domain
	// recording at the line the entry is written on.
	BasisRecordedLine EvidenceBasis = "recorded_line"
	// BasisBoundedContext means the finding's subject is the bounded
	// context, and the entry is that context.
	BasisBoundedContext EvidenceBasis = "bounded_context"
	// BasisDeclaringFile means the finding is anchored in a file that
	// declares the entry. The finding is about that file; the entry is
	// one of the declarations in it.
	BasisDeclaringFile EvidenceBasis = "declaring_file"
)

// DeclarationState says how the entry's code was found. Located means
// one declaration spells the entry, which is presence, never proof that
// the code honours what the entry records.
type DeclarationState string

const (
	// DeclarationLocated means exactly one declaration spells the entry.
	DeclarationLocated DeclarationState = "located"
	// DeclarationAmbiguous means several declarations spell the entry
	// and nothing tells which is the model's.
	DeclarationAmbiguous DeclarationState = "ambiguous"
	// DeclarationMissing means the context's code was read and no
	// declaration spells the entry.
	DeclarationMissing DeclarationState = "missing"
	// DeclarationNotObserved means none of the context's code was read
	// in a language ArcLint analyzes, so nothing says it is missing.
	DeclarationNotObserved DeclarationState = "not_observed"
	// DeclarationNotApplicable means the entry is not spelled by one
	// declaration: a bounded context, an invariant or an assertion.
	DeclarationNotApplicable DeclarationState = "not_applicable"
)

// EvidenceReading is how an entry's evidence reads as a whole.
type EvidenceReading string

const (
	// ReadingFindings means at least one active finding is attributed.
	ReadingFindings EvidenceReading = "findings"
	// ReadingPartial means no active finding is attributed, but the
	// assessment could not look everywhere it would have needed to.
	ReadingPartial EvidenceReading = "partial"
	// ReadingNoAttributedFindings means the complete assessment attributes
	// no active finding to the entry. It says nothing of the findings no
	// entry can be given, and it is not proof of correctness: the
	// checks are structural.
	ReadingNoAttributedFindings EvidenceReading = "no_attributed_findings"
)

// PartialKind names why an assessment could not decide everything it
// would have needed to about an entry.
type PartialKind string

const (
	// PartialAssessmentIncomplete means an Extension stopped a Rule, so
	// the assessment has no result for it.
	PartialAssessmentIncomplete PartialKind = "assessment_incomplete"
	// PartialParseFailed means a file failed to parse and was skipped.
	PartialParseFailed PartialKind = "parse_failed"
	// PartialContractUndecided means a check of one of the entry's
	// contracts did not decide, or did not run.
	PartialContractUndecided PartialKind = "contract_undecided"
	// PartialCheckUndecided means a Rule judging the entry's bounded
	// context, or a file declaring the entry, did not decide.
	PartialCheckUndecided PartialKind = "check_undecided"
	// PartialNotObserved means none of the entry's context's code was
	// read in a language ArcLint analyzes.
	PartialNotObserved PartialKind = "not_observed"
)

// Partiality is one reason an entry's evidence is partial, with the
// Rule, contract or files it concerns.
type Partiality struct {
	Kind PartialKind
	// Rule is the qualified Rule identity when one Rule did not decide.
	Rule string
	// Contract is the key of the contract that did not decide.
	Contract string
	// Outcome is what the undecided check concluded, or the contract's
	// coverage status.
	Outcome string
	// Paths are the files concerned: the files that failed to parse.
	Paths []string
}

// EntryAddress names one recorded entry as `arclint domain show` names
// it: its concept, bounded context, the owner it is recorded under when
// nested, and its name or key.
type EntryAddress struct {
	Concept vocab.Concept
	Context string
	Owner   string
	Name    string
}

// AttributedFinding is one finding of the assessment with every basis
// that ties it to the entry.
type AttributedFinding struct {
	Violation Violation
	Bases     []EvidenceBasis
}

// ContractEvidence is one recorded invariant or assertion of the entry:
// the declaration that carries it when one was located, and its
// coverage when the assessment checked it.
type ContractEvidence struct {
	Kind         vocab.Concept
	Owner        string
	OwnerConcept vocab.Concept
	Key          string
	// Carrier is the declaration carrying the contract; Located is false
	// when none was found.
	Carrier Carrier
	Located bool
	// CodeRead is false when none of the context's code was read in a
	// language ArcLint analyzes, so an unlocated carrier is not missing.
	CodeRead bool
	// Coverage is the assessment's coverage of the contract; Covered is
	// false when the assessment reported none.
	Coverage ContractCoverage
	Covered  bool
}

// CoverageStatus is the coverage's status, or "unchecked" when the
// assessment reported no coverage for the contract.
func (c ContractEvidence) CoverageStatus() string {
	if !c.Covered {
		return "unchecked"
	}
	return c.Coverage.Status()
}

// decided reports whether every check of the contract ran and reached
// a conclusion. The coverage status names the most severe outcome, so a
// violation can hide a check that was not performed or did not decide;
// each check and every unperformed one is read instead.
func (c ContractEvidence) decided() bool {
	if !c.Covered || len(c.Coverage.Checks()) == 0 || len(c.Coverage.Unperformed()) > 0 {
		return false
	}
	for _, e := range c.Coverage.Checks() {
		switch e.Outcome() {
		case OutcomeConforms, OutcomeViolates, OutcomeSuspectedViolation, OutcomeNotApplicable:
		case OutcomeUndetermined, OutcomeUnsupported, OutcomeFailed:
			return false
		default:
			return false
		}
	}
	return true
}

// undecidedOutcome names why the contract is not decided: the coverage
// status, or incomplete when a check was not performed beside others
// that decided.
func (c ContractEvidence) undecidedOutcome() string {
	if c.Covered && len(c.Coverage.Unperformed()) > 0 {
		return "incomplete"
	}
	return c.CoverageStatus()
}

// EntryEvidenceSpec is the input to EntryEvidence construction.
type EntryEvidenceSpec struct {
	Entry EntryAddress
	// Recording is the domain recording's path; RecordedLine the line
	// the entry is written on there, 0 when unknown.
	Recording    string
	RecordedLine int
	Declarations DeclarationState
	Locations    []Carrier
	Contracts    []ContractEvidence
	Findings     []AttributedFinding
	Partial      []Partiality
}

// EntryEvidence is what one assessment shows about one recorded entry:
// the declarations located for it or missing, the coverage of its
// contracts, and the findings attributed to it. A located declaration
// is presence, never proof the code honours the entry.
type EntryEvidence struct {
	entry        EntryAddress
	recording    string
	recordedLine int
	declarations DeclarationState
	locations    []Carrier
	contracts    []ContractEvidence
	findings     []AttributedFinding
	partial      []Partiality
}

// NewEntryEvidence validates the evidence of one entry. Every finding
// must be tied to the entry by a basis its structure shows, and a
// contract that did not decide, or code that was not read, makes the
// evidence partial whatever the caller says.
func NewEntryEvidence(s EntryEvidenceSpec) (EntryEvidence, error) {
	if s.Entry.Name == "" || s.Entry.Context == "" || !evidenceConcept(s.Entry.Concept) {
		return EntryEvidence{}, fmt.Errorf("entry evidence: the entry needs a concept, a context and a name")
	}
	if err := validDeclarations(s); err != nil {
		return EntryEvidence{}, err
	}
	e := EntryEvidence{
		entry:        s.Entry,
		recording:    s.Recording,
		recordedLine: s.RecordedLine,
		declarations: s.Declarations,
		locations:    slices.Clone(s.Locations),
		contracts:    slices.Clone(s.Contracts),
		partial:      slices.Clone(s.Partial),
	}
	for _, f := range s.Findings {
		if len(f.Bases) == 0 {
			return EntryEvidence{}, fmt.Errorf("entry evidence %s: finding %q names no basis", s.Entry.Name, f.Violation.Message())
		}
		for _, b := range f.Bases {
			if !e.ties(b, f.Violation) {
				return EntryEvidence{}, fmt.Errorf("entry evidence %s: %s does not tie finding %q to the entry", s.Entry.Name, b, f.Violation.Message())
			}
		}
		e.findings = append(e.findings, AttributedFinding{Violation: f.Violation, Bases: slices.Clone(f.Bases)})
	}
	if err := e.ensurePartialNeverReadsClean(); err != nil {
		return EntryEvidence{}, err
	}
	return e, nil
}

func validDeclarations(s EntryEvidenceSpec) error {
	n := len(s.Locations)
	switch s.Declarations {
	case DeclarationLocated:
		if n != 1 {
			return fmt.Errorf("entry evidence %s: a located declaration has one location, not %d", s.Entry.Name, n)
		}
	case DeclarationAmbiguous:
		if n < 2 {
			return fmt.Errorf("entry evidence %s: an ambiguous declaration has several locations, not %d", s.Entry.Name, n)
		}
	case DeclarationMissing, DeclarationNotObserved, DeclarationNotApplicable:
		if n != 0 {
			return fmt.Errorf("entry evidence %s: a %s declaration has no location", s.Entry.Name, s.Declarations)
		}
	default:
		return fmt.Errorf("entry evidence %s: declaration state %q invalid", s.Entry.Name, s.Declarations)
	}
	return nil
}

// ties reports whether the basis, read from the finding's structure,
// ties the finding to this entry.
func (e EntryEvidence) ties(b EvidenceBasis, v Violation) bool {
	switch b {
	case BasisContract:
		for _, c := range e.contracts {
			if c.Covered && coverageHolds(c.Coverage, v) {
				return true
			}
		}
	case BasisRecordedLine:
		return e.recordedLine > 0 && v.Path() == e.recording && v.Line() == e.recordedLine
	case BasisBoundedContext:
		subject, err := rule.ContextSubject(e.entry.Context)
		return err == nil && e.entry.Concept == vocab.ConceptBoundedContext && v.Subject().Equals(subject)
	case BasisDeclaringFile:
		for _, l := range e.locations {
			if v.Path() == l.Path {
				return true
			}
		}
	}
	return false
}

// coverageHolds reports whether one of the coverage's checks holds the
// finding, matched as Assessment.RelabelViolations matches it.
func coverageHolds(c ContractCoverage, v Violation) bool {
	for _, check := range c.Checks() {
		for _, held := range check.Violations() {
			if held.Fingerprint() == v.Fingerprint() && held.Path() == v.Path() && held.Line() == v.Line() {
				return true
			}
		}
	}
	return false
}

// ensurePartialNeverReadsClean adds the partiality the evidence itself
// shows: a contract whose checks did not decide, and a context whose
// code was not read. With it the evidence never reads as free of
// findings when something it needed was not decided.
func (e *EntryEvidence) ensurePartialNeverReadsClean() error {
	for _, c := range e.contracts {
		if !c.decided() && !e.hasPartial(PartialContractUndecided, c.Key) {
			e.partial = append(e.partial, Partiality{Kind: PartialContractUndecided, Contract: c.Key, Outcome: c.undecidedOutcome()})
		}
	}
	if e.declarations == DeclarationNotObserved && !e.hasPartial(PartialNotObserved, "") {
		e.partial = append(e.partial, Partiality{Kind: PartialNotObserved})
	}
	for _, p := range e.partial {
		switch p.Kind {
		case PartialAssessmentIncomplete, PartialParseFailed, PartialContractUndecided, PartialCheckUndecided, PartialNotObserved:
		default:
			return fmt.Errorf("entry evidence %s: partial kind %q invalid", e.entry.Name, p.Kind)
		}
	}
	return nil
}

func (e EntryEvidence) hasPartial(kind PartialKind, contract string) bool {
	for _, p := range e.partial {
		if p.Kind == kind && p.Contract == contract {
			return true
		}
	}
	return false
}

// Entry names the entry the evidence is about.
func (e EntryEvidence) Entry() EntryAddress { return e.entry }

// RecordedAt is where the domain recording writes the entry; false
// when the line is unknown.
func (e EntryEvidence) RecordedAt() (Carrier, bool) {
	if e.recordedLine <= 0 || e.recording == "" {
		return Carrier{}, false
	}
	return Carrier{Path: e.recording, Line: e.recordedLine}, true
}

// Declarations is how the entry's code was found.
func (e EntryEvidence) Declarations() DeclarationState { return e.declarations }

// Locations are the declarations spelling the entry.
func (e EntryEvidence) Locations() []Carrier { return slices.Clone(e.locations) }

// Contracts are the entry's recorded invariants and assertions, or the
// contract the entry is.
func (e EntryEvidence) Contracts() []ContractEvidence { return slices.Clone(e.contracts) }

// Findings are the findings attributed to the entry, in the
// assessment's reporting order.
func (e EntryEvidence) Findings() []AttributedFinding { return slices.Clone(e.findings) }

// Partial lists why the assessment could not decide everything about
// the entry.
func (e EntryEvidence) Partial() []Partiality { return slices.Clone(e.partial) }

// ActiveFindings counts the attributed findings that report and gate.
func (e EntryEvidence) ActiveFindings() int {
	n := 0
	for _, f := range e.findings {
		if f.Violation.Status() == StatusActive {
			n++
		}
	}
	return n
}

// Reading is how the evidence reads as a whole: findings when an
// active finding is attributed, partial when none is but something was
// not decided, and no attributed findings only otherwise.
func (e EntryEvidence) Reading() EvidenceReading {
	switch {
	case e.ActiveFindings() > 0:
		return ReadingFindings
	case len(e.partial) > 0:
		return ReadingPartial
	default:
		return ReadingNoAttributedFindings
	}
}

// evidenceConcept reports whether the concept is one an assessment can
// show evidence about: every recorded concept a declaration, a contract
// or a recorded line can tie a finding to. A question has no code.
func evidenceConcept(c vocab.Concept) bool {
	switch c {
	case vocab.ConceptBoundedContext, vocab.ConceptAggregate, vocab.ConceptEntity, vocab.ConceptValueObject,
		vocab.ConceptInvariant, vocab.ConceptAssertion, vocab.ConceptDomainEvent, vocab.ConceptDomainService,
		vocab.ConceptSpecification:
		return true
	case vocab.ConceptAggregateRoot, vocab.ConceptBusinessRule, vocab.ConceptQuestion:
		// A root is shown as its aggregate and a business rule as its
		// invariant or assertion; a question records no code.
		return false
	}
	return false
}
