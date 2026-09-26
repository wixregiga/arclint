import type { TenantID } from "../../tenant/index.js";
import { UserNotFound, UsernameTaken, type User, type UserID, type UserRepository } from "../../user/index.js";

/** In-memory UserRepository. */
export class InMemoryUsers implements UserRepository {
  private readonly byId = new Map<string, User>();

  async add(user: User): Promise<void> {
    if (this.byId.has(user.userId.toString())) {
      throw new UsernameTaken(user.userId);
    }
    this.byId.set(user.userId.toString(), user);
  }

  async save(user: User): Promise<void> {
    if (!this.byId.has(user.userId.toString())) {
      throw new UserNotFound(user.userId);
    }
    this.byId.set(user.userId.toString(), user);
  }

  async userOfId(id: UserID): Promise<User> {
    const user = this.byId.get(id.toString());
    if (user === undefined) {
      throw new UserNotFound(id);
    }
    return user;
  }

  async usersOfTenant(tenantId: TenantID): Promise<User[]> {
    return [...this.byId.values()].filter((u) => u.tenantId.equals(tenantId));
  }
}
