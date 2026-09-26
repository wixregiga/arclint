package provisiontenant

// ProvisionTenantRequest carries what the outside world supplies to
// provision a tenant, as plain text. The use case turns it into domain
// values through their constructor doors.
type ProvisionTenantRequest struct {
	TenantName                string
	TenantDescription         string
	AdministratorFirstName    string
	AdministratorLastName     string
	AdministratorEmailAddress string
}
