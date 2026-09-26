import type { TenantID } from "../tenant/index.js";
import type { Username } from "./username.js";

/** The identity of a User: its tenant and its username, unique within that tenant. */
export class UserID {
  constructor(readonly tenantId: TenantID, readonly username: Username) {}

  equals(other: UserID): boolean {
    return this.tenantId.equals(other.tenantId) && this.username.equals(other.username);
  }

  toString(): string {
    return `${this.tenantId.toString()}/${this.username.toString()}`;
  }
}
