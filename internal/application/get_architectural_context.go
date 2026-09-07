package application

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// ContextRequest selects the scope: no paths and no zones means the
// repository; Paths are repo-relative files or folders (a folder
// matches through the Zone path globs), and Zones name declared
// Zones directly. Full keeps the whole recorded domain in a worksite
// answer instead of the part that anchors into the scope.
type ContextRequest struct {
	Paths []string
	Zones []string
	Full  bool
}

// PathBinding maps one requested path to the declared Zones owning
// it.
type PathBinding struct {
	Path  string
	Zones []string
}

// KindInUse pairs one Rule Type appearing in the configuration with
// its one-line meaning.
type KindInUse struct {
	Kind    string
	Meaning string
}

// ZonePolicy is the plain view of one declared Zone and its
// dependency policy, derived from the consumes Rule bound to it.
type ZonePolicy struct {
	Name        string
	Description string
	Paths       []string
	// Internal is the allow-list of other Zones; nil when
	// unrestricted, empty when the Zone may import no other Zone.
	Internal           []string
	InternalRestricted bool
	External           string // allow or forbid
	Stdlib             string // allow or forbid
}

// AppliedRule pairs one Rule with the reason it applies to the scope.
type AppliedRule struct {
	Summary RuleSummary
	Reason  string
	// Via lists the scope parts that pulled the Rule in; empty when
	// the scope has a single part.
	Via []string
}

// ArchitecturalContext is the human- and agent-readable view of the
// Rules, Zones, and applicability reasons for one scope: the same
// facts for both audiences, distinguishing intended Rules from
// observed code.
type ArchitecturalContext struct {
	// Scope is "repository" or a repo-relative path.
	Scope     string
	Languages []string
	RuleCount int
	// Zones holds every declared Zone for repository scope, the
	// involved Zones for a worksite scope.
	Zones []ZonePolicy
	// Rules holds, for a worksite scope, each Rule that applies and
	// why, deduplicated across the scope parts.
	Rules []AppliedRule
	// Paths maps each requested path to its owning Zones; empty for
	// repository scope.
	Paths []PathBinding
	// Kinds lists the Rule Types the configuration uses with their
	// meanings; repository scope only.
	Kinds []KindInUse
	// UnknownImports is the effective scan policy for unclassifiable
	// imports; repository scope only.
	UnknownImports string
	// Domain is the project's recorded domain model summary; nil when
	// the project records none. Repository scope and Full carry the
	// whole model; a worksite carries the part anchored into it.
	Domain *DomainKnowledge `json:"domain,omitempty"`
}

// DomainAggregateRef is one aggregate inside a bounded-context
// summary: its root's name, the identity it is known by, and its
// member entities.
type DomainAggregateRef struct {
	Name     string   `json:"name"`
	Identity string   `json:"identity,omitempty"`
	Entities []string `json:"entities,omitempty"`
}

// DomainInvariantRef is one invariant with its owner and the outcome
// of looking for it in source: an aggregate's invariant is carried by
// the root method its key names, a value object's by the constructor.
type DomainInvariantRef struct {
	Key       string `json:"key"`
	Statement string `json:"statement"`
	Owner     string `json:"owner"`
	// OwnerConcept is aggregate or value_object.
	OwnerConcept vocab.Concept `json:"ownerConcept"`
	// Source is file:line of the carrying declaration when Anchor is
	// found; empty otherwise.
	Source string         `json:"source,omitempty"`
	Anchor ContractAnchor `json:"anchor,omitempty"`
}

// DomainAssertionRef is one assertion with the operation it is on and
// the outcome of looking for the root method its key names.
type DomainAssertionRef struct {
	Key       string         `json:"key"`
	Statement string         `json:"statement"`
	Owner     string         `json:"owner"`
	On        string         `json:"on"`
	Source    string         `json:"source,omitempty"`
	Anchor    ContractAnchor `json:"anchor,omitempty"`
}

