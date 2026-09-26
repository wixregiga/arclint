// Package tenant is the home of the Tenant aggregate of the identityaccess
// context: the subscribing organisation under which users, groups, and
// roles exist.
package tenant

import (
	"errors"
	"fmt"
	"time"

	"example.com/saasovation/identityaccess/event"
)

// Tenant is the aggregate root. It owns its registration invitations and
// enforces that at most one available invitation exists per description.
type Tenant struct {
	event.Recorder

	tenantID    TenantID
	name        TenantName
	description string
	active      bool
	invitations []RegistrationInvitation
	clock       func() time.Time
}

// ErrTenantInactive is returned when a command requires an active tenant.
var ErrTenantInactive = errors.New("tenant is not active")

// ErrTenantAlreadyActive is returned when an active tenant is activated again.
var ErrTenantAlreadyActive = errors.New("tenant is already active")

// ErrTenantAlreadyInactive is returned when an inactive tenant is deactivated again.
var ErrTenantAlreadyInactive = errors.New("tenant is already inactive")

// ErrInvitationAlreadyOffered is returned when an available invitation with
// the same description already exists.
var ErrInvitationAlreadyOffered = errors.New("an available invitation with that description already exists")

// ErrInvitationUnknown is returned when no invitation answers to an identifier.
var ErrInvitationUnknown = errors.New("no registration invitation answers to that identifier")

// Provision is the constructor door: a tenant comes into existence active,
// because its administrator is registered under it in the same process.
func Provision(id TenantID, name TenantName, description string, now func() time.Time) (*Tenant, error) {
	if id.IsZero() {
		return nil, errors.New("tenant requires an assigned identity")
	}
	t := &Tenant{tenantID: id, name: name, description: description, active: true, clock: now}
	if err := t.EnsureOneOpenInvitationPerDescription(); err != nil {
		return nil, err
	}
	t.Raise(TenantProvisioned{TenantID: id, Name: name, occurredOn: now()})
	return t, nil
}

// Reconstitute rebuilds a tenant from its persisted state; a repository
// adapter is its only caller.
func Reconstitute(id TenantID, name TenantName, description string, active bool, invitations []RegistrationInvitation, now func() time.Time) (*Tenant, error) {
	t := &Tenant{tenantID: id, name: name, description: description, active: active, invitations: invitations, clock: now}
	if err := t.EnsureOneOpenInvitationPerDescription(); err != nil {
		return nil, err
	}
	return t, nil
}

// TenantID is the aggregate's identity.
func (t *Tenant) TenantID() TenantID { return t.tenantID }

// Name is the name the tenant registered under.
func (t *Tenant) Name() TenantName { return t.name }

// Description is the free text the tenant was described with.
func (t *Tenant) Description() string { return t.description }

// IsActive reports whether the tenant may be used.
func (t *Tenant) IsActive() bool { return t.active }

// Invitations lists the tenant's registration invitations, available or not.
func (t *Tenant) Invitations() []RegistrationInvitation {
	out := make([]RegistrationInvitation, len(t.invitations))
	copy(out, t.invitations)
	return out
}

// Activate puts the tenant back into use.
func (t *Tenant) Activate() error {
	if t.active {
		return ErrTenantAlreadyActive
	}
	t.active = true
	if err := t.EnsureOneOpenInvitationPerDescription(); err != nil {
		return err
	}
	t.Raise(TenantActivated{TenantID: t.tenantID, occurredOn: t.clock()})
	return nil
}

// ANCHOR: command-ensure

// Deactivate takes the tenant out of use; every user of the tenant loses
// access until it is activated again.
func (t *Tenant) Deactivate() error {
	if !t.active {
		return ErrTenantAlreadyInactive
	}
	t.active = false
	if err := t.EnsureOneOpenInvitationPerDescription(); err != nil {
		return err
	}
	t.Raise(TenantDeactivated{TenantID: t.tenantID, occurredOn: t.clock()})
	return nil
}

// ANCHOR_END: command-ensure

// ANCHOR: command-assert

// OfferRegistrationInvitation opens a new open-ended invitation under the
// given description and returns it.
func (t *Tenant) OfferRegistrationInvitation(description string) (RegistrationInvitation, error) {
	if err := t.AssertInvitesOnlyWhileActive(); err != nil {
		return RegistrationInvitation{}, err
	}
	invitation, err := newRegistrationInvitation(description)
	if err != nil {
		return RegistrationInvitation{}, err
	}
	if t.IsRegistrationAvailableThrough(invitation.Description()) {
		return RegistrationInvitation{}, ErrInvitationAlreadyOffered
	}
	t.invitations = append(t.invitations, invitation)
	if err := t.EnsureOneOpenInvitationPerDescription(); err != nil {
		return RegistrationInvitation{}, err
	}
	return invitation, nil
}

// ANCHOR_END: command-assert

// LimitInvitation closes the window of an offered invitation to the given
// period.
func (t *Tenant) LimitInvitation(identifier string, startingOn, until time.Time) error {
	if err := t.AssertInvitesOnlyWhileActive(); err != nil {
		return err
	}
	index := t.invitationIndex(identifier)
	if index < 0 {
		return ErrInvitationUnknown
	}
	limited, err := t.invitations[index].withWindow(startingOn, until)
	if err != nil {
		return err
	}
	t.invitations[index] = limited
	return t.EnsureOneOpenInvitationPerDescription()
}

// WithdrawInvitation removes an invitation so that nobody registers through it.
func (t *Tenant) WithdrawInvitation(identifier string) error {
	index := t.invitationIndex(identifier)
	if index < 0 {
		return ErrInvitationUnknown
	}
	t.invitations = append(t.invitations[:index], t.invitations[index+1:]...)
	return t.EnsureOneOpenInvitationPerDescription()
}

// ANCHOR: query

// IsRegistrationAvailableThrough is a side-effect-free function: it
// answers whether a user may register through the identified invitation
// right now.
func (t *Tenant) IsRegistrationAvailableThrough(identifier string) bool {
	index := t.invitationIndex(identifier)
	return index >= 0 && t.invitations[index].IsAvailable(t.clock())
}

// ANCHOR_END: query

// ANCHOR: ensure

// EnsureOneOpenInvitationPerDescription enforces the aggregate invariant
// one-open-invitation-per-description: a tenant holds at most one
// available registration invitation per description.
func (t *Tenant) EnsureOneOpenInvitationPerDescription() error {
	now := t.clock()
	seen := make(map[string]struct{}, len(t.invitations))
	for _, invitation := range t.invitations {
		if !invitation.IsAvailable(now) {
			continue
		}
		if _, dup := seen[invitation.Description()]; dup {
			return fmt.Errorf("tenant %s: %w: %q", t.tenantID, ErrInvitationAlreadyOffered, invitation.Description())
		}
		seen[invitation.Description()] = struct{}{}
	}
	return nil
}

// ANCHOR_END: ensure

// ANCHOR: assert

// AssertInvitesOnlyWhileActive checks the assertion invites-only-while-active
// on OfferRegistrationInvitation and LimitInvitation: an invitation is
// offered or limited only by an active tenant.
func (t *Tenant) AssertInvitesOnlyWhileActive() error {
	if !t.active {
		return fmt.Errorf("tenant %s: %w", t.tenantID, ErrTenantInactive)
	}
	return nil
}

// ANCHOR_END: assert

func (t *Tenant) invitationIndex(identifier string) int {
	for i, invitation := range t.invitations {
		if invitation.IsIdentifiedBy(identifier) {
			return i
		}
	}
	return -1
}
