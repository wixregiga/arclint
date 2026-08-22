package vocab

// Entity is a Definition plus the Aggregate designation. Only Entities
// can carry that designation: Aggregate is not a separate stored object
// and is never valid on Value Objects, Business Rules, or Events.
type Entity struct {
	Definition
	Aggregate bool
}
