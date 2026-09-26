package user

import "example.com/saasovation/identityaccess/tenant"

// UserID is the identity of a User: the tenant it belongs to and its
// username, which is unique within that tenant.
type UserID struct {
	TenantID tenant.TenantID
	Username Username
}

// NewUserID composes the identity from its two parts.
func NewUserID(tenantID tenant.TenantID, username Username) UserID {
	return UserID{TenantID: tenantID, Username: username}
}
