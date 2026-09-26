// Package role is the home of the Role aggregate of the identityaccess
// context: a named capability a tenant grants to some of its users.
package role

import (
	"errors"
	"fmt"
	"time"

	"example.com/saasovation/identityaccess/event"
	"example.com/saasovation/identityaccess/tenant"
	"example.com/saasovation/identityaccess/user"
)

// Role is the aggregate root. It lists the users assigned to it by
// identity; it never holds a User.
type Role struct {
	event.Recorder

	tenantID    tenant.TenantID
	name        RoleName
	description string
	members     []user.Username
	clock       func() time.Time
}

// ErrAssigneeOfAnotherTenant is returned when a user of another tenant is assigned.
var ErrAssigneeOfAnotherTenant = errors.New("the user belongs to another tenant")

// ErrMemberListedTwice is returned when a member appears twice in a role.
var ErrMemberListedTwice = errors.New("a role lists each member once")

// Provision is the constructor door: a tenant provisions a role by name.
func Provision(tenantID tenant.TenantID, name RoleName, description string, now func() time.Time) (*Role, error) {
	if tenantID.IsZero() {
		return nil, errors.New("role requires a tenant")
	}
	r := &Role{tenantID: tenantID, name: name, description: description, clock: now}
	if err := r.EnsureMembersListedOnce(); err != nil {
		return nil, err
	}
	r.Raise(RoleProvisioned{TenantID: tenantID, Name: name, occurredOn: now()})
	return r, nil
}

// Reconstitute rebuilds a role from persisted state; a repository adapter
// is its only caller.
func Reconstitute(tenantID tenant.TenantID, name RoleName, description string, members []user.Username, now func() time.Time) (*Role, error) {
	r := &Role{tenantID: tenantID, name: name, description: description, members: members, clock: now}
	if err := r.EnsureMembersListedOnce(); err != nil {
		return nil, err
	}
	return r, nil
}

// RoleID is the aggregate's identity.
func (r *Role) RoleID() RoleID { return NewRoleID(r.tenantID, r.name) }

// TenantID is the tenant the role belongs to.
func (r *Role) TenantID() tenant.TenantID { return r.tenantID }

// Name is the role's name.
func (r *Role) Name() RoleName { return r.name }

// Description is the free text the role was described with.
func (r *Role) Description() string { return r.description }

// Members lists the usernames assigned to the role.
func (r *Role) Members() []user.Username {
	out := make([]user.Username, len(r.members))
	copy(out, r.members)
	return out
}

// IsAssigned is a side-effect-free function: whether the user plays the role.
func (r *Role) IsAssigned(username user.Username) bool {
	for _, m := range r.members {
		if m == username {
			return true
		}
	}
	return false
}

// AssignUser grants the role to a user of the same tenant. Assigning a
// user who already plays the role changes nothing.
func (r *Role) AssignUser(id user.UserID) error {
	if err := r.AssertAssigneeBelongsToTenant(id); err != nil {
		return err
	}
	if r.IsAssigned(id.Username) {
		return nil
	}
	r.members = append(r.members, id.Username)
	if err := r.EnsureMembersListedOnce(); err != nil {
		return err
	}
	r.Raise(UserAssignedToRole{TenantID: r.tenantID, Name: r.name, Username: id.Username, occurredOn: r.clock()})
	return nil
}

// UnassignUser revokes the role from a user.
func (r *Role) UnassignUser(id user.UserID) error {
	if err := r.AssertAssigneeBelongsToTenant(id); err != nil {
		return err
	}
	kept := r.members[:0]
	for _, m := range r.members {
		if m != id.Username {
			kept = append(kept, m)
		}
	}
	r.members = kept
	return r.EnsureMembersListedOnce()
}

// EnsureMembersListedOnce enforces the aggregate invariant
// members-listed-once: a role lists each member once.
func (r *Role) EnsureMembersListedOnce() error {
	seen := make(map[user.Username]struct{}, len(r.members))
	for _, m := range r.members {
		if _, dup := seen[m]; dup {
			return fmt.Errorf("role %s: %w: %s", r.name, ErrMemberListedTwice, m)
		}
		seen[m] = struct{}{}
	}
	return nil
}

// AssertAssigneeBelongsToTenant checks the assertion
// assignee-belongs-to-tenant on AssignUser and UnassignUser: the user
// belongs to the role's tenant.
func (r *Role) AssertAssigneeBelongsToTenant(id user.UserID) error {
	if id.TenantID != r.tenantID {
		return fmt.Errorf("role %s: %w", r.name, ErrAssigneeOfAnotherTenant)
	}
	return nil
}
