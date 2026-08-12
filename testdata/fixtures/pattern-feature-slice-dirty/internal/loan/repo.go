package loan

// Repo is the port for loan persistence.
type Repo interface {
	OpenCount(memberID string) (int, error)
}