// DomainSpecificationRef is one specification name with its source
// location.
type DomainSpecificationRef struct {
	Name   string         `json:"name"`
	Source string         `json:"source,omitempty"`
	Anchor ContractAnchor `json:"anchor,omitempty"`
}

// DomainRelationRef is one context-map edge.
type DomainRelationRef struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

// DomainContextKnowledge is one bounded context projected into
// architectural context: canonical names only, with every recorded
// contract.
type DomainContextKnowledge struct {
	Name           string                   `json:"name"`
	Aggregates     []DomainAggregateRef     `json:"aggregates,omitempty"`
	ValueObjects   []string                 `json:"valueObjects,omitempty"`
	Invariants     []DomainInvariantRef     `json:"invariants,omitempty"`
	Assertions     []DomainAssertionRef     `json:"assertions,omitempty"`
	Specifications []DomainSpecificationRef `json:"specifications,omitempty"`
	Events         []string                 `json:"events,omitempty"`
	Services       []string                 `json:"services,omitempty"`
}

// UnanchoredContract is one listed contract that no observed
// declaration carries, kept apart from the per-context listing so a
// reader cannot skim past it.
type UnanchoredContract struct {
	Kind    ContractKind `json:"kind"`
	Context string       `json:"context"`
	Owner   string       `json:"owner,omitempty"`
	Key     string       `json:"key,omitempty"`
	// Statement carries the invariant or assertion statement; Name the
	// specification name.
	Statement string `json:"statement,omitempty"`
	Name      string `json:"name,omitempty"`
	// Expected names the declaration the recording says carries the
	// contract, the one no observed file declares.
	Expected string `json:"expected"`
}

// DomainKnowledge is the project's recorded domain model summary as
// projected into architectural context. Counts always tally the whole
// recorded model; when Scoped, the listing was narrowed to what
// anchors into the worksite and Shown tallies the listing.
type DomainKnowledge struct {
	Source string       `json:"source"`
	Counts vocab.Counts `json:"counts"`
	Scoped bool         `json:"scoped,omitempty"`
	Shown  vocab.Counts `json:"shown"`
	// Located is true when observed declarations were used to anchor
	// the contracts; false leaves every Anchor empty.
	Located   bool                     `json:"located"`
	Contexts  []DomainContextKnowledge `json:"contexts,omitempty"`
	Relations []DomainRelationRef      `json:"relations,omitempty"`
	// Unanchored lists every listed contract whose Anchor is missing.
	Unanchored []UnanchoredContract `json:"unanchored,omitempty"`
}

// GetArchitecturalContext projects Rules, Zones, and applicability
// reasons for a selected scope.
type GetArchitecturalContext struct {
	rules        rule.Repository
	knowledge    vocab.Repository
	observations ObservationSource
}

// NewGetArchitecturalContext requires the Rule and domain-model
// repository ports.
func NewGetArchitecturalContext(rules rule.Repository, knowledge vocab.Repository) (GetArchitecturalContext, error) {
	if rules == nil {
		return GetArchitecturalContext{}, fmt.Errorf("architectural context: missing rule repository")
	}
	if knowledge == nil {
		return GetArchitecturalContext{}, fmt.Errorf("architectural context: missing domain model repository")
	}
	return GetArchitecturalContext{rules: rules, knowledge: knowledge}, nil
}

// WithObservations lends the observation source used to locate
// recorded contracts and terms in source.
func (uc GetArchitecturalContext) WithObservations(observations ObservationSource) GetArchitecturalContext {
	uc.observations = observations
	return uc
}

