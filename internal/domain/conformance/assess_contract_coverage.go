package conformance

import (
	"fmt"
	"sort"

	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

type recordedContract struct {
	spec         ContractCoverageSpec
	ownerConcept vocab.Concept
	invariant    vocab.Invariant
	assertion    vocab.Assertion
}

func recordedContracts(knowledge vocab.UbiquitousLanguage) []recordedContract {
	var out []recordedContract
	for _, ctx := range knowledge.Contexts {
		for _, inv := range ctx.Invariants() {
			out = append(out, recordedContract{
				ownerConcept: inv.OwnerConcept, invariant: inv.Invariant,
				spec: ContractCoverageSpec{
					Context: ctx.Name, Owner: inv.Owner, Kind: vocab.ConceptInvariant,
					Key: inv.Invariant.Key, Statement: inv.Invariant.Statement, Path: knowledge.Recording.Path(), Line: inv.Invariant.Line,
				},
			})
		}
		for _, as := range ctx.Assertions() {
			out = append(out, recordedContract{
				ownerConcept: as.OwnerConcept, assertion: as.Assertion,
				spec: ContractCoverageSpec{
					Context: ctx.Name, Owner: as.Owner, Kind: vocab.ConceptAssertion,
					Key: as.Assertion.Key, Statement: as.Assertion.Statement, Path: knowledge.Recording.Path(), Line: as.Assertion.Line,
				},
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].spec, out[j].spec
		if a.Context != b.Context {
			return a.Context < b.Context
		}
		if a.Owner != b.Owner {
			return a.Owner < b.Owner
		}
		return a.Key < b.Key
	})
	return out
}

// The built-in Rules that check recorded invariants and assertions.
const (
	ruleAssertionChecked        = "assertion/checked-by-its-operation"
	ruleValueObjectConstructor  = "value_object/constructed-through-one-door"
	ruleInvariantsEnforcedRoot  = "aggregate/invariants-enforced-by-root"
	ruleInvariantEnforcedMutate = "invariant/enforced-at-every-mutation"
)

// ContractRuleIDs names every built-in Rule whose evaluation decides
// the coverage of a recorded invariant or assertion; a check limited
// to them still covers every recorded contract.
func ContractRuleIDs() []string {
	return []string{ruleAssertionChecked, ruleValueObjectConstructor, ruleInvariantsEnforcedRoot, ruleInvariantEnforcedMutate}
}

func (c recordedContract) ruleIDs() []string {
	if c.spec.Kind == vocab.ConceptAssertion {
		return []string{ruleAssertionChecked}
	}
	if c.ownerConcept == vocab.ConceptValueObject {
		return []string{ruleValueObjectConstructor}
	}
	return []string{ruleInvariantsEnforcedRoot, ruleInvariantEnforcedMutate}
}

// isolate retains the located code but passes only this contract to the
// same evaluator used by the Rule. Coverage never borrows another owner's
// finding or infers an individual result from a context-wide pass.
func (c recordedContract) isolate(cc contextCode) contextCode {
	narrow := vocab.BoundedContext{Name: cc.ctx.Name, Definition: cc.ctx.Definition, Line: cc.ctx.Line}
	switch c.ownerConcept {
	case vocab.ConceptAggregate:
		a, _ := cc.ctx.Aggregate(c.spec.Owner)
		a.Invariants, a.Assertions = nil, nil
		if c.spec.Kind == vocab.ConceptAssertion {
			a.Assertions = []vocab.Assertion{c.assertion}
		} else {
			a.Invariants = []vocab.Invariant{c.invariant}
		}
		narrow.Aggregates = []vocab.Aggregate{a}
	case vocab.ConceptValueObject:
		v, _ := cc.ctx.ValueObject(c.spec.Owner)
		v.Invariants = []vocab.Invariant{c.invariant}
		narrow.ValueObjects = []vocab.ValueObject{v}
	case vocab.ConceptDomainService:
		s, _ := cc.ctx.Service(c.spec.Owner)
		s.Assertions = []vocab.Assertion{c.assertion}
		narrow.Services = []vocab.DomainService{s}
	default:
		// recordedContracts only supplies the three legal contract owners.
	}
	cc.ctx = narrow
	return cc
}

// carrier locates the declaration carrying the contract in the code a
// check of it read, exactly as a listing of the language does; nil
// when nothing there carries it.
func (c recordedContract) carrier(code domainCode) (*Carrier, error) {
	carriers := carriersOf(code)
	var (
		at    Carrier
		found bool
		err   error
	)
	switch {
	case c.spec.Kind == vocab.ConceptAssertion:
		at, found, err = carriers.Assertion(c.spec.Context, c.spec.Owner, c.spec.Key)
	case c.ownerConcept == vocab.ConceptValueObject:
		at, found = carriers.Constructor(c.spec.Context, c.spec.Owner)
	default:
		at, found, err = carriers.Invariant(c.spec.Context, c.spec.Owner, c.spec.Key)
	}
	if err != nil {
		return nil, fmt.Errorf("contract coverage: %s %s of %s: %w", c.spec.Kind, c.spec.Key, c.spec.Owner, err)
	}
	if !found {
		return nil, nil
	}
	return &at, nil
}

func (c recordedContract) ownerLocated(cc contextCode) bool {
	if c.ownerConcept == vocab.ConceptAggregate {
		return cc.aggregates[c.spec.Owner].located()
	}
	return cc.terms[c.spec.Owner].located()
}

func assessContractCoverage(req Request, prepared map[string]Facts) ([]ContractCoverage, error) {
	rules := map[string]rule.Rule{}
	for _, r := range req.Rules {
		rules[r.ID().Qualified()] = r
	}
	// Resolve each Rule's scoped observations once, independently of how
	// many invariants and assertions it checks.
	codeByRule := map[string]domainCode{}
	var out []ContractCoverage
	var err error
	for _, contract := range recordedContracts(req.Knowledge) {
		spec := contract.spec
		for _, id := range contract.ruleIDs() {
			r, selected := rules[id]
			if !selected {
				spec.Unperformed = append(spec.Unperformed, id+": not selected")
				continue
			}
			if d, disabled := r.Disablement(); disabled {
				spec.Unperformed = append(spec.Unperformed, id+": disabled: "+d.Reason())
				continue
			}
			code, cached := codeByRule[id]
			if !cached {
				code, err = resolveDomain(prepared[id], req.Knowledge)
				if err != nil {
					return nil, err
				}
				codeByRule[id] = code
			}
			if spec.Carrier == nil {
				if spec.Carrier, err = contract.carrier(code); err != nil {
					return nil, err
				}
			}
			for _, cc := range code.contexts {
				if cc.ctx.Name != spec.Context {
					continue
				}
				e, err := evaluateRecordedContract(r, contract, cc, code)
				if err != nil {
					return nil, err
				}
				spec.Checks = append(spec.Checks, e)
			}
		}
		coverage, err := NewContractCoverage(spec)
		if err != nil {
			return nil, err
		}
		out = append(out, coverage)
	}
	return out, nil
}

func evaluateRecordedContract(r rule.Rule, contract recordedContract, cc contextCode, code domainCode) (Evaluation, error) {
	subject, err := rule.ContextSubject(cc.ctx.Name)
	if err != nil {
		return Evaluation{}, fmt.Errorf("contract coverage: %w", err)
	}
	inv, err := r.Params().(rule.DomainParams).BlockInvariant()
	if err != nil {
		return Evaluation{}, fmt.Errorf("contract coverage: %w", err)
	}
	if r.Scope().ExcludedFile(cc.recording) {
		return simpleEvaluation(r, subject, OutcomeNotApplicable)
	}
	if cc.scopeParseFailures > 0 {
		return simpleEvaluation(r, subject, OutcomeFailed)
	}
	if outcome, unavailable := cc.undecidable(inv.Enforcement.Facts); unavailable {
		return simpleEvaluation(r, subject, outcome)
	}
	// Partial analysis cannot establish coverage for an individual contract.
	for _, fact := range inv.Enforcement.Facts {
		if available := cc.scopeIdx.available(rule.Fact(fact)); fact != string(rule.FactFileTree) && available < cc.scopeAnalysisFiles {
			return undeterminedEvaluation(r, subject, fmt.Sprintf("%d of %d analyzed files in %s yield %s facts",
				available, cc.scopeAnalysisFiles, cc.searched(), fact))
		}
	}
	if !contract.ownerLocated(cc) {
		return undeterminedEvaluation(r, subject, fmt.Sprintf("no single declaration of %s is located in %s",
			contract.spec.Owner, cc.searched()))
	}
	check, exists := domainChecks[r.ID().Local()]
	if !exists {
		return Evaluation{}, fmt.Errorf("contract coverage: no check for %s", r.ID())
	}
	violations, err := check.run(r, subject, contract.isolate(cc), code)
	if err != nil {
		return Evaluation{}, fmt.Errorf("contract coverage: %w", err)
	}
	return completeEvaluation(r, subject, violations)
}

// undeterminedEvaluation leaves a contract undecided with the reason
// the check could not decide it.
func undeterminedEvaluation(r rule.Rule, subject rule.Subject, reason string) (Evaluation, error) {
	e, err := simpleEvaluation(r, subject, OutcomeUndetermined)
	if err != nil {
		return Evaluation{}, err
	}
	return e.withUnresolved([]string{reason})
}
