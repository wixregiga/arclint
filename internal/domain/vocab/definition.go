package vocab

// Definition is one recorded project term: its canonical name and
// optional meaning and aliases. Aggregate designation is not a field
// of Definition; only Entity carries it. Line is where the term is
// written down in the recorded Ubiquitous Language file, so a finding
// about the term points at the term; it is 0 for a term that is not
// written down yet.
type Definition struct {
	Name       string
	Definition string
	Aliases    []string
	Line       int
}

// Counts tallies contexts, terms, and relations. Aggregates counts
// designated Entities across every context. Invariants counts recorded
// invariants (including those entered via business_rule that resolved
// to an invariant). Assertions and Specifications count their own
// sections.
type Counts struct {
	Contexts       int
	Entities       int
	Aggregates     int
	ValueObjects   int
	Invariants     int
	Assertions     int
	Specifications int
	Events         int
	Relations      int
}