// Execute projects the context for one scope: an empty request means
// the repository (every Zone, the Rule kinds in use, and the
// enforcement posture); paths and named Zones select a worksite (its
// per-path bindings, the involved Zones, the deduplicated Rules
// that govern it, and the recorded domain anchored into it).
func (uc GetArchitecturalContext) Execute(req ContextRequest) (ArchitecturalContext, error) {
	cfg, err := uc.rules.ConfiguredRules()
	if err != nil {
		return ArchitecturalContext{}, fmt.Errorf("load configured rules: %w", err)
	}
	out := ArchitecturalContext{Scope: "repository", RuleCount: len(cfg.Rules)}
	for _, l := range cfg.Languages {
		out.Languages = append(out.Languages, string(l))
	}
	carriers, err := uc.recordedDomain(&out, cfg)
	if err != nil {
		return ArchitecturalContext{}, err
	}
	if len(req.Paths) == 0 && len(req.Zones) == 0 {
		for _, m := range cfg.Zones {
			out.Zones = append(out.Zones, zonePolicy(m, cfg.Rules))
		}
		out.Kinds = kindsInUse(cfg.Rules)
		policy := cfg.Scan.UnknownImports
		if policy == "" {
			policy = rule.UnknownImportsWarn
		}
		out.UnknownImports = string(policy)
		if out.Domain != nil {
			out.Domain.Unanchored = unanchoredContracts(out.Domain, cfg.Languages)
		}
		return out, nil
	}
	out, err = worksite(out, cfg, req)
	if err != nil {
		return ArchitecturalContext{}, err
	}
	if out.Domain != nil {
		if !req.Full {
			out.Domain = scopeDomainKnowledge(out.Domain, carriers, worksiteScope(cfg, req))
		}
		out.Domain.Unanchored = unanchoredContracts(out.Domain, cfg.Languages)
	}
	return out, nil
}

// recordedDomain projects the recorded language into the context and,
// when an observation source is lent, locates its contracts in source.
// Without a recorded domain the context carries none and no carriers
// are read.
func (uc GetArchitecturalContext) recordedDomain(out *ArchitecturalContext, cfg rule.Configured) (conformance.Carriers, error) {
	lang, found, err := uc.knowledge.RecordedLanguage()
	if err != nil {
		return conformance.Carriers{}, fmt.Errorf("load domain model: %w", err)
	}
	if !found {
		return conformance.Carriers{}, nil
	}
	out.Domain = domainKnowledgeOf(lang)
	if uc.observations == nil {
		return conformance.Carriers{}, nil
	}
	obs, err := uc.observations.Observe(cfg.Languages, cfg.Scan, []rule.Fact{rule.FactDeclarations})
	if err != nil {
		return conformance.Carriers{}, fmt.Errorf("observe contracts: %w", err)
	}
	carriers, err := conformance.NewCarriers(obs, lang, cfg.Zones)
	if err != nil {
		return conformance.Carriers{}, fmt.Errorf("locate contracts: %w", err)
	}
	if err := locateDomainContracts(out.Domain, carriers); err != nil {
		return conformance.Carriers{}, fmt.Errorf("locate contracts: %w", err)
	}
	return carriers, nil
}

