import type { TenantID } from "../tenant/index.js";
import type { User } from "./user.js";
import type { UserID } from "./user-id.js";

export class UserNotFound extends Error {
  constructor(readonly id: UserID) {
    super(`user not found: ${id.toString()}`);
  }
}

export class UsernameTaken extends Error {
  constructor(readonly id: UserID) {
    super(`username is already registered under this tenant: ${id.toString()}`);
  }
}

/** The collection of all users of all tenants. */
export interface UserRepository {
  add(user: User): Promise<void>;
  save(user: User): Promise<void>;
  userOfId(id: UserID): Promise<User>;
  usersOfTenant(tenantId: TenantID): Promise<User[]>;
}
