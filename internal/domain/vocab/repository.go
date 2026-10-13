package vocab

// Repository is the domain-owned persistence port for the project's
// recorded Ubiquitous Language vocabulary. Implementations live in
// infrastructure; this port is owned by the domain so use cases depend
// on the UbiquitousLanguage value, never on a file format.
//
//nolint:iface // Domain-owned vocabulary port is consumed and implemented across package boundaries.
type Repository interface {
	// Recording is where this repository writes the language down,
	// whether or not the file exists yet.
	Recording() DomainRecording
	// RecordedLanguage returns the vocabulary and whether the file
	// exists. A language that is read carries the DomainRecording it was read
	// from; a missing file is an empty language carrying the DomainRecording
	// it would be written to. A file that cannot become a valid
	// UbiquitousLanguage is an error, never a partial value.
	RecordedLanguage() (UbiquitousLanguage, bool, error)
	// Record persists the complete recorded Ubiquitous Language,
	// preserving human authoring (comments, ordering) of untouched
	// entries and writing atomically.
	Record(UbiquitousLanguage) error
}
