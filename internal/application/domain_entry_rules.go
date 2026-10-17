package application

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// DomainRuleStatus is how the current check judges one recorded
// invariant or assertion, in the words a reader asks with.
type DomainRuleStatus string

const (
	// RuleConforms means every check that ran found the enforcing
	// method and its calls where the recording says they are.
	RuleConforms DomainRuleStatus = "conforms"
	// RuleFinding means a check found a violation; Findings carries it.
	RuleFinding DomainRuleStatus = "finding"
	// RuleNotObservable means ArcLint reads none of the context's code
	// in the languages it analyzes, so no check could look.
	RuleNotObservable DomainRuleStatus = "not_observable"
	// RuleUndetermined means a check looked but could not decide, or
	// the checks that ran conform while another did not run; Reasons
	// and NotChecked say which.
	RuleUndetermined DomainRuleStatus = "undetermined"
	// RuleNotChecked means no check ran: the Rules are disabled, not
	// configured, or exclude the recording, or show was given no check.
	RuleNotChecked DomainRuleStatus = "not_checked"
	// RuleCheckFailed means the context's code failed to parse.
	RuleCheckFailed DomainRuleStatus = "failed"
)

// DomainEntryRules is what a shown entry records about its invariants
// and assertions. An entity records none of its own; Aggregate names
// the aggregate that records them and ShowAggregate the command showing
// it. Record is the command recording a first one when the entry
// records none and can hold one.
type DomainEntryRules struct {
	Checks        []DomainRuleCheck
	Aggregate     string
	ShowAggregate string
	Record        string
	// Checked is true when the current check ran; CheckError says why
	// it did not when it was asked for and failed.
	Checked    bool
	CheckError string
}

// DomainRuleCheck is one recorded invariant or assertion with the
// declaration enforcing it and the current check's judgment of it.
type DomainRuleCheck struct {
	Kind         vocab.Concept
	Owner        string
	OwnerConcept vocab.Concept
	Key          string
	On           string
	Statement    string
	// Source is the file:line of the enforcing declaration when a
	// check located it; Expected names the declaration the recording
	// expects when a check read the code and did not find it. Both are
	// empty when no check read the code.
	Source   string
	Expected string
	Status   DomainRuleStatus
	Findings []DomainRuleFinding
	// Reasons says why the status is undetermined, not observable,
	// not checked, or failed.
	Reasons []string
	// NotChecked lists the checks that did not run, with their reasons,
	// next to a status the checks that ran decided.
	NotChecked []string
	// Implementations lists, for an assertion of a domain service
	// declared as an interface or an abstract class, each implementation
	// and whether its operation reaches the check in the model.
	Implementations []DomainImplementationCheck
}

// DomainImplementationCheck is one implementation of a declared domain
// service as the check of an assertion judged it.
type DomainImplementationCheck struct {
	Type      string
	Operation string
	// Source is the file:line of the implementation's operation.
	Source string
	// Evidence is exact, heuristic, or undetermined: how the language
	// established that the type implements the service.
	Evidence string
	// Must says what the operation does to reach the check.
	Must   string
	Status DomainRuleStatus
	// Reason says why the status is undetermined, or what the finding
	// is.
	Reason string
}

// DomainRuleFinding is one violation a check found for the rule.
type DomainRuleFinding struct {
	Rule    string
	Message string
	Path    string
	Line    int
	// Status is active, suppressed, or baselined.
	Status string
}

// implementationStatus reads the outcome of one implementation of a
// declared domain service as a rule status; every other outcome is
// undetermined.
var implementationStatus = map[conformance.Outcome]DomainRuleStatus{
	conformance.OutcomeConforms: RuleConforms,
	conformance.OutcomeViolates: RuleFinding,
}

