package member

// Repo is the technology-agnostic port for member persistence.
type Repo interface {
	Find(id string) (Member, error)
}
