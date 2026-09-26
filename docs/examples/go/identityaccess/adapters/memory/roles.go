package memory

import (
	"context"
	"sync"

	"example.com/saasovation/identityaccess/role"
	"example.com/saasovation/identityaccess/tenant"
)

// Roles implements role.Repository in memory.
type Roles struct {
	mu   sync.RWMutex
	byID map[role.RoleID]*role.Role
}

// NewRoles makes an empty collection.
func NewRoles() *Roles { return &Roles{byID: map[role.RoleID]*role.Role{}} }

// Add stores a new role, refusing a taken name within the tenant.
func (r *Roles) Add(_ context.Context, rl *role.Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, taken := r.byID[rl.RoleID()]; taken {
		return role.ErrRoleNameTaken
	}
	r.byID[rl.RoleID()] = rl
	return nil
}

// Save stores the current state of a known role.
func (r *Roles) Save(_ context.Context, rl *role.Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, known := r.byID[rl.RoleID()]; !known {
		return role.ErrRoleNotFound
	}
	r.byID[rl.RoleID()] = rl
	return nil
}

// RoleOfID finds a role by identity.
func (r *Roles) RoleOfID(_ context.Context, id role.RoleID) (*role.Role, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rl, ok := r.byID[id]
	if !ok {
		return nil, role.ErrRoleNotFound
	}
	return rl, nil
}

// RolesOfTenant lists every role of a tenant.
func (r *Roles) RolesOfTenant(_ context.Context, tenantID tenant.TenantID) ([]*role.Role, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*role.Role
	for id, rl := range r.byID {
		if id.TenantID == tenantID {
			out = append(out, rl)
		}
	}
	return out, nil
}
