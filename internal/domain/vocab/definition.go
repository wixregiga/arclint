package vocab

// Definition is one recorded project term: its canonical name and
// optional meaning and aliases. Aggregate designation is not a field
// of Definition; only Entity carries it.
type Definition struct {
	Name       string
	Definition string
	Aliases    []string
}

// Counts tallies contexts, terms, and relations. Aggregates counts
// designated Entities across every context. Invariants counts every
// recorded invariant (including those entered via business_rule or
// assertion).
type Counts struct {
	Contexts     int
	Entities     int
	Aggregates   int
	ValueObjects int
	Invariants   int
	Events       int
	Relations    int
}
