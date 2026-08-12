package loan

// MaxOpenLoans is the domain invariant: it lives here, never in shared.
const MaxOpenLoans = 5

func WithinLimit(open int) bool { return open < MaxOpenLoans }
