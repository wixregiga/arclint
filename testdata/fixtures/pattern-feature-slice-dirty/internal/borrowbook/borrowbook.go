package borrowbook

import (
	"example.com/library/internal/loan"
	"example.com/library/internal/member"
)

// UseCase orchestrates concepts and ports; nothing more.
type UseCase struct {
	members member.Repo
}

func New(members member.Repo) *UseCase { return &UseCase{members: members} }

func (u *UseCase) Handle(cmd Command) (loan.Loan, error) {
	return loan.Loan{}, nil
}
