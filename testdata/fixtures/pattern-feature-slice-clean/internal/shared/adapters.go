package shared

import (
	"database/sql"

	"example.com/library/internal/copy"
	"example.com/library/internal/loan"
	"example.com/library/internal/member"
)

// Adapters exposes every technology adapter the same way; app is the only
// caller.
type Adapters struct {
	Members member.Repo
	Loans   loan.Repo
	Copies  copy.Repo
}

func NewAdapters() Adapters {
	var db *sql.DB
	_ = db
	return Adapters{}
}
