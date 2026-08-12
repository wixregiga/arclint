package copy

// Repo is the port for copy persistence.
type Repo interface {
	Available(id string) (bool, error)
}
