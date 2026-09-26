import assert from "node:assert/strict";
import { test } from "node:test";
import { DirectUnitOfWork, InMemoryEventLog, InMemoryRoles, InMemoryTenants, InMemoryUsers } from "../adapters/memory/index.js";
import { Pbkdf2Encrypter, RandomPasswordGenerator } from "../adapters/passwords/index.js";
import { ADMINISTRATOR_ROLE_NAME, TenantProvisioningService } from "../provisioning/index.js";
import { RoleID, RoleName, UserAssignedToRole } from "../role/index.js";
import { Tenant, TenantID, TenantName, TenantNameTaken, TenantProvisioned } from "../tenant/index.js";
import { UserRegistered } from "../user/index.js";
import { ProvisionTenant } from "./provision-tenant.js";

function fixture() {
  const tenants = new InMemoryTenants();
  const users = new InMemoryUsers();
  const roles = new InMemoryRoles();
  const log = new InMemoryEventLog();
  const encrypter = new Pbkdf2Encrypter();
  const service = new TenantProvisioningService(tenants, users, roles, new RandomPasswordGenerator(), encrypter, () => new Date());
  return { tenants, users, roles, log, encrypter, useCase: new ProvisionTenant(service, log, new DirectUnitOfWork()) };
}

test("provisioning registers an enabled administrator under an active tenant", async () => {
  const f = fixture();
  const out = await f.useCase.execute({
    tenantName: "SaaSOvation",
    tenantDescription: "The first tenant.",
    administratorFirstName: "Jane",
    administratorLastName: "Doe",
    administratorEmailAddress: "jane.doe@saasovation.example",
  });

  const tenant = await f.tenants.tenantOfId(out.tenantId);
  assert.equal(tenant.isActive(), true);
  assert.equal(tenant.isRegistrationAvailableThrough("init"), false);

  const admin = await f.users.userOfId(out.administratorId);
  assert.equal(admin.isEnabled(), true);
  assert.equal(f.encrypter.matches(out.initialPassword, admin.password), true);

  const role = await f.roles.roleOfId(new RoleID(out.tenantId, RoleName.from(ADMINISTRATOR_ROLE_NAME)));
  assert.equal(role.isAssigned(admin.username), true);

  const published = f.log.published();
  assert.ok(published.some((e) => e instanceof TenantProvisioned));
  assert.ok(published.some((e) => e instanceof UserRegistered));
  assert.ok(published.some((e) => e instanceof UserAssignedToRole));
});

test("provisioning refuses a registered name", async () => {
  const f = fixture();
  const request = { tenantName: "Twice", tenantDescription: "", administratorFirstName: "A", administratorLastName: "B", administratorEmailAddress: "a.b@example.com" };
  await f.useCase.execute(request);
  await assert.rejects(() => f.useCase.execute(request), TenantNameTaken);
});

test("malformed input is rejected at the value object door", async () => {
  const f = fixture();
  await assert.rejects(
    () => f.useCase.execute({ tenantName: " ", tenantDescription: "", administratorFirstName: "A", administratorLastName: "B", administratorEmailAddress: "a@example.com" }),
    /tenant name must not be blank/,
  );
});

test("a deactivated tenant offers no invitation", () => {
  const tenant = Tenant.provision(TenantID.next(), TenantName.from("Dormant"), "", () => new Date());
  tenant.deactivate();
  assert.throws(() => tenant.offerRegistrationInvitation("open"), /is not active/);
});
