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
					Key: inv.Invariant.Key, Statement: inv.Invariant.Statement, Line: inv.Invariant.Line,
				},
			})
		}
		for _, as := range ctx.Assertions() {
			out = append(out, recordedContract{
				ownerConcept: as.OwnerConcept, assertion: as.Assertion,
				spec: ContractCoverageSpec{
					Context: ctx.Name, Owner: as.Owner, Kind: vocab.ConceptAssertion,
					Key: as.Assertion.Key, Statement: as.Assertion.Statement, Line: as.Assertion.Line,
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

func (c recordedContract) ruleIDs() []string {
	if c.spec.Kind == vocab.ConceptAssertion {
		return []string{"assertion/checked-by-its-operation"}
	}
	if c.ownerConcept == vocab.ConceptValueObject {
		return []string{"value_object/constructed-through-one-door"}
	}
	return []string{"aggregate/invariants-enforced-by-root", "invariant/enforced-at-every-mutation"}
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
				var err error
				code, err = resolveDomain(prepared[id], req.Knowledge)
				if err != nil {
					return nil, err
				}
				codeByRule[id] = code
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
	if r.Scope().ExcludedFile(vocab.UbiquitousLanguageFileName) {
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
		if fact != string(rule.FactFileTree) && cc.scopeIdx.available(rule.Fact(fact)) < cc.scopeAnalysisFiles {
			return simpleEvaluation(r, subject, OutcomeUndetermined)
		}
	}
	if !contract.ownerLocated(cc) {
		return simpleEvaluation(r, subject, OutcomeUndetermined)
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
