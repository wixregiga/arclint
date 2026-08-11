package entities

// UserRedis persists users on the redis substrate. No counterpart exists
// under internal/setup/ — the correspondence contract flags this file.
type UserRedis struct {
	Addr string
}
