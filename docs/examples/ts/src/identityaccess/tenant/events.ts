import type { DomainEvent } from "../event/index.js";
import type { TenantID } from "./tenant-id.js";
import type { TenantName } from "./tenant-name.js";

// ANCHOR: event
/** A new tenant came into existence; provisioning and billing care. */
export class TenantProvisioned implements DomainEvent {
  constructor(readonly tenantId: TenantID, readonly name: TenantName, readonly occurredOn: Date) {}
}
// ANCHOR_END: event

/** A tenant may again be used. */
export class TenantActivated implements DomainEvent {
  constructor(readonly tenantId: TenantID, readonly occurredOn: Date) {}
}

/** A tenant was taken out of use. */
export class TenantDeactivated implements DomainEvent {
  constructor(readonly tenantId: TenantID, readonly occurredOn: Date) {}
}
