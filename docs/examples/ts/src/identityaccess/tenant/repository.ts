import type { Tenant } from "./tenant.js";
import type { TenantID } from "./tenant-id.js";
import type { TenantName } from "./tenant-name.js";

export class TenantNotFound extends Error {
  constructor(readonly identifier: string) {
    super(`tenant not found: ${identifier}`);
  }
}

export class TenantNameTaken extends Error {
  constructor(readonly tenantName: TenantName) {
    super(`tenant name is already registered: ${tenantName.toString()}`);
  }
}

/** The collection of all tenants; names are unique across it. */
export interface TenantRepository {
  nextIdentity(): TenantID;
  add(tenant: Tenant): Promise<void>;
  save(tenant: Tenant): Promise<void>;
  tenantOfId(id: TenantID): Promise<Tenant>;
  tenantNamed(name: TenantName): Promise<Tenant | undefined>;
}
