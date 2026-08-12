package loan

import "example.com/library/internal/member"

// Loan links a member to a borrowed copy.
type Loan struct {
	Member member.Member
	CopyID string
}
