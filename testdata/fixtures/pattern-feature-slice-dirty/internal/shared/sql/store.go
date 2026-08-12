package sql

import "example.com/library/internal/member"

// Store maps rows to domain types; deciding whether a member may borrow is
// not its business.
type Store struct{}

func (Store) Find(id string) (member.Member, error) { return member.Member{}, nil }
