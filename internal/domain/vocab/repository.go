package vocab

// Repository is the domain-owned persistence port for the project's
// recorded Ubiquitous Language vocabulary. Implementations live in
// infrastructure; this port is owned by the domain so use cases depend
// on the UbiquitousLanguage value, never on a file format.
type Repository interface {
	// RecordedLanguage returns the vocabulary and whether the file
	// exists. A file that cannot become a valid UbiquitousLanguage is
	// an error, never a partial value.
	RecordedLanguage() (UbiquitousLanguage, bool, error)
	// Record persists the complete recorded Ubiquitous Language,
	// preserving human authoring (comments, ordering) of untouched
	// entries and writing atomically.
	Record(UbiquitousLanguage) error
}
