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
			Rule:    "Every must-always/must-never statement -> invariant (or assertion); assign exactly one owner. Always-true -> invariants[]; named operation -> assertions[] with id and on. Value-object owner, no id; aggregate owner of a named cluster contract requires id.",
			Example: "total = sum of lines -> owner Order root; every tier priced before Publish -> assertion on Publish",
		},
		{
			ID:      "specification-as-thing",
			Rule:    "Experts pass the predicate around as a thing -> specifications[], a type with SatisfiedBy; never a flag on a value object and never an invariant.",
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
			ID:      "homeless-operation",
			Rule:    "Logic spanning aggregates or needing external domain knowledge -> domain_service.",
			Example: "DiscountCalculator over Order+Customer",
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