// recordedRules lists the invariants and assertions a shown entry
// records, nil for a concept that records none.
func recordedRules(lang vocab.UbiquitousLanguage, view DomainEntryView) *DomainEntryRules {
	ctx, _ := lang.Context(view.Context)
	out := &DomainEntryRules{}
	switch view.Concept {
	case vocab.ConceptAggregate:
		for _, inv := range view.Aggregate.Invariants {
			out.Checks = append(out.Checks, invariantCheck(view.Name, vocab.ConceptAggregate, inv))
		}
		for _, as := range view.Aggregate.Assertions {
			out.Checks = append(out.Checks, assertionCheck(view.Name, vocab.ConceptAggregate, as))
		}
		if len(out.Checks) == 0 {
			out.Record = recordRuleCommand(lang, view, "invariant <key> --owner %s --statement \"<statement>\"",
				"assertion <key> --owner %s --on <operation> --statement \"<statement>\"")
		}
	case vocab.ConceptValueObject:
		for _, inv := range view.ValueObject.Invariants {
			out.Checks = append(out.Checks, invariantCheck(view.Name, vocab.ConceptValueObject, inv))
		}
		if len(out.Checks) == 0 {
			out.Record = recordRuleCommand(lang, view, "invariant <key> --owner %s --statement \"<statement>\"")
		}
	case vocab.ConceptDomainService:
		for _, as := range view.Service.Assertions {
			out.Checks = append(out.Checks, assertionCheck(view.Name, vocab.ConceptDomainService, as))
		}
		if len(out.Checks) == 0 {
			out.Record = recordRuleCommand(lang, view, "assertion <key> --owner %s --on <operation> --statement \"<statement>\"")
		}
	case vocab.ConceptInvariant:
		ownerConcept := vocab.ConceptAggregate
		if _, ok := ctx.ValueObject(view.Owner); ok {
			ownerConcept = vocab.ConceptValueObject
		}
		out.Checks = append(out.Checks, invariantCheck(view.Owner, ownerConcept, view.Invariant))
	case vocab.ConceptAssertion:
		ownerConcept := vocab.ConceptAggregate
		if _, ok := ctx.Service(view.Owner); ok {
			ownerConcept = vocab.ConceptDomainService
		}
		out.Checks = append(out.Checks, assertionCheck(view.Owner, ownerConcept, view.Assertion))
	case vocab.ConceptEntity:
		out.Aggregate = view.Owner
		out.ShowAggregate = entryCandidate{concept: vocab.ConceptAggregate, context: view.Context, name: view.Owner}.command(len(lang.Contexts))
	default:
		return nil
	}
	return out
}

func invariantCheck(owner string, ownerConcept vocab.Concept, inv vocab.Invariant) DomainRuleCheck {
	return DomainRuleCheck{
		Kind: vocab.ConceptInvariant, Owner: owner, OwnerConcept: ownerConcept,
		Key: inv.Key, Statement: inv.Statement, Status: RuleNotChecked,
		Reasons: []string{"no check was run"},
	}
}

func assertionCheck(owner string, ownerConcept vocab.Concept, as vocab.Assertion) DomainRuleCheck {
	return DomainRuleCheck{
		Kind: vocab.ConceptAssertion, Owner: owner, OwnerConcept: ownerConcept,
		Key: as.Key, On: as.On, Statement: as.Statement, Status: RuleNotChecked,
		Reasons: []string{"no check was run"},
	}
}

// recordRuleCommand is the define command recording a first rule under
// the entry, one per form the concept accepts, joined by "or".
func recordRuleCommand(lang vocab.UbiquitousLanguage, view DomainEntryView, forms ...string) string {
	commands := make([]string, 0, len(forms))
	for _, form := range forms {
		cmd := "arclint domain define " + fmt.Sprintf(form, shellWord(view.Name))
		if len(lang.Contexts) > 1 {
			cmd += " --context " + shellWord(view.Context)
		}
		commands = append(commands, cmd)
	}
	return strings.Join(commands, " or ")
}

// judge runs the Conformance Check limited to the Rules that check
// recorded invariants and assertions, with the committed Baseline
// applied as arclint check applies it, and records each rule's
// enforcing declaration and status. A check that cannot run leaves
// every rule not checked, with the reason.
func (uc ShowDomainDefinition) judge(context string, rules *DomainEntryRules) {
	if uc.rules == nil || uc.assess == nil {
		return
	}
	cfg, err := uc.rules.ConfiguredRules()
	if err != nil {
		rules.CheckError = fmt.Sprintf("load configured rules: %v", err)
		return
	}
	configured := map[string]bool{}
	for _, r := range cfg.Rules {
		configured[r.ID().Qualified()] = true
	}
	var only []string
	for _, id := range conformance.ContractRuleIDs() {
		if configured[id] {
			only = append(only, id)
		}
	}
	if len(only) == 0 {
		rules.CheckError = "no Rule that checks invariants or assertions is configured"
		return
	}
	assessment, err := uc.assess.Execute(AssessConformanceRequest{Only: only})
	if err != nil {
		rules.CheckError = err.Error()
		return
	}
	rules.Checked = true
	for i := range rules.Checks {
		check := &rules.Checks[i]
		coverage, found := coverageOf(assessment, context, *check)
		if !found {
			check.Reasons = []string{"the check reported nothing for this rule"}
			continue
		}
		check.Expected = expectedCarrier(*check, cfg.Languages, coverage.BodilessOwner())
		judgeRule(check, coverage, cfg)
	}
}

func coverageOf(a conformance.Assessment, context string, check DomainRuleCheck) (conformance.ContractCoverage, bool) {
	for _, c := range a.Contracts() {
		if c.Context() == context && c.Kind() == check.Kind && c.Owner() == check.Owner && c.Key() == check.Key {
			return c, true
		}
	}
	return conformance.ContractCoverage{}, false
}

