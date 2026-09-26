// Package provisioning holds the TenantProvisioningService, the domain
// service that brings a tenant into existence together with its first
// administrator. The process spans three aggregates (Tenant, User, Role),
// so no one of them is its natural home.
package provisioning

import (
	"context"
	"errors"
	"fmt"
	"time"

	"example.com/saasovation/identityaccess/event"
	"example.com/saasovation/identityaccess/role"
	"example.com/saasovation/identityaccess/tenant"
	"example.com/saasovation/identityaccess/user"
)

// AdministratorRoleName is the name of the role every tenant starts with.
const AdministratorRoleName = "Administrator"

// initialInvitation is the description of the invitation the first
// administrator registers through; it is withdrawn once used.
const initialInvitation = "init"

// PasswordGenerator is the domain service that proposes a strong password
// for a user who has not chosen one yet. Infrastructure implements it.
type PasswordGenerator interface {
	GenerateStrongPassword() (user.Password, error)
}

// Administrator is what the process needs to know about the first
// administrator of a new tenant.
type Administrator struct {
	Name  user.FullName
	Email user.EmailAddress
}

// Provisioned is the outcome of ProvisionTenant: the identities the
// process created, the generated password to hand to the administrator
// once, and every event the three aggregates raised.
type Provisioned struct {
	TenantID          tenant.TenantID
	AdministratorID   user.UserID
	AdministratorRole role.RoleID
	InitialPassword   user.Password
	Events            []event.Event
}

// TenantProvisioningService provisions tenants. It holds its collaborators
// and no state across invocations.
type TenantProvisioningService struct {
	tenants   tenant.Repository
	users     user.Repository
	roles     role.Repository
	passwords PasswordGenerator
	encrypter user.Encrypter
	clock     func() time.Time
}

// NewTenantProvisioningService wires the service to its collaborators.
func NewTenantProvisioningService(tenants tenant.Repository, users user.Repository, roles role.Repository, passwords PasswordGenerator, encrypter user.Encrypter, clock func() time.Time) *TenantProvisioningService {
	return &TenantProvisioningService{tenants: tenants, users: users, roles: roles, passwords: passwords, encrypter: encrypter, clock: clock}
}

// ErrTenantLeftInactive is returned when the process ends with an inactive tenant.
var ErrTenantLeftInactive = errors.New("provisioning left the tenant inactive")

// ErrAdministratorNotRegistered is returned when the process ends without
// an enabled administrator in the Administrator role.
var ErrAdministratorNotRegistered = errors.New("provisioning left the tenant without an administrator")

// ErrInitialInvitationLeftOpen is returned when the process ends with the
// initial invitation still available.
var ErrInitialInvitationLeftOpen = errors.New("provisioning left the initial invitation open")

// ANCHOR: service-operation

// ProvisionTenant brings a tenant into existence with its Administrator
// role and first administrator, registered through a one-time invitation.
//
// Contract (assertions on ProvisionTenant):
//   - tenant-active-on-completion
//   - administrator-registered
//   - initial-invitation-withdrawn
func (s *TenantProvisioningService) ProvisionTenant(ctx context.Context, name tenant.TenantName, description string, admin Administrator) (Provisioned, error) {
	if existing, err := s.tenants.TenantNamed(ctx, name); err == nil && existing != nil {
		return Provisioned{}, fmt.Errorf("provision tenant %q: %w", name, tenant.ErrTenantNameTaken)
	} else if err != nil && !errors.Is(err, tenant.ErrTenantNotFound) {
		return Provisioned{}, fmt.Errorf("provision tenant %q: %w", name, err)
	}

	t, err := tenant.Provision(s.tenants.NextIdentity(), name, description, s.clock)
	if err != nil {
		return Provisioned{}, err
	}
	invitation, err := t.OfferRegistrationInvitation(initialInvitation)
	if err != nil {
		return Provisioned{}, err
	}

	adminUser, password, err := s.registerAdministrator(t, invitation, admin)
	if err != nil {
		return Provisioned{}, err
	}
	if err := t.WithdrawInvitation(invitation.InvitationID()); err != nil {
		return Provisioned{}, err
	}

	adminRoleName, err := role.NewRoleName(AdministratorRoleName)
	if err != nil {
		return Provisioned{}, err
	}
	adminRole, err := role.Provision(t.TenantID(), adminRoleName, "Default tenant administrator.", s.clock)
	if err != nil {
		return Provisioned{}, err
	}
	if err := adminRole.AssignUser(adminUser.UserID()); err != nil {
		return Provisioned{}, err
	}

	if err := s.AssertTenantActiveOnCompletion(t); err != nil {
		return Provisioned{}, err
	}
	if err := s.AssertAdministratorRegistered(t, adminUser, adminRole); err != nil {
		return Provisioned{}, err
	}
	if err := s.AssertInitialInvitationWithdrawn(t); err != nil {
		return Provisioned{}, err
	}

	if err := s.tenants.Add(ctx, t); err != nil {
		return Provisioned{}, fmt.Errorf("provision tenant %q: %w", name, err)
	}
	if err := s.users.Add(ctx, adminUser); err != nil {
		return Provisioned{}, fmt.Errorf("provision tenant %q: %w", name, err)
	}
	if err := s.roles.Add(ctx, adminRole); err != nil {
		return Provisioned{}, fmt.Errorf("provision tenant %q: %w", name, err)
	}

	events := append(t.Events(), adminUser.Events()...)
	events = append(events, adminRole.Events()...)
	return Provisioned{
		TenantID:          t.TenantID(),
		AdministratorID:   adminUser.UserID(),
		AdministratorRole: adminRole.RoleID(),
		InitialPassword:   password,
		Events:            events,
	}, nil
}

