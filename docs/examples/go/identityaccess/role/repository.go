package role

import (
	"context"
	"errors"

	"example.com/saasovation/identityaccess/tenant"
)

// ErrRoleNotFound is returned when no role carries the requested identity.
var ErrRoleNotFound = errors.New("role not found")

// ErrRoleNameTaken is returned when the tenant already has a role of that name.
var ErrRoleNameTaken = errors.New("role name is already provisioned under this tenant")

// Repository is the collection of all roles of all tenants.
type Repository interface {
	Add(ctx context.Context, r *Role) error
	Save(ctx context.Context, r *Role) error
	RoleOfID(ctx context.Context, id RoleID) (*Role, error)
	RolesOfTenant(ctx context.Context, tenantID tenant.TenantID) ([]*Role, error)
}
