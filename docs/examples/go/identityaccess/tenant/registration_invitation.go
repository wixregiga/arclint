package tenant

import (
	"crypto/rand"
	"errors"
	"strings"
	"time"
)

// RegistrationInvitation is a member entity of the Tenant aggregate: a
// tenant's standing offer under which a user may register. It is local to
// its tenant and is never referenced from outside the aggregate.
type RegistrationInvitation struct {
	invitationID string
	description  string
	startingOn   time.Time
	until        time.Time
}

// ErrInvitationDescriptionBlank is returned when an invitation has no description.
var ErrInvitationDescriptionBlank = errors.New("invitation description must not be blank")

// ErrInvitationWindowInverted is returned when an invitation ends before it starts.
var ErrInvitationWindowInverted = errors.New("invitation must not end before it starts")

func newRegistrationInvitation(description string) (RegistrationInvitation, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		return RegistrationInvitation{}, ErrInvitationDescriptionBlank
	}
	return RegistrationInvitation{invitationID: rand.Text(), description: description}, nil
}

// InvitationID is the invitation's identity, local to its tenant.
func (i RegistrationInvitation) InvitationID() string { return i.invitationID }

// Description is the text the invitation was offered under.
func (i RegistrationInvitation) Description() string { return i.description }

// IsAvailable reports whether the invitation is open at the given instant:
// an open-ended invitation always is; a windowed one only inside its window.
func (i RegistrationInvitation) IsAvailable(at time.Time) bool {
	if i.startingOn.IsZero() && i.until.IsZero() {
		return true
	}
	return !at.Before(i.startingOn) && !at.After(i.until)
}

// IsIdentifiedBy reports whether the invitation answers to the identifier,
// which is either its invitation id or its description.
func (i RegistrationInvitation) IsIdentifiedBy(identifier string) bool {
	return i.invitationID == identifier || i.description == identifier
}

func (i RegistrationInvitation) withWindow(startingOn, until time.Time) (RegistrationInvitation, error) {
	if until.Before(startingOn) {
		return RegistrationInvitation{}, ErrInvitationWindowInverted
	}
	i.startingOn = startingOn
	i.until = until
	return i, nil
}
