import type { DomainEvent } from "../event/index.js";
import { Role, RoleName, type RoleID, type RoleRepository } from "../role/index.js";
import { Tenant, TenantNameTaken, type RegistrationInvitation, type TenantID, type TenantName, type TenantRepository } from "../tenant/index.js";
import { Credentials, Enablement, Password, Person, User, Username, type EmailAddress, type Encrypter, type FullName, type UserID, type UserRepository } from "../user/index.js";

/** The name of the role every tenant starts with. */
export const ADMINISTRATOR_ROLE_NAME = "Administrator";

const INITIAL_INVITATION = "init";

/** The domain service that proposes a strong password for a user who has not chosen one. */
export interface PasswordGenerator {
  generateStrongPassword(): Password;
}

/** What the process needs to know about the first administrator. */
export interface Administrator {
  readonly name: FullName;
  readonly email: EmailAddress;
}

/** The outcome of provisionTenant. */
export interface Provisioned {
  readonly tenantId: TenantID;
  readonly administratorId: UserID;
  readonly administratorRole: RoleID;
  readonly initialPassword: Password;
  readonly events: DomainEvent[];
}

export type Clock = () => Date;

/**
 * The domain service that brings a tenant into existence together with
 * its first administrator. The process spans Tenant, User, and Role, so
 * no one of them is its natural home. It holds collaborators, never
 * state across invocations.
 */
export class TenantProvisioningService {
  constructor(
    private readonly tenants: TenantRepository,
    private readonly users: UserRepository,
    private readonly roles: RoleRepository,
    private readonly passwords: PasswordGenerator,
    private readonly encrypter: Encrypter,
    private readonly clock: Clock,
  ) {}

  // ANCHOR: service-operation
  /**
   * Contract (assertions on provisionTenant):
   * tenant-active-on-completion, administrator-registered, initial-invitation-withdrawn.
   */
  async provisionTenant(name: TenantName, description: string, admin: Administrator): Promise<Provisioned> {
    if ((await this.tenants.tenantNamed(name)) !== undefined) {
      throw new TenantNameTaken(name);
    }
    const tenant = Tenant.provision(this.tenants.nextIdentity(), name, description, this.clock);
    const invitation = tenant.offerRegistrationInvitation(INITIAL_INVITATION);

    const { user: adminUser, password } = this.registerAdministrator(tenant, invitation, admin);
    tenant.withdrawInvitation(invitation.invitationId);

    const adminRole = Role.provision(tenant.tenantId, RoleName.from(ADMINISTRATOR_ROLE_NAME), "Default tenant administrator.", this.clock);
    adminRole.assignUser(adminUser.userId);

    this.assertTenantActiveOnCompletion(tenant);
    this.assertAdministratorRegistered(tenant, adminUser, adminRole);
    this.assertInitialInvitationWithdrawn(tenant);

    await this.tenants.add(tenant);
    await this.users.add(adminUser);
    await this.roles.add(adminRole);

    return {
      tenantId: tenant.tenantId,
      administratorId: adminUser.userId,
      administratorRole: adminRole.roleId,
      initialPassword: password,
      events: [...tenant.drainEvents(), ...adminUser.drainEvents(), ...adminRole.drainEvents()],
    };
  }
  // ANCHOR_END: service-operation

  private registerAdministrator(tenant: Tenant, invitation: RegistrationInvitation, admin: Administrator): { user: User; password: Password } {
    if (!tenant.isRegistrationAvailableThrough(invitation.invitationId)) {
      throw new Error(`tenant ${tenant.tenantId.toString()}: no registration invitation answers to that identifier`);
    }
    const username = Username.from(admin.email.address);
    const password = this.passwords.generateStrongPassword();
    const credentials = Credentials.of(username, password);
    const person = Person.of(tenant.tenantId, admin.name, admin.email);
    const user = User.register(tenant.tenantId, credentials, Enablement.indefinite(), person, this.encrypter, this.clock);
    return { user, password };
  }

  // ANCHOR: service-assert
  /** Checks the contract assertion tenant-active-on-completion on provisionTenant. */
  assertTenantActiveOnCompletion(tenant: Tenant): void {
    if (!tenant.isActive()) {
      throw new Error(`tenant ${tenant.tenantId.toString()}: provisioning left the tenant inactive`);
    }
  }
  // ANCHOR_END: service-assert

  /** Checks the contract assertion administrator-registered on provisionTenant. */
  assertAdministratorRegistered(tenant: Tenant, admin: User, adminRole: Role): void {
    const sameTenant = admin.tenantId.equals(tenant.tenantId) && adminRole.tenantId.equals(tenant.tenantId);
    if (!sameTenant || adminRole.name.toString() !== ADMINISTRATOR_ROLE_NAME || !admin.isEnabled() || !adminRole.isAssigned(admin.username)) {
      throw new Error(`tenant ${tenant.tenantId.toString()}: provisioning left the tenant without an administrator`);
    }
  }

  /** Checks the contract assertion initial-invitation-withdrawn on provisionTenant. */
  assertInitialInvitationWithdrawn(tenant: Tenant): void {
    if (tenant.isRegistrationAvailableThrough(INITIAL_INVITATION)) {
      throw new Error(`tenant ${tenant.tenantId.toString()}: provisioning left the initial invitation open`);
    }
  }
}
