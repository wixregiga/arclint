import type { DomainEvent } from "../event/index.js";
import type { TenantID } from "../tenant/index.js";
import type { Username } from "../user/index.js";
import type { RoleName } from "./role-name.js";

/** A tenant gained a role. */
export class RoleProvisioned implements DomainEvent {
  constructor(readonly tenantId: TenantID, readonly name: RoleName, readonly occurredOn: Date) {}
}

/** A user now plays a role; access control in every other context cares. */
export class UserAssignedToRole implements DomainEvent {
  constructor(readonly tenantId: TenantID, readonly name: RoleName, readonly username: Username, readonly occurredOn: Date) {}
}
