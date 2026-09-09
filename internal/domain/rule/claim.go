package rule

import (
	"errors"
	"strings"
)

// Claim preserves the legacy Rule display value for existing Go callers.
// Deprecated: use Rationale for authored reasons and Rule.Proposition for
// the computed architectural proposition.
type Claim struct {
	statement string
}

// NewClaim requires a non-empty, intention-revealing statement.
func NewClaim(statement string) (Claim, error) {
	statement = strings.TrimSpace(statement)
	if statement == "" {
		return Claim{}, errors.New("claim: empty statement")
	}
	return Claim{statement: statement}, nil
}

// Statement returns what the Rule requires.
func (c Claim) Statement() string { return c.statement }

// Describe explains what the Rule requires.
func (c Claim) Describe() string { return c.statement }

// IsZero reports an unconstructed Claim.
func (c Claim) IsZero() bool { return c.statement == "" }

func (c Claim) String() string { return c.statement }
