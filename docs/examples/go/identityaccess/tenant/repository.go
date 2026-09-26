package tenant

import (
	"context"
	"errors"
)

// ErrTenantNotFound is returned when no tenant carries the requested identity.
var ErrTenantNotFound = errors.New("tenant not found")

// ErrTenantNameTaken is returned when another tenant already registered the name.
var ErrTenantNameTaken = errors.New("tenant name is already registered")

// Repository is the collection of all tenants. Names are unique across
// the collection; Add rejects a second tenant with a registered name.
type Repository interface {
	NextIdentity() TenantID
	Add(ctx context.Context, t *Tenant) error
	Save(ctx context.Context, t *Tenant) error
	TenantOfID(ctx context.Context, id TenantID) (*Tenant, error)
	TenantNamed(ctx context.Context, name TenantName) (*Tenant, error)
}
