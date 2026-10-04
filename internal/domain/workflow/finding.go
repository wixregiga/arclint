package workflow

// Finding explains a departure using a quote from one named supplied passage.
type Finding struct {
	Evidence   string
	Quote      string
	Departure  string
	Correction string
}
