import { EventRecorder, type DomainEvent } from "../event/index.js";
import { TenantActivated, TenantDeactivated, TenantProvisioned } from "./events.js";
import { RegistrationInvitation } from "./registration-invitation.js";
import type { TenantID } from "./tenant-id.js";
import type { TenantName } from "./tenant-name.js";

export type Clock = () => Date;

/**
 * The Tenant aggregate root: the subscribing organisation under which
 * users and roles exist. It owns its registration invitations.
 */
export class Tenant {
  private readonly recorder = new EventRecorder();

  private constructor(
    private readonly id: TenantID,
    private readonly tenantName: TenantName,
    private readonly tenantDescription: string,
    private active: boolean,
    private invitations: RegistrationInvitation[],
    private readonly clock: Clock,
  ) {
    this.ensureOneOpenInvitationPerDescription();
  }

  /** The constructor door: a tenant comes into existence active. */
  static provision(id: TenantID, name: TenantName, description: string, clock: Clock): Tenant {
    const tenant = new Tenant(id, name, description, true, [], clock);
    tenant.recorder.raise(new TenantProvisioned(id, name, clock()));
    return tenant;
  }

  /** Rebuilds a tenant from persisted state; a repository adapter is its only caller. */
  static reconstitute(id: TenantID, name: TenantName, description: string, active: boolean, invitations: RegistrationInvitation[], clock: Clock): Tenant {
    return new Tenant(id, name, description, active, [...invitations], clock);
  }

  get tenantId(): TenantID {
    return this.id;
  }

  get name(): TenantName {
    return this.tenantName;
  }

  get description(): string {
    return this.tenantDescription;
  }

  isActive(): boolean {
    return this.active;
  }

  registrationInvitations(): readonly RegistrationInvitation[] {
    return [...this.invitations];
  }

  drainEvents(): DomainEvent[] {
    return this.recorder.drain();
  }

  activate(): void {
    if (this.active) {
      throw new Error("tenant is already active");
    }
    this.active = true;
    this.ensureOneOpenInvitationPerDescription();
    this.recorder.raise(new TenantActivated(this.id, this.clock()));
  }

  // ANCHOR: command-ensure
  deactivate(): void {
    if (!this.active) {
      throw new Error("tenant is already inactive");
    }
    this.active = false;
    this.ensureOneOpenInvitationPerDescription();
    this.recorder.raise(new TenantDeactivated(this.id, this.clock()));
  }
  // ANCHOR_END: command-ensure

  // ANCHOR: command-assert
  /** Opens a new open-ended invitation under the description. */
  offerRegistrationInvitation(description: string): RegistrationInvitation {
    this.assertInvitesOnlyWhileActive();
    const invitation = RegistrationInvitation.offer(description);
    if (this.isRegistrationAvailableThrough(invitation.description)) {
      throw new Error("an available invitation with that description already exists");
    }
    this.invitations.push(invitation);
    this.ensureOneOpenInvitationPerDescription();
    return invitation;
  }
  // ANCHOR_END: command-assert

  /** Closes the window of an offered invitation to the period. */
  limitInvitation(identifier: string, startingOn: Date, until: Date): void {
    this.assertInvitesOnlyWhileActive();
    const index = this.invitationIndex(identifier);
    if (index < 0) {
      throw new Error("no registration invitation answers to that identifier");
    }
    const current = this.invitations[index];
    if (current === undefined) {
      throw new Error("no registration invitation answers to that identifier");
    }
    this.invitations[index] = current.withWindow(startingOn, until);
    this.ensureOneOpenInvitationPerDescription();
  }

  withdrawInvitation(identifier: string): void {
    const index = this.invitationIndex(identifier);
    if (index < 0) {
      throw new Error("no registration invitation answers to that identifier");
    }
    this.invitations.splice(index, 1);
    this.ensureOneOpenInvitationPerDescription();
  }

  // ANCHOR: query
  /** Side-effect-free: whether a user may register through the identified invitation now. */
  isRegistrationAvailableThrough(identifier: string): boolean {
    const invitation = this.invitations.find((i) => i.isIdentifiedBy(identifier));
    return invitation !== undefined && invitation.isAvailable(this.clock());
  }
  // ANCHOR_END: query

  // ANCHOR: ensure
  /**
   * Enforces the aggregate invariant one-open-invitation-per-description:
   * a tenant holds at most one available registration invitation per description.
   */
  ensureOneOpenInvitationPerDescription(): void {
    const now = this.clock();
    const seen = new Set<string>();
    for (const invitation of this.invitations) {
      if (!invitation.isAvailable(now)) {
        continue;
      }
      if (seen.has(invitation.description)) {
        throw new Error(`tenant ${this.id.toString()}: two available invitations share the description "${invitation.description}"`);
      }
      seen.add(invitation.description);
    }
  }
  // ANCHOR_END: ensure

  // ANCHOR: assert
  /**
   * Checks the assertion invites-only-while-active on offerRegistrationInvitation
   * and limitInvitation: an invitation is offered or limited only by an active tenant.
   */
  assertInvitesOnlyWhileActive(): void {
    if (!this.active) {
      throw new Error(`tenant ${this.id.toString()} is not active`);
    }
  }
  // ANCHOR_END: assert

  private invitationIndex(identifier: string): number {
    return this.invitations.findIndex((i) => i.isIdentifiedBy(identifier));
  }
}
