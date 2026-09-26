import type { TenantID } from "../tenant/index.js";
import type { RoleName } from "./role-name.js";

/** The identity of a Role: its tenant and its name. */
export class RoleID {
  constructor(readonly tenantId: TenantID, readonly name: RoleName) {}

  equals(other: RoleID): boolean {
    return this.tenantId.equals(other.tenantId) && this.name.equals(other.name);
  }

  toString(): string {
    return `${this.tenantId.toString()}/${this.name.toString()}`;
  }
}
