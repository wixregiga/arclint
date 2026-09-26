import type { DomainEvent } from "../event/index.js";
import type { TenantID } from "../tenant/index.js";
import type { Enablement } from "./enablement.js";
import type { EmailAddress, FullName } from "./person.js";
import type { Username } from "./username.js";

/** A user account came into existence under a tenant. */
export class UserRegistered implements DomainEvent {
  constructor(
    readonly tenantId: TenantID,
    readonly username: Username,
    readonly name: FullName,
    readonly emailAddress: EmailAddress,
    readonly occurredOn: Date,
  ) {}
}

/** A user chose a new password. */
export class UserPasswordChanged implements DomainEvent {
  constructor(readonly tenantId: TenantID, readonly username: Username, readonly occurredOn: Date) {}
}

/** When a user may sign in changed. */
export class UserEnablementChanged implements DomainEvent {
  constructor(readonly tenantId: TenantID, readonly username: Username, readonly enablement: Enablement, readonly occurredOn: Date) {}
}
