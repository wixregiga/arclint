package tenant

import "time"

// ANCHOR: event

// TenantProvisioned records that a new tenant came into existence; the
// provisioning use case and the billing context care.
type TenantProvisioned struct {
	TenantID   TenantID
	Name       TenantName
	occurredOn time.Time
}

// ANCHOR_END: event

// OccurredOn is the instant the tenant was provisioned.
func (e TenantProvisioned) OccurredOn() time.Time { return e.occurredOn }

// TenantActivated records that a tenant may again be used.
type TenantActivated struct {
	TenantID   TenantID
	occurredOn time.Time
}

// OccurredOn is the instant the tenant was activated.
func (e TenantActivated) OccurredOn() time.Time { return e.occurredOn }

// TenantDeactivated records that a tenant was taken out of use.
type TenantDeactivated struct {
	TenantID   TenantID
	occurredOn time.Time
}

// OccurredOn is the instant the tenant was deactivated.
func (e TenantDeactivated) OccurredOn() time.Time { return e.occurredOn }
