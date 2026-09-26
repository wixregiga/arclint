import { EventRecorder, type DomainEvent } from "../event/index.js";
import type { TenantID } from "../tenant/index.js";
import { Credentials } from "./credentials.js";
import type { Enablement } from "./enablement.js";
import { UserEnablementChanged, UserPasswordChanged, UserRegistered } from "./events.js";
import type { EncryptedPassword, Encrypter, Password } from "./password.js";
import type { FullName, Person } from "./person.js";
import { UserID } from "./user-id.js";
import type { Username } from "./username.js";

export type Clock = () => Date;

/** The User aggregate root: an account under a tenant, the person behind it, and its credentials. */
export class User {
  private readonly recorder = new EventRecorder();

  private constructor(
    private readonly tenant: TenantID,
    private readonly name: Username,
    private encrypted: EncryptedPassword,
    private enablementValue: Enablement,
    private personValue: Person,
    private readonly clock: Clock,
  ) {
    this.ensurePersonBelongsToTheUsersTenant();
  }

  /**
   * The constructor door. The encrypter is passed in for this one
   * operation and never held.
   */
  static register(tenantId: TenantID, credentials: Credentials, enablement: Enablement, person: Person, encrypter: Encrypter, clock: Clock): User {
    const user = new User(tenantId, credentials.username, encrypter.encrypt(credentials.password), enablement, person, clock);
    user.recorder.raise(new UserRegistered(tenantId, credentials.username, person.name, person.emailAddress, clock()));
    return user;
  }

  /** Rebuilds a user from persisted state; a repository adapter is its only caller. */
  static reconstitute(tenantId: TenantID, username: Username, password: EncryptedPassword, enablement: Enablement, person: Person, clock: Clock): User {
    return new User(tenantId, username, password, enablement, person, clock);
  }

  get userId(): UserID {
    return new UserID(this.tenant, this.name);
  }

  get tenantId(): TenantID {
    return this.tenant;
  }

  get username(): Username {
    return this.name;
  }

  get password(): EncryptedPassword {
    return this.encrypted;
  }

  get enablement(): Enablement {
    return this.enablementValue;
  }

  get person(): Person {
    return this.personValue;
  }

  drainEvents(): DomainEvent[] {
    return this.recorder.drain();
  }

  /** Side-effect-free: whether the user may sign in right now. */
  isEnabled(): boolean {
    return this.enablementValue.isEnablementEnabled(this.clock());
  }

  changePassword(current: Password, changed: Password, encrypter: Encrypter): void {
    if (!encrypter.matches(current, this.encrypted)) {
      throw new Error("current password does not match");
    }
    this.assertChangedPasswordDiffersFromCurrent(current, changed);
    const credentials = Credentials.of(this.name, changed);
    this.encrypted = encrypter.encrypt(credentials.password);
    this.ensurePersonBelongsToTheUsersTenant();
    this.recorder.raise(new UserPasswordChanged(this.tenant, this.name, this.clock()));
  }

  defineEnablement(enablement: Enablement): void {
    this.enablementValue = enablement;
    this.ensurePersonBelongsToTheUsersTenant();
    this.recorder.raise(new UserEnablementChanged(this.tenant, this.name, enablement, this.clock()));
  }

  changePersonalName(name: FullName): void {
    this.personValue = this.personValue.withName(name);
    this.ensurePersonBelongsToTheUsersTenant();
  }

  /**
   * Enforces the aggregate invariant person-belongs-to-the-users-tenant:
   * the person record a user carries belongs to the user's own tenant.
   */
  ensurePersonBelongsToTheUsersTenant(): void {
    if (!this.personValue.tenantId.equals(this.tenant)) {
      throw new Error(`user ${this.name.toString()}: the person belongs to another tenant`);
    }
  }

  /** Checks the assertion changed-password-differs-from-current on changePassword. */
  assertChangedPasswordDiffersFromCurrent(current: Password, changed: Password): void {
    if (current.equals(changed)) {
      throw new Error(`user ${this.name.toString()}: new password must differ from the current one`);
    }
  }
}
