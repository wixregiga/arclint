import { EventRecorder, type DomainEvent } from "../event/index.js";
import type { TenantID } from "../tenant/index.js";
import type { UserID, Username } from "../user/index.js";
import { RoleProvisioned, UserAssignedToRole } from "./events.js";
import { RoleID } from "./role-id.js";
import type { RoleName } from "./role-name.js";

export type Clock = () => Date;

/** The Role aggregate root: a named capability a tenant grants to some of its users, listed by identity. */
export class Role {
  private readonly recorder = new EventRecorder();

  private constructor(
    private readonly tenant: TenantID,
    private readonly roleName: RoleName,
    private readonly roleDescription: string,
    private members: Username[],
    private readonly clock: Clock,
  ) {
    this.ensureMembersListedOnce();
  }

  /** The constructor door: a tenant provisions a role by name. */
  static provision(tenantId: TenantID, name: RoleName, description: string, clock: Clock): Role {
    const role = new Role(tenantId, name, description, [], clock);
    role.recorder.raise(new RoleProvisioned(tenantId, name, clock()));
    return role;
  }

  /** Rebuilds a role from persisted state; a repository adapter is its only caller. */
  static reconstitute(tenantId: TenantID, name: RoleName, description: string, members: Username[], clock: Clock): Role {
    return new Role(tenantId, name, description, [...members], clock);
  }

  get roleId(): RoleID {
    return new RoleID(this.tenant, this.roleName);
  }

  get tenantId(): TenantID {
    return this.tenant;
  }

  get name(): RoleName {
    return this.roleName;
  }

  get description(): string {
    return this.roleDescription;
  }

  assignedUsernames(): readonly Username[] {
    return [...this.members];
  }

  drainEvents(): DomainEvent[] {
    return this.recorder.drain();
  }

  /** Side-effect-free: whether the user plays the role. */
  isAssigned(username: Username): boolean {
    return this.members.some((m) => m.equals(username));
  }

  /** Grants the role to a user of the same tenant; assigning twice changes nothing. */
  assignUser(id: UserID): void {
    this.assertAssigneeBelongsToTenant(id);
    if (this.isAssigned(id.username)) {
      return;
    }
    this.members.push(id.username);
    this.ensureMembersListedOnce();
    this.recorder.raise(new UserAssignedToRole(this.tenant, this.roleName, id.username, this.clock()));
  }

  unassignUser(id: UserID): void {
    this.assertAssigneeBelongsToTenant(id);
    this.members = this.members.filter((m) => !m.equals(id.username));
    this.ensureMembersListedOnce();
  }

  /** Enforces the aggregate invariant members-listed-once: a role lists each member once. */
  ensureMembersListedOnce(): void {
    const seen = new Set<string>();
    for (const m of this.members) {
      if (seen.has(m.toString())) {
        throw new Error(`role ${this.roleName.toString()}: member listed twice: ${m.toString()}`);
      }
      seen.add(m.toString());
    }
  }

  /** Checks the assertion assignee-belongs-to-tenant on assignUser and unassignUser. */
  assertAssigneeBelongsToTenant(id: UserID): void {
    if (!id.tenantId.equals(this.tenant)) {
      throw new Error(`role ${this.roleName.toString()}: the user belongs to another tenant`);
    }
  }
}