// expectedCarrier names the declaration the recording says enforces
// the rule, spelled the way the project's languages spell a method.
func expectedCarrier(check DomainRuleCheck, languages []rule.Language, bodilessOwner string) string {
	if check.Kind == vocab.ConceptAssertion {
		return expectedAssertionCarrier(check.Key, check.Owner, check.OwnerConcept, bodilessOwner, languages)
	}
	return expectedInvariantCarrier(DomainInvariantRef{Key: check.Key, Owner: check.Owner, OwnerConcept: check.OwnerConcept}, languages)
}

// judgeRule reads one contract's coverage into the rule's status: the
// declaration a check located, the findings, and the reasons a check
// could not decide or did not run.
func judgeRule(check *DomainRuleCheck, c conformance.ContractCoverage, cfg rule.Configured) {
	if at, located := c.Carrier(); located {
		check.Source = fmt.Sprintf("%s:%d", at.Path, at.Line)
		check.Expected = ""
	}
	check.Reasons = nil
	check.NotChecked = c.Unperformed()
	switch c.Status() {
	case "structurally_checked":
		check.Status = RuleConforms
	case "incomplete":
		// The checks that ran conform, but one that did not run could
		// have found what they cannot see.
		check.Status = RuleUndetermined
	case string(conformance.OutcomeViolates), string(conformance.OutcomeSuspectedViolation):
		check.Status = RuleFinding
	case string(conformance.OutcomeUnsupported):
		check.Status = RuleNotObservable
	case string(conformance.OutcomeUndetermined):
		check.Status = RuleUndetermined
	case string(conformance.OutcomeFailed):
		check.Status = RuleCheckFailed
	default:
		check.Status = RuleNotChecked
	}
	switch check.Status {
	case RuleNotChecked:
		check.Reasons, check.NotChecked = check.NotChecked, nil
		check.Expected = ""
	case RuleNotObservable, RuleCheckFailed:
		// No check read the code, so nothing says the declaration is
		// missing.
		check.Expected = ""
	case RuleConforms, RuleFinding, RuleUndetermined:
		// A check read the code; Expected stands when nothing carried
		// the rule.
	}
	for _, impl := range c.Implementations() {
		status, judged := implementationStatus[impl.Outcome]
		if !judged {
			status = RuleUndetermined
		}
		check.Implementations = append(check.Implementations, DomainImplementationCheck{
			Type: impl.Type, Operation: impl.Operation, Source: fmt.Sprintf("%s:%d", impl.Path, impl.Line),
			Evidence: string(impl.Evidence), Must: impl.Must, Status: status, Reason: impl.Reason,
		})
	}
	for _, e := range c.Checks() {
		id := e.Rule().Qualified()
		switch e.Outcome() {
		case conformance.OutcomeViolates, conformance.OutcomeSuspectedViolation:
			for _, v := range e.Violations() {
				check.Findings = append(check.Findings, DomainRuleFinding{
					Rule: id, Message: v.Message(), Path: v.Path(), Line: v.Line(), Status: string(v.Status()),
				})
			}
		case conformance.OutcomeUnsupported:
			check.Reasons = append(check.Reasons, fmt.Sprintf("%s: none of context %s's code is in a language ArcLint analyzes here (%s)",
				id, c.Context(), languageNames(cfg.Languages)))
		case conformance.OutcomeUndetermined:
			for _, reason := range e.Unresolved() {
				check.Reasons = append(check.Reasons, id+": "+reason)
			}
		case conformance.OutcomeFailed:
			check.Reasons = append(check.Reasons, fmt.Sprintf("%s: files of context %s's code failed to parse; arclint check . names them", id, c.Context()))
		case conformance.OutcomeNotApplicable:
			check.Reasons = append(check.Reasons, id+": "+notApplicableReason(cfg, e.Rule(), c))
		case conformance.OutcomeConforms:
		}
	}
}

func languageNames(languages []rule.Language) string {
	if len(languages) == 0 {
		return "no language is configured"
	}
	names := make([]string, 0, len(languages))
	for _, l := range languages {
		names = append(names, string(l))
	}
	return strings.Join(names, ", ")
}

// notApplicableReason says which exclusion of the Rule removed the
// contract: the domain recording it is recorded in, or every analyzed
// file of its context's code.
func notApplicableReason(cfg rule.Configured, id rule.ID, c conformance.ContractCoverage) string {
	for _, r := range cfg.Rules {
		if r.ID().Equals(id) && r.Scope().ExcludedFile(c.Path()) {
			return fmt.Sprintf("the rule's scope excludes the domain recording %s", c.Path())
		}
	}
	return fmt.Sprintf("the rule's scope excludes every analyzed file of context %s's code", c.Context())
}
