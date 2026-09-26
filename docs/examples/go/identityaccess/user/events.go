package user

import (
	"time"

	"example.com/saasovation/identityaccess/tenant"
)

// UserRegistered records that a user account came into existence under a
// tenant; the tenant's administrators and the collaboration context care.
type UserRegistered struct {
	TenantID   tenant.TenantID
	Username   Username
	Name       FullName
	Email      EmailAddress
	occurredOn time.Time
}

// OccurredOn is the instant the user was registered.
func (e UserRegistered) OccurredOn() time.Time { return e.occurredOn }

// UserPasswordChanged records that a user chose a new password.
type UserPasswordChanged struct {
	TenantID   tenant.TenantID
	Username   Username
	occurredOn time.Time
}

// OccurredOn is the instant the password changed.
func (e UserPasswordChanged) OccurredOn() time.Time { return e.occurredOn }

// UserEnablementChanged records that when a user may sign in changed.
type UserEnablementChanged struct {
	TenantID   tenant.TenantID
	Username   Username
	Enablement Enablement
	occurredOn time.Time
}

// OccurredOn is the instant the enablement changed.
func (e UserEnablementChanged) OccurredOn() time.Time { return e.occurredOn }
