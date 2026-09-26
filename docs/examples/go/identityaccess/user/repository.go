package user

import (
	"context"
	"errors"

	"example.com/saasovation/identityaccess/tenant"
)

// ErrUserNotFound is returned when no user carries the requested identity.
var ErrUserNotFound = errors.New("user not found")

// ErrUsernameTaken is returned when the tenant already has a user of that name.
var ErrUsernameTaken = errors.New("username is already registered under this tenant")

// Repository is the collection of all users of all tenants.
type Repository interface {
	Add(ctx context.Context, u *User) error
	Save(ctx context.Context, u *User) error
	UserOfID(ctx context.Context, id UserID) (*User, error)
	UsersOfTenant(ctx context.Context, tenantID tenant.TenantID) ([]*User, error)
}
