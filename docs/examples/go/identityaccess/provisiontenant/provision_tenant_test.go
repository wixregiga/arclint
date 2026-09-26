package provisiontenant_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"example.com/saasovation/identityaccess/adapters/memory"
	"example.com/saasovation/identityaccess/adapters/passwords"
	"example.com/saasovation/identityaccess/provisioning"
	"example.com/saasovation/identityaccess/provisiontenant"
	"example.com/saasovation/identityaccess/role"
	"example.com/saasovation/identityaccess/tenant"
	"example.com/saasovation/identityaccess/user"
)

type fixture struct {
	tenants *memory.Tenants
	users   *memory.Users
	roles   *memory.Roles
	log     *memory.EventLog
	useCase *provisiontenant.ProvisionTenant
}

func newFixture() fixture {
	tenants, users, roles, log := memory.NewTenants(), memory.NewUsers(), memory.NewRoles(), &memory.EventLog{}
	service := provisioning.NewTenantProvisioningService(tenants, users, roles, passwords.RandomGenerator{}, passwords.PBKDF2Encrypter{}, time.Now)
	return fixture{tenants: tenants, users: users, roles: roles, log: log, useCase: provisiontenant.NewProvisionTenant(service, log, memory.UnitOfWork{})}
}

func TestProvisionTenantRegistersAnEnabledAdministrator(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	out, err := f.useCase.Execute(ctx, provisiontenant.ProvisionTenantRequest{
		TenantName:                "SaaSOvation",
		TenantDescription:         "The first tenant.",
		AdministratorFirstName:    "Jane",
		AdministratorLastName:     "Doe",
		AdministratorEmailAddress: "jane.doe@saasovation.example",
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	tn, err := f.tenants.TenantOfID(ctx, out.TenantID)
	if err != nil {
		t.Fatalf("tenant not stored: %v", err)
	}
	if !tn.IsActive() {
		t.Fatal("tenant must be active after provisioning")
	}
	if tn.IsRegistrationAvailableThrough("init") {
		t.Fatal("initial invitation must be withdrawn")
	}

	admin, err := f.users.UserOfID(ctx, out.AdministratorID)
	if err != nil {
		t.Fatalf("administrator not stored: %v", err)
	}
	if !admin.IsEnabled() {
		t.Fatal("administrator must be enabled")
	}
	if !(passwords.PBKDF2Encrypter{}).Matches(out.InitialPassword, admin.Password()) {
		t.Fatal("initial password must match the stored one")
	}

	adminRoleName, _ := role.NewRoleName(provisioning.AdministratorRoleName)
	adminRole, err := f.roles.RoleOfID(ctx, role.NewRoleID(out.TenantID, adminRoleName))
	if err != nil {
		t.Fatalf("administrator role not stored: %v", err)
	}
	if !adminRole.IsAssigned(admin.Username()) {
		t.Fatal("administrator must play the Administrator role")
	}

	var provisioned, registered, assigned bool
	for _, e := range f.log.Published() {
		switch e.(type) {
		case tenant.TenantProvisioned:
			provisioned = true
		case user.UserRegistered:
			registered = true
		case role.UserAssignedToRole:
			assigned = true
		}
	}
	if !provisioned || !registered || !assigned {
		t.Fatalf("expected TenantProvisioned, UserRegistered, UserAssignedToRole; got %#v", f.log.Published())
	}
}

func TestProvisionTenantRefusesARegisteredName(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	req := provisiontenant.ProvisionTenantRequest{TenantName: "Twice", AdministratorFirstName: "A", AdministratorLastName: "B", AdministratorEmailAddress: "a.b@example.com"}
	if _, err := f.useCase.Execute(ctx, req); err != nil {
		t.Fatalf("first execute: %v", err)
	}
	_, err := f.useCase.Execute(ctx, req)
	if !errors.Is(err, tenant.ErrTenantNameTaken) {
		t.Fatalf("expected ErrTenantNameTaken, got %v", err)
	}
}

func TestProvisionTenantRejectsMalformedInputAtTheDoor(t *testing.T) {
	f := newFixture()
	_, err := f.useCase.Execute(context.Background(), provisiontenant.ProvisionTenantRequest{TenantName: " ", AdministratorFirstName: "A", AdministratorLastName: "B", AdministratorEmailAddress: "a@example.com"})
	if !errors.Is(err, tenant.ErrTenantNameBlank) {
		t.Fatalf("expected ErrTenantNameBlank, got %v", err)
	}
	_, err = f.useCase.Execute(context.Background(), provisiontenant.ProvisionTenantRequest{TenantName: "Ok", AdministratorFirstName: "A", AdministratorLastName: "B", AdministratorEmailAddress: "not an address"})
	if !errors.Is(err, user.ErrEmailAddressMalformed) {
		t.Fatalf("expected ErrEmailAddressMalformed, got %v", err)
	}
}

func TestDeactivatedTenantOffersNoInvitation(t *testing.T) {
	name, _ := tenant.NewTenantName("Dormant")
	tn, err := tenant.Provision(tenant.NewTenantID(), name, "", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err := tn.Deactivate(); err != nil {
		t.Fatal(err)
	}
	if _, err := tn.OfferRegistrationInvitation("open"); !errors.Is(err, tenant.ErrTenantInactive) {
		t.Fatalf("expected ErrTenantInactive, got %v", err)
	}
}
