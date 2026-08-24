package vocab

import "errors"

// ErrDefinitionNotFound reports that a named definition is absent from
// the project domain model. Delivery maps it to exit 1.
var ErrDefinitionNotFound = errors.New("definition not found in the project domain model")

// Change is a partial update applied by Define. Omitted fields leave
// the existing value unchanged. SetDefinition with an empty
// DefinitionText clears the definition. SetAliases replaces the
// complete alias set. ClearAliases clears aliases and is mutually
// exclusive with SetAliases. SetAggregate designates or clears the
// Aggregate flag on an Entity (guided authoring). SetOwner sets the
// enforcing owner on an Invariant (required when creating an
// invariant, assertion, or business_rule).
type Change struct {
	SetDefinition  bool
	DefinitionText string
	SetAliases     bool
	Aliases        []string
	ClearAliases   bool
	SetAggregate   bool
	Aggregate      bool
	SetOwner       bool
	Owner          string
}

// Outcome is the result classification of a Define call.
type Outcome string

// The defined Define outcomes.
const (
	OutcomeCreated   Outcome = "created"
	OutcomeUpdated   Outcome = "updated"
	OutcomeUnchanged Outcome = "unchanged"
)

// DefineResult reports what Define did to one definition.
type DefineResult struct {
	Outcome Outcome
	// Changed lists which fields actually differed, in the fixed order
	// definition, aliases, aggregate for entities; definition, aliases
	// for value objects and events; owner for invariants.
	Changed []string
}

// RemoveResult reports what Remove did. EntityPreserved is true when
// an Aggregate designation was cleared and the Entity remains.
type RemoveResult struct {
	EntityPreserved bool
}
