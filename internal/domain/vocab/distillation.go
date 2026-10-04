package vocab

// DistillationRule is one domain-librarian distillation rule from
// VOCAB.yaml: stable id, rule text, and example.
type DistillationRule struct {
	ID      string
	Rule    string
	Example string
}

// DistillationRules returns the rules in VOCAB.yaml order with
// char-exact id/rule/example strings.
func DistillationRules() []DistillationRule {
	return []DistillationRule{
		{
			ID:      "identity-test",
			Rule:    "Concept keeps its business identity while attributes change -> entity.",
			Example: "order keeps its number when status changes",
		},
		{
			ID:      "value-test",
			Rule:    "Two instances with identical values are interchangeable -> value_object.",
			Example: "any two USD 10 amounts are the same money",
		},
		{
			ID:      "invariant-ownership",
			Rule:    "First establish a domain consistency rule and its capable owner; must-always/must-never wording alone does not establish one. Reviewer guidance, contributor workflow, host guarantees, delivery validation and programming guards are not automatically invariants or assertions. A justified rule holding at all observable times -> the owner's invariants, keyed; a justified guarantee of a named domain operation -> the owner's assertions, keyed, with on: the aggregate's for its command, the domain service's for its operation. A value object's key names value integrity checked at construction; an aggregate's key names the root method that enforces the cluster rule.",
			Example: "total = sum of lines -> Order.invariants.total-is-sum-of-lines; every tier priced before Publish -> Order.assertions.tiers-priced on Publish",
		},
		{
			ID:      "specification-as-thing",
			Rule:    "Experts pass the predicate around as a thing -> specifications, a type with SatisfiedBy; never a flag on a value object and never an invariant.",
			Example: "preferred customer is handed to pricing as a spec, not inlined as a must-always",
		},
		{
			ID:      "language-not-guards",
			Rule:    "Record only what you would say to an expert who never saw the language; programming-only guards are not domain contracts.",
			Example: "nil receiver check is not an invariant; a Price is never negative is",
		},
		{
			ID:      "transaction-boundary",
			Rule:    "Smallest cluster consistent in one transaction -> one aggregate; everything else by ID, eventually.",
			Example: "reject: customer and all orders save together",
		},
		{
			ID:      "thing-first",
			Rule:    "A behavior goes to the entity or value that naturally owns it before any service is considered; a service that could be a method on one root is a wrong home.",
			Example: "Tenant.Deactivate, not TenantService.Deactivate(tenant)",
		},
		{
			ID:      "command-vs-query",
			Rule:    "Changes the state of one aggregate and returns no domain information -> a command of that root; changes nothing and answers a question -> a query of the root or value; never both in one operation. Neither is recorded.",
			Example: "Tenant.Deactivate is a command; Tenant.IsRegistrationAvailableThrough is a query",
		},
		{
			ID:      "domain-service-when-no-owner",
			Rule:    "Does it make a business decision that no one aggregate can make alone? Yes -> domain_service, recorded under services with its contract; it may read through repositories, never commits, never publishes.",
			Example: "TenantProvisioningService over Tenant, User, and Role; Encrypter for knowledge no aggregate holds",
		},
		{
			ID:      "application-service-holds-no-rule",
			Rule:    "If I delete every step here, is any business decision lost? No -> application_service: loading, invoking, committing, publishing, notifying, translating; not recorded. Yes -> the decision is placed under its owner and reported as a finding.",
			Example: "the ProvisionTenant use case runs the service in one unit of work and publishes; it holds no rule",
		},
		{
			ID:      "contract-of-a-service",
			Rule:    "A guarantee of one operation of a domain service -> the service's assertions, keyed, with on naming the operation; the set is the service's contract.",
			Example: "TenantProvisioningService.assertions.tenant-name-unique on ProvisionTenant",
		},
		{
			ID:      "precondition-as-assertion",
			Rule:    "A rule that holds only when one named command runs, not at every instant -> the owner's assertions with on that command; the statement stays as the expert said it and is never rewritten into a post-condition.",
			Example: "Tenant.assertions.invites-only-while-active on OfferRegistrationInvitation, not a Tenant invariant, because a deactivated tenant is legal",
		},
		{
			ID:      "not-a-domain-rule",
			Rule:    "A promise about delivery (notify, export, retry, transaction control) or a programming guard -> no entry; the application service or the code does it.",
			Example: "notify billing on TenantProvisioned is a subscriber, not an assertion",
		},
		{
			ID:      "intention-revealing-name",
			Rule:    "Name a service and its operations for effect and purpose in the ubiquitous language, never for the means; a name ending in Manager, Helper, Handler, or Processor is a finding to rename, not a kind. An application service's input is named for the request, never with the word command.",
			Example: "TenantProvisioningService.ProvisionTenant, not TenantManager.Handle; ProvisionTenantRequest, not ProvisionTenantCommand",
		},
		{
			ID:      "event-detection",
			Rule:    "State change experts say in past tense -> domain_event; technical changes are not events.",
			Example: "OrderConfirmed yes, RowUpdated no",
		},
		{
			ID:      "context-split",
			Rule:    "Same word, materially different meaning per team -> separate bounded_contexts + relation.",
			Example: "Product: price in Catalog, weight in Shipping",
		},
		{
			ID:      "language-fidelity",
			Rule:    "Record terms exactly as experts say them; reject developer jargon.",
			Example: "policy renewal, not ContractUpdateManager",
		},
		{
			ID:      "synonym-collapse",
			Rule:    "One meaning, one canonical term per context; synonyms become aliases.",
			Example: "client/customer/account -> customer",
		},
		{
			ID:      "repository-gate",
			Rule:    "Repositories only for aggregate roots; inner entity wanting one = wrong boundary.",
			Example: "reject OrderLineRepository; reject \"we must list comps, so Comp is a root\"",
		},
		{
			ID:      "minimal-evidence",
			Rule:    "Classify on the fewest deciding facts; two plausible kinds -> ask, never guess.",
			Example: "'we track shipments' alone decides nothing -> ask",
		},
	}
}

// DistillationRuleByID returns the rule with the given id, or false.
func DistillationRuleByID(id string) (DistillationRule, bool) {
	for _, r := range DistillationRules() {
		if r.ID == id {
			return r, true
		}
	}
	return DistillationRule{}, false
}
