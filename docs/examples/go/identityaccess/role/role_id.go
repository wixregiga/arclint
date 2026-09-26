package role

import "example.com/saasovation/identityaccess/tenant"

// RoleID is the identity of a Role: the tenant it belongs to and its name.
type RoleID struct {
	TenantID tenant.TenantID
	Name     RoleName
}

// NewRoleID composes the identity from its two parts.
func NewRoleID(tenantID tenant.TenantID, name RoleName) RoleID {
	return RoleID{TenantID: tenantID, Name: name}
}
