// Package memory holds in-memory adapters of the identityaccess
// repositories: the reference implementation the use case tests run
// against and the fallback for local development.
package memory

import (
	"context"
	"sync"

	"example.com/saasovation/identityaccess/tenant"
)

// Tenants implements tenant.Repository in memory.
type Tenants struct {
	mu    sync.RWMutex
	byID  map[tenant.TenantID]*tenant.Tenant
	names map[string]tenant.TenantID
}

// NewTenants makes an empty collection.
func NewTenants() *Tenants {
	return &Tenants{byID: map[tenant.TenantID]*tenant.Tenant{}, names: map[string]tenant.TenantID{}}
}

// NextIdentity mints a fresh TenantID.
func (r *Tenants) NextIdentity() tenant.TenantID { return tenant.NewTenantID() }

// Add stores a new tenant, refusing a registered name.
func (r *Tenants) Add(_ context.Context, t *tenant.Tenant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, taken := r.names[t.Name().String()]; taken {
		return tenant.ErrTenantNameTaken
	}
	r.byID[t.TenantID()] = t
	r.names[t.Name().String()] = t.TenantID()
	return nil
}

// Save stores the current state of a known tenant.
func (r *Tenants) Save(_ context.Context, t *tenant.Tenant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, known := r.byID[t.TenantID()]; !known {
		return tenant.ErrTenantNotFound
	}
	r.byID[t.TenantID()] = t
	return nil
}

// TenantOfID finds a tenant by identity.
func (r *Tenants) TenantOfID(_ context.Context, id tenant.TenantID) (*tenant.Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.byID[id]
	if !ok {
		return nil, tenant.ErrTenantNotFound
	}
	return t, nil
}

// TenantNamed finds a tenant by name.
func (r *Tenants) TenantNamed(_ context.Context, name tenant.TenantName) (*tenant.Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.names[name.String()]
	if !ok {
		return nil, tenant.ErrTenantNotFound
	}
	return r.byID[id], nil
}
