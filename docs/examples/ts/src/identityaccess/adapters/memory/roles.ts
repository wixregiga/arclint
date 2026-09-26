import { RoleNameTaken, RoleNotFound, type Role, type RoleID, type RoleRepository } from "../../role/index.js";
import type { TenantID } from "../../tenant/index.js";

/** In-memory RoleRepository. */
export class InMemoryRoles implements RoleRepository {
  private readonly byId = new Map<string, Role>();

  async add(role: Role): Promise<void> {
    if (this.byId.has(role.roleId.toString())) {
      throw new RoleNameTaken(role.roleId);
    }
    this.byId.set(role.roleId.toString(), role);
  }

  async save(role: Role): Promise<void> {
    if (!this.byId.has(role.roleId.toString())) {
      throw new RoleNotFound(role.roleId);
    }
    this.byId.set(role.roleId.toString(), role);
  }

  async roleOfId(id: RoleID): Promise<Role> {
    const role = this.byId.get(id.toString());
    if (role === undefined) {
      throw new RoleNotFound(id);
    }
    return role;
  }

  async rolesOfTenant(tenantId: TenantID): Promise<Role[]> {
    return [...this.byId.values()].filter((r) => r.tenantId.equals(tenantId));
  }
}