// worksite assembles the scoped view: per-path Zone bindings, each
// involved Zone once, and the union of governing Rules with the
// scope parts that pulled each in.
func worksite(out ArchitecturalContext, cfg rule.Configured, req ContextRequest) (ArchitecturalContext, error) {
	declared := map[rule.ZoneName]rule.Zone{}
	for _, m := range cfg.Zones {
		declared[m.Name()] = m
	}
	seenCard := map[rule.ZoneName]bool{}
	addCard := func(name rule.ZoneName) {
		if !seenCard[name] {
			seenCard[name] = true
			out.Zones = append(out.Zones, zonePolicy(declared[name], cfg.Rules))
		}
	}
	ruleIndex := map[string]int{}
	addRule := func(r rule.Rule, reason, via string) {
		id := r.ID().Qualified()
		if i, ok := ruleIndex[id]; ok {
			out.Rules[i].Via = appendUnique(out.Rules[i].Via, via)
			return
		}
		ruleIndex[id] = len(out.Rules)
		out.Rules = append(out.Rules, AppliedRule{Summary: summarize(r), Reason: reason, Via: []string{via}})
	}

	var scopeParts []string
	for _, p := range req.Paths {
		binding := PathBinding{Path: p}
		var owning []rule.ZoneName
		for _, m := range cfg.Zones {
			if m.Contains(p) {
				owning = append(owning, m.Name())
				binding.Zones = append(binding.Zones, string(m.Name()))
				addCard(m.Name())
			}
		}
		out.Paths = append(out.Paths, binding)
		scopeParts = append(scopeParts, p)
		for _, r := range cfg.Rules {
			if reason, applies := appliesToScope(r, p, owning); applies {
				addRule(r, reason, p)
			}
		}
	}
	for _, name := range req.Zones {
		mn := rule.ZoneName(name)
		if _, ok := declared[mn]; !ok {
			return ArchitecturalContext{}, fmt.Errorf(
				"zone %q is not declared; declared zones: %s", name, declaredNames(cfg.Zones))
		}
		addCard(mn)
		part := "zone " + name
		scopeParts = append(scopeParts, part)
		for _, r := range cfg.Rules {
			if reason, applies := appliesToZone(r, mn); applies {
				addRule(r, reason, part)
			}
		}
	}
	out.Scope = strings.Join(scopeParts, ", ")
	if len(scopeParts) == 1 {
		for i := range out.Rules {
			out.Rules[i].Via = nil
		}
	}
	return out, nil
}

// kindsInUse lists the distinct Rule Types of the configuration in
// published enum order, each with its meaning.
func kindsInUse(rules []rule.Rule) []KindInUse {
	seen := map[rule.Type]bool{}
	for _, r := range rules {
		seen[r.Type()] = true
	}
	var out []KindInUse
	for _, t := range rule.Types() {
		if seen[t] {
			out = append(out, KindInUse{Kind: string(t), Meaning: t.Meaning()})
		}
	}
	return out
}

func appendUnique(list []string, v string) []string {
	for _, e := range list {
		if e == v {
			return list
		}
	}
	return append(list, v)
}

func declaredNames(zones []rule.Zone) string {
	names := make([]string, 0, len(zones))
	for _, m := range zones {
		names = append(names, string(m.Name()))
	}
	return strings.Join(names, ", ")
}

// zonePolicy derives one Zone's dependency policy from the
// consumes Rule bound to it, when any.
func zonePolicy(m rule.Zone, rules []rule.Rule) ZonePolicy {
	p := ZonePolicy{
		Name:        string(m.Name()),
		Description: m.Description(),
		External:    string(rule.ImportAllow),
		Stdlib:      string(rule.ImportAllow),
	}
	for _, g := range m.Paths() {
		p.Paths = append(p.Paths, g.String())
	}
	for _, r := range rules {
		params, ok := r.Params().(rule.ConsumesParams)
		if !ok || !r.Applicability().WouldSelectZone(m.Name()) {
			continue
		}
		if params.Internal != nil {
			p.InternalRestricted = true
			p.Internal = []string{}
			for _, name := range params.Internal.Zones() {
				p.Internal = append(p.Internal, string(name))
			}
		}
		if params.External.Forbids() {
			p.External = string(rule.ImportForbid)
		}
		if params.Stdlib.Forbids() {
			p.Stdlib = string(rule.ImportForbid)
		}
		break
	}
	return p
}

