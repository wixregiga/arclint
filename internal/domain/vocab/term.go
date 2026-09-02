package vocab

// Term is one of the 24 domain-librarian vocabulary entries from
// VOCAB.yaml (context_relation is exposed via RelationKindDocs, not as
// a single string definition).
type Term string

// Vocabulary term keys in VOCAB.yaml order.
const (
	TermDomain             Term = "domain"
	TermModel              Term = "model"
	TermUbiquitousLanguage Term = "ubiquitous_language"
	TermContext            Term = "context"
	TermBoundedContext     Term = "bounded_context"
	TermContextMap         Term = "context_map"
	TermCoreDomain         Term = "core_domain"
	TermGenericSubdomain   Term = "generic_subdomain"
	TermBigBallOfMud       Term = "big_ball_of_mud"
	TermContextRelation    Term = "context_relation"
	TermEntity             Term = "entity"
	TermValueObject        Term = "value_object"
	TermInvariant          Term = "invariant"
	TermAssertion          Term = "assertion"
	TermSpecification      Term = "specification"
	TermAggregate          Term = "aggregate"
	TermAggregateRoot      Term = "aggregate_root"
	TermRepository         Term = "repository"
	TermFactory            Term = "factory"
	TermDomainService      Term = "domain_service"
	TermApplicationService Term = "application_service"
	TermDomainEvent        Term = "domain_event"
	TermModule             Term = "module"
	TermBusinessRule       Term = "business_rule"
)

// TermDef is one vocabulary entry: key plus exact VOCAB.yaml one-liner.
// TermContextRelation has Definition empty; its body is the eight
// RelationKind meanings (see RelationKindDocs / ContextRelationFlowYAML).
type TermDef struct {
	Term       Term
	Definition string
}

// VocabularyTerms returns the 24 terms in VOCAB.yaml order with
// char-exact one-line definitions.
func VocabularyTerms() []TermDef {
	return []TermDef{
		{Term: TermDomain, Definition: "Sphere of knowledge/activity the software addresses."},
		{Term: TermModel, Definition: "System of abstractions describing selected domain aspects."},
		{Term: TermUbiquitousLanguage, Definition: "Language structured around the model."},
		{Term: TermContext, Definition: "The setting that determines a statement's meaning."},
		{Term: TermBoundedContext, Definition: "Explicit boundary within which one model applies."},
		{Term: TermContextMap, Definition: "Inventory of all bounded_contexts and their relations."},
		{Term: TermCoreDomain, Definition: "The business-differentiating heart of the model; keep small."},
		{Term: TermGenericSubdomain, Definition: "Supporting area with no differentiation; factor out."},
		{Term: TermBigBallOfMud, Definition: "Boundary-less mixed-model region; wall it off."},
		{Term: TermContextRelation}, // body is RelationKindDocs / ContextRelationFlowYAML
		{Term: TermEntity, Definition: "Object defined by identity that persists across attribute change."},
		{Term: TermValueObject, Definition: "Immutable, identity-less object describing a characteristic."},
		{Term: TermInvariant, Definition: "Consistency rule that must hold at all times within its owner's boundary."},
		{Term: TermAssertion, Definition: "Post-condition of a named operation, not a truth that holds at all times."},
		{Term: TermSpecification, Definition: "Named predicate experts pass around as a thing: a type with a satisfaction method, never an invariant."},
		{Term: TermAggregate, Definition: "Cluster of entities + value_objects changed as one unit."},
		{Term: TermAggregateRoot, Definition: "The single entry-point entity of an aggregate."},
		{Term: TermRepository, Definition: "Collection-like abstraction over storage/retrieval of aggregate roots."},
		{Term: TermFactory, Definition: "Encapsulates complex creation."},
		{Term: TermDomainService, Definition: "Stateless domain operation spanning aggregates."},
		{Term: TermApplicationService, Definition: "Use-case orchestration + transaction boundary; no domain rules."},
		{Term: TermDomainEvent, Definition: "Record of something that happened, past-tense expert language."},
		{Term: TermModule, Definition: "Cohesive package telling the story of one model area."},
		{Term: TermBusinessRule, Definition: "Umbrella term; always resolves to invariant or assertion, never specification, owned by entity/aggregate root (behavior) or value_object (construction)."},
	}
}

// TermDefinition returns the one-line definition for term, or "" for
// context_relation (use ContextRelationFlowYAML) and unknown keys.
func TermDefinition(term Term) string {
	for _, t := range VocabularyTerms() {
		if t.Term == term {
			return t.Definition
		}
	}
	return ""
}
