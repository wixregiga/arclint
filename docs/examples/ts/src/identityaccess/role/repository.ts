import type { TenantID } from "../tenant/index.js";
import type { Role } from "./role.js";
import type { RoleID } from "./role-id.js";

export class RoleNotFound extends Error {
  constructor(readonly id: RoleID) {
    super(`role not found: ${id.toString()}`);
  }
}

export class RoleNameTaken extends Error {
  constructor(readonly id: RoleID) {
    super(`role name is already provisioned under this tenant: ${id.toString()}`);
  }
}

/** The collection of all roles of all tenants. */
export interface RoleRepository {
  add(role: Role): Promise<void>;
  save(role: Role): Promise<void>;
  roleOfId(id: RoleID): Promise<Role>;
  rolesOfTenant(tenantId: TenantID): Promise<Role[]>;
}