// appliesToScope decides whether one Rule binds a path and states the
// reason in domain language.
func appliesToScope(r rule.Rule, path string, owning []rule.ZoneName) (string, bool) {
	switch params := r.Params().(type) {
	case rule.ConsumesParams, rule.StructureParams, rule.NamingParams, rule.ContentParams, rule.ExtensionParams:
		_ = params
		if r.AppliesToFile(path, owning) {
			shared := sharedZones(r, owning)
			if len(shared) == 0 {
				// Repository-scoped content and extension Rules select
				// every file without any Zone in common.
				return "selects the file repository-wide", true
			}
			return fmt.Sprintf("selects the file through Zone(s) %s", joinNames(shared)), true
		}
		if r.Applicability().ExcludedFile(path) && r.Applicability().WouldSelectFile(path, owning) {
			return "excluded from this Rule's Applicability", true
		}
	case rule.LayersParams:
		for _, name := range params.Layers {
			if nameIn(owning, name) {
				return fmt.Sprintf("Zone %q is layered by this Rule", name), true
			}
		}
	case rule.ProtectedParams:
		if nameIn(owning, params.Zone) {
			return fmt.Sprintf("Zone %q is protected by this Rule", params.Zone), true
		}
	case rule.AcyclicParams:
		scope := params.Zones
		if len(scope) == 0 {
			if len(owning) > 0 {
				return "every declared Zone is in the acyclic scope", true
			}
			return "", false
		}
		for _, name := range scope {
			if nameIn(owning, name) {
				return fmt.Sprintf("Zone %q is in the acyclic scope", name), true
			}
		}
	}
	return "", false
}

// appliesToZone decides whether one Rule binds a declared Zone
// named directly in the scope.
func appliesToZone(r rule.Rule, name rule.ZoneName) (string, bool) {
	switch params := r.Params().(type) {
	case rule.ConsumesParams, rule.StructureParams, rule.NamingParams, rule.ContentParams, rule.ExtensionParams:
		_ = params
		if r.Applicability().WouldSelectZone(name) {
			return fmt.Sprintf("selects Zone %q", name), true
		}
	case rule.LayersParams:
		if nameIn(params.Layers, name) {
			return fmt.Sprintf("Zone %q is layered by this Rule", name), true
		}
	case rule.ProtectedParams:
		if params.Zone == name {
			return fmt.Sprintf("Zone %q is protected by this Rule", params.Zone), true
		}
	case rule.AcyclicParams:
		if len(params.Zones) == 0 {
			return "every declared Zone is in the acyclic scope", true
		}
		if nameIn(params.Zones, name) {
			return fmt.Sprintf("Zone %q is in the acyclic scope", name), true
		}
	}
	return "", false
}

func sharedZones(r rule.Rule, owning []rule.ZoneName) []rule.ZoneName {
	var out []rule.ZoneName
	for _, m := range r.Applicability().Zones() {
		if nameIn(owning, m) {
			out = append(out, m)
		}
	}
	return out
}

func nameIn(list []rule.ZoneName, name rule.ZoneName) bool {
	for _, v := range list {
		if v == name {
			return true
		}
	}
	return false
}

func joinNames(names []rule.ZoneName) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += string(n)
	}
	return out
}

