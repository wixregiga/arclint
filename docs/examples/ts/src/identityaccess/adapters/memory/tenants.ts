import { TenantID, TenantNameTaken, TenantNotFound, type Tenant, type TenantName, type TenantRepository } from "../../tenant/index.js";

/** In-memory TenantRepository; the reference implementation tests run against. */
export class InMemoryTenants implements TenantRepository {
  private readonly byId = new Map<string, Tenant>();
  private readonly byName = new Map<string, string>();

  nextIdentity(): TenantID {
    return TenantID.next();
  }

  async add(tenant: Tenant): Promise<void> {
    if (this.byName.has(tenant.name.toString())) {
      throw new TenantNameTaken(tenant.name);
    }
    this.byId.set(tenant.tenantId.toString(), tenant);
    this.byName.set(tenant.name.toString(), tenant.tenantId.toString());
  }

  async save(tenant: Tenant): Promise<void> {
    if (!this.byId.has(tenant.tenantId.toString())) {
      throw new TenantNotFound(tenant.tenantId.toString());
    }
    this.byId.set(tenant.tenantId.toString(), tenant);
  }

  async tenantOfId(id: TenantID): Promise<Tenant> {
    const tenant = this.byId.get(id.toString());
    if (tenant === undefined) {
      throw new TenantNotFound(id.toString());
    }
    return tenant;
  }

  async tenantNamed(name: TenantName): Promise<Tenant | undefined> {
    const id = this.byName.get(name.toString());
    return id === undefined ? undefined : this.byId.get(id);
  }
}
