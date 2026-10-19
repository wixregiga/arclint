package application

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// DefineDomainRequest records one entry of the Ubiquitous Language:
// the concept spelled as vocab.ParseConcept accepts it, the bounded
// context (empty when the project records exactly one), the owner for
// a nested concept (the aggregate of an entity, the aggregate or value
// object of an invariant, the aggregate of an assertion; empty when
// the name already finds the entry), the name or key, and the change
// to apply. The change carries a Set flag per property so that an
// omitted property stays as recorded and an empty one is recorded
// deliberately.
type DefineDomainRequest struct {
	Concept string
	Context string
	Owner   string
	Name    string
	Change  vocab.Change
}

// DomainDefineResult reports one define: what was done, the concept
// the entry was recorded as, where it sits, and the properties that
// changed, in the meta-model's order.
type DomainDefineResult struct {
	Outcome vocab.Outcome
	Concept vocab.Concept
	Context string
	Owner   string
	Name    string
	Changed []string
	// Target and Values describe edits to the context map or project description.
	Target string
	Values map[string]string
	// Check says where a recorded assertion is checked, chosen by how
	// its owner is declared in the code; nil for every other concept
	// and when no observations were lent.
	Check *conformance.AssertionHome
}

// DefineDomainDefinition records or updates one entry of the project's
// Ubiquitous Language through the repository port.
type DefineDomainDefinition struct {
	knowledge    vocab.Repository
	project      string
	rules        rule.Repository
	observations ObservationSource
}

// NewDefineDomainDefinition requires the Ubiquitous Language repository
// port and the project name a first recording is filed under when no
// domain file exists yet: the repository directory's name by default,
// the same default init records.
func NewDefineDomainDefinition(knowledge vocab.Repository, project string) (DefineDomainDefinition, error) {
	if knowledge == nil {
		return DefineDomainDefinition{}, fmt.Errorf("define domain definition: missing knowledge repository")
	}
	project = strings.TrimSpace(project)
	if project == "" {
		return DefineDomainDefinition{}, fmt.Errorf("define domain definition: missing project name")
	}
	return DefineDomainDefinition{knowledge: knowledge, project: project}, nil
}

// WithObservations lends rules and observations so that defining an
// assertion says where its guarantee is checked, chosen by how its
// owner is declared in the code.
func (uc DefineDomainDefinition) WithObservations(rules rule.Repository, observations ObservationSource) DefineDomainDefinition {
	uc.rules = rules
	uc.observations = observations
	return uc
}

// Execute records the entry. A request that cannot be carried out as
// spelled (no name, no context to record in, a property the concept
// does not record, a required property missing on first recording)
// wraps ErrDomainUsage; a change the recorded language refuses names
// the meta-model invariant it would break. A missing domain file is an
// empty language named for the project; the model is saved only when
// something changed.
func (uc DefineDomainDefinition) Execute(req DefineDomainRequest) (DomainDefineResult, error) {
	c, err := vocab.ParseConcept(strings.TrimSpace(req.Concept))
	if err != nil {
		return DomainDefineResult{}, fmt.Errorf("%w: %v", ErrDomainUsage, err)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return DomainDefineResult{}, fmt.Errorf("%w: %s: a name is required", ErrDomainUsage, c)
	}
	lang, found, err := uc.knowledge.RecordedLanguage()
	if err != nil {
		return DomainDefineResult{}, fmt.Errorf("load domain model: %w", err)
	}
	if !found {
		lang = vocab.UbiquitousLanguage{Project: uc.project}
	}

	at := vocab.Locator{Owner: strings.TrimSpace(req.Owner), Name: name}
	if c != vocab.ConceptBoundedContext {
		at.Context, err = resolveContext(lang, req.Context)
		if err != nil {
			return DomainDefineResult{}, err
		}
	}
	next, res, err := lang.Define(c, at, req.Change)
	if err != nil {
		return DomainDefineResult{}, domainRequestError(err)
	}
	out := DomainDefineResult{
		Outcome: res.Outcome,
		Concept: res.Concept,
		Context: at.Context,
		Owner:   res.Owner,
		Name:    name,
		Changed: res.Changed,
	}
	if res.Concept == vocab.ConceptBoundedContext {
		out.Context = name
	}
	if res.Concept == vocab.ConceptAssertion {
		// Located before the write, so a repository that cannot be
		// observed leaves the recording as it was.
		out.Check, err = uc.assertionHome(next, at.Context, res.Owner, name)
		if err != nil {
			return DomainDefineResult{}, err
		}
	}
	if res.Outcome == vocab.OutcomeUnchanged {
		return out, nil
	}
	if err := uc.knowledge.Record(next); err != nil {
		return DomainDefineResult{}, fmt.Errorf("save domain model: %w", err)
	}
	return out, nil
}

// assertionHome locates the owner of the assertion in the code and says
// where its guarantee is checked; nil without observations.
func (uc DefineDomainDefinition) assertionHome(lang vocab.UbiquitousLanguage, context, owner, key string) (*conformance.AssertionHome, error) {
	if uc.rules == nil || uc.observations == nil {
		return nil, nil
	}
	cfg, err := uc.rules.ConfiguredRules()
	if err != nil {
		return nil, fmt.Errorf("load configured rules: %w", err)
	}
	required := carrierFacts()
	obs, err := uc.observations.Observe(cfg.Languages, cfg.Scan, required, nil, conformance.NamedTestsToRead(cfg.Rules, lang))
	if err != nil {
		return nil, fmt.Errorf("observe the assertion's owner: %w", err)
	}
	facts, err := conformance.NewInspectionFacts(cfg.Zones, obs, required...)
	if err != nil {
		return nil, fmt.Errorf("prepare domain facts: %w", err)
	}
	carriers, err := conformance.NewCarriers(facts, lang)
	if err != nil {
		return nil, fmt.Errorf("locate the assertion's owner: %w", err)
	}
	home, found, err := carriers.AssertionHome(context, owner, key, cfg.Languages)
	if err != nil {
		return nil, fmt.Errorf("locate the assertion's owner: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("assertion %s of %s is not recorded in context %s after it was defined", key, owner, context)
	}
	return &home, nil
}
