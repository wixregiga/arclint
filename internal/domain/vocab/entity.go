package vocab

// Entity is a Definition plus the Aggregate designation. Only Entities
// can carry that designation: Aggregate is not a separate stored object
// and is never valid on Value Objects, Invariants, or Events.
// ConceptAggregate and ConceptAggregateRoot are designations on an
// Entity, never stored objects of their own.
type Entity struct {
	Definition
	Aggregate bool
}
