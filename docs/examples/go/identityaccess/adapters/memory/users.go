package memory

import (
	"context"
	"sync"

	"example.com/saasovation/identityaccess/tenant"
	"example.com/saasovation/identityaccess/user"
)

// Users implements user.Repository in memory.
type Users struct {
	mu   sync.RWMutex
	byID map[user.UserID]*user.User
}

// NewUsers makes an empty collection.
func NewUsers() *Users { return &Users{byID: map[user.UserID]*user.User{}} }

// Add stores a new user, refusing a taken username within the tenant.
func (r *Users) Add(_ context.Context, u *user.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, taken := r.byID[u.UserID()]; taken {
		return user.ErrUsernameTaken
	}
	r.byID[u.UserID()] = u
	return nil
}

// Save stores the current state of a known user.
func (r *Users) Save(_ context.Context, u *user.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, known := r.byID[u.UserID()]; !known {
		return user.ErrUserNotFound
	}
	r.byID[u.UserID()] = u
	return nil
}

// UserOfID finds a user by identity.
func (r *Users) UserOfID(_ context.Context, id user.UserID) (*user.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.byID[id]
	if !ok {
		return nil, user.ErrUserNotFound
	}
	return u, nil
}

// UsersOfTenant lists every user of a tenant.
func (r *Users) UsersOfTenant(_ context.Context, tenantID tenant.TenantID) ([]*user.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*user.User
	for id, u := range r.byID {
		if id.TenantID == tenantID {
			out = append(out, u)
		}
	}
	return out, nil
}
