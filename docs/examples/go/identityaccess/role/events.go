package role

import (
	"time"

	"example.com/saasovation/identityaccess/tenant"
	"example.com/saasovation/identityaccess/user"
)

// RoleProvisioned records that a tenant gained a role.
type RoleProvisioned struct {
	TenantID   tenant.TenantID
	Name       RoleName
	occurredOn time.Time
}

// OccurredOn is the instant the role was provisioned.
func (e RoleProvisioned) OccurredOn() time.Time { return e.occurredOn }

// UserAssignedToRole records that a user now plays a role; the access
// control checks of every other context care.
type UserAssignedToRole struct {
	TenantID   tenant.TenantID
	Name       RoleName
	Username   user.Username
	occurredOn time.Time
}

// OccurredOn is the instant the user was assigned.
func (e UserAssignedToRole) OccurredOn() time.Time { return e.occurredOn }