// domainKnowledgeOf projects a recorded Ubiquitous Language into the
// context summary: per-context aggregates with their members, value
// objects, every contract with its owner, events, services, and the
// context map.
func domainKnowledgeOf(lang vocab.UbiquitousLanguage) *DomainKnowledge {
	dk := &DomainKnowledge{
		Source: vocab.UbiquitousLanguageFileName,
		Counts: lang.Counts(),
		Shown:  lang.Counts(),
	}
	for _, ctx := range lang.Contexts {
		summary := DomainContextKnowledge{Name: ctx.Name}
		for _, a := range ctx.Aggregates {
			ref := DomainAggregateRef{Name: a.Name, Identity: a.Identity}
			for _, e := range a.Entities {
				ref.Entities = append(ref.Entities, e.Name)
			}
			summary.Aggregates = append(summary.Aggregates, ref)
		}
		for _, v := range ctx.ValueObjects {
			summary.ValueObjects = append(summary.ValueObjects, v.Name)
		}
		for _, oi := range ctx.Invariants() {
			summary.Invariants = append(summary.Invariants, DomainInvariantRef{
				Key:          oi.Invariant.Key,
				Statement:    oi.Invariant.Statement,
				Owner:        oi.Owner,
				OwnerConcept: oi.OwnerConcept,
			})
		}
		for _, oa := range ctx.Assertions() {
			summary.Assertions = append(summary.Assertions, DomainAssertionRef{
				Key:       oa.Assertion.Key,
				Statement: oa.Assertion.Statement,
				Owner:     oa.Owner,
				On:        oa.Assertion.On,
			})
		}
		for _, s := range ctx.Specifications {
			summary.Specifications = append(summary.Specifications, DomainSpecificationRef{Name: s.Name})
		}
		for _, e := range ctx.Events {
			summary.Events = append(summary.Events, e.Name)
		}
		for _, s := range ctx.Services {
			summary.Services = append(summary.Services, s.Name)
		}
		dk.Contexts = append(dk.Contexts, summary)
	}
	for _, rel := range lang.Relations {
		dk.Relations = append(dk.Relations, DomainRelationRef{
			From: rel.From,
			To:   rel.To,
			Kind: string(rel.Kind),
		})
	}
	return dk
}

// unanchoredContracts collects every listed contract whose Anchor is
// missing, in listing order, each naming the declaration the recording
// expects, spelled the way the project's languages spell a method.
// Empty when the contracts were never located.
func unanchoredContracts(dk *DomainKnowledge, languages []rule.Language) []UnanchoredContract {
	if !dk.Located {
		return nil
	}
	var out []UnanchoredContract
	for _, ctx := range dk.Contexts {
		for _, inv := range ctx.Invariants {
			if inv.Anchor != AnchorMissing {
				continue
			}
			out = append(out, UnanchoredContract{
				Kind: ContractInvariant, Context: ctx.Name, Owner: inv.Owner, Key: inv.Key,
				Statement: inv.Statement, Expected: expectedInvariantCarrier(inv, languages),
			})
		}
		for _, a := range ctx.Assertions {
			if a.Anchor != AnchorMissing {
				continue
			}
			out = append(out, UnanchoredContract{
				Kind: ContractAssertion, Context: ctx.Name, Owner: a.Owner, Key: a.Key,
				Statement: a.Statement, Expected: expectedMethod(conformance.AssertKey(a.Key), a.Owner, languages),
			})
		}
		for _, s := range ctx.Specifications {
			if s.Anchor != AnchorMissing {
				continue
			}
			out = append(out, UnanchoredContract{
				Kind: ContractSpecification, Context: ctx.Name, Name: s.Name,
				Expected: fmt.Sprintf("satisfaction method on %s", s.Name),
			})
		}
	}
	return out
}

// expectedInvariantCarrier names the declaration an invariant's
// recording says carries it.
func expectedInvariantCarrier(inv DomainInvariantRef, languages []rule.Language) string {
	if inv.OwnerConcept == vocab.ConceptValueObject {
		return fmt.Sprintf("constructor of %s", inv.Owner)
	}
	return expectedMethod(conformance.EnsureKey(inv.Key), inv.Owner, languages)
}

// expectedMethod names the method a kebab-case key expects on its
// owner, spelled per configured language and labelled with the
// language when several are configured; a project with no language
// configured is told the key.
func expectedMethod(key, owner string, languages []rule.Language) string {
	var spellings []string
	for _, lang := range languages {
		name, err := conformance.MethodName(key, lang)
		if err != nil {
			continue
		}
		if len(languages) == 1 {
			spellings = append(spellings, name)
			continue
		}
		spellings = append(spellings, fmt.Sprintf("%s (%s)", name, lang))
	}
	if len(spellings) == 0 {
		return fmt.Sprintf("method %s on %s", key, owner)
	}
	return fmt.Sprintf("method %s on %s", strings.Join(spellings, " or "), owner)
}