// ANCHOR_END: service-operation

func (s *TenantProvisioningService) registerAdministrator(t *tenant.Tenant, invitation tenant.RegistrationInvitation, admin Administrator) (*user.User, user.Password, error) {
	if !t.IsRegistrationAvailableThrough(invitation.InvitationID()) {
		return nil, user.Password{}, fmt.Errorf("tenant %s: %w", t.TenantID(), tenant.ErrInvitationUnknown)
	}
	username, err := user.NewUsername(admin.Email.Address())
	if err != nil {
		return nil, user.Password{}, err
	}
	password, err := s.passwords.GenerateStrongPassword()
	if err != nil {
		return nil, user.Password{}, fmt.Errorf("generate administrator password: %w", err)
	}
	credentials, err := user.NewCredentials(username, password)
	if err != nil {
		return nil, user.Password{}, err
	}
	person, err := user.NewPerson(t.TenantID(), admin.Name, admin.Email)
	if err != nil {
		return nil, user.Password{}, err
	}
	adminUser, err := user.Register(t.TenantID(), credentials, user.IndefiniteEnablement(), person, s.encrypter, s.clock)
	if err != nil {
		return nil, user.Password{}, err
	}
	return adminUser, password, nil
}

// ANCHOR: service-assert

// AssertTenantActiveOnCompletion checks the contract assertion
// tenant-active-on-completion on ProvisionTenant: the provisioned tenant
// is active.
func (s *TenantProvisioningService) AssertTenantActiveOnCompletion(t *tenant.Tenant) error {
	if !t.IsActive() {
		return fmt.Errorf("tenant %s: %w", t.TenantID(), ErrTenantLeftInactive)
	}
	return nil
}

// ANCHOR_END: service-assert

// AssertAdministratorRegistered checks the contract assertion
// administrator-registered on ProvisionTenant: an enabled user of the
// tenant plays its Administrator role.
func (s *TenantProvisioningService) AssertAdministratorRegistered(t *tenant.Tenant, admin *user.User, adminRole *role.Role) error {
	if admin.TenantID() != t.TenantID() || adminRole.TenantID() != t.TenantID() {
		return fmt.Errorf("tenant %s: %w", t.TenantID(), ErrAdministratorNotRegistered)
	}
	if adminRole.Name().String() != AdministratorRoleName || !admin.IsEnabled() || !adminRole.IsAssigned(admin.Username()) {
		return fmt.Errorf("tenant %s: %w", t.TenantID(), ErrAdministratorNotRegistered)
	}
	return nil
}

// AssertInitialInvitationWithdrawn checks the contract assertion
// initial-invitation-withdrawn on ProvisionTenant: nobody else registers
// through the invitation the administrator used.
func (s *TenantProvisioningService) AssertInitialInvitationWithdrawn(t *tenant.Tenant) error {
	if t.IsRegistrationAvailableThrough(initialInvitation) {
		return fmt.Errorf("tenant %s: %w", t.TenantID(), ErrInitialInvitationLeftOpen)
	}
	return nil
}
