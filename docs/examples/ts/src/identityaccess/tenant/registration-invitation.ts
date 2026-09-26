import { randomBytes } from "node:crypto";

/**
 * A member entity of the Tenant aggregate: a tenant's standing offer
 * under which a user may register. Local to its tenant.
 */
export class RegistrationInvitation {
  private constructor(
    readonly invitationId: string,
    readonly description: string,
    readonly startingOn: Date | undefined,
    readonly until: Date | undefined,
  ) {}

  /** Opens an open-ended invitation; only Tenant calls this. */
  static offer(description: string): RegistrationInvitation {
    const trimmed = description.trim();
    if (trimmed === "") {
      throw new Error("invitation description must not be blank");
    }
    return new RegistrationInvitation(randomBytes(8).toString("hex"), trimmed, undefined, undefined);
  }

  /** Rebuilds an invitation from persisted state. */
  static reconstitute(invitationId: string, description: string, startingOn?: Date, until?: Date): RegistrationInvitation {
    return new RegistrationInvitation(invitationId, description, startingOn, until);
  }

  /** Whether the invitation is open at the instant. */
  isAvailable(at: Date): boolean {
    if (this.startingOn === undefined || this.until === undefined) {
      return true;
    }
    return at >= this.startingOn && at <= this.until;
  }

  isIdentifiedBy(identifier: string): boolean {
    return this.invitationId === identifier || this.description === identifier;
  }

  withWindow(startingOn: Date, until: Date): RegistrationInvitation {
    if (until < startingOn) {
      throw new Error("invitation must not end before it starts");
    }
    return new RegistrationInvitation(this.invitationId, this.description, startingOn, until);
  }
}
