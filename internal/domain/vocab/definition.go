package vocab

// Definition is one recorded project term: its canonical name and
// optional meaning and aliases. Aggregate designation is not a field
// of Definition; only Entity carries it.
type Definition struct {
	Name       string
	Definition string
	Aliases    []string
}

// Counts tallies each concept group. Aggregates counts designated
// Entities.
type Counts struct {
	Entities      int
	Aggregates    int
	ValueObjects  int
	BusinessRules int
	Events        int
}
