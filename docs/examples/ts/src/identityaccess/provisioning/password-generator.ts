import { Password } from "../user/index.js";

/** The message of a proposed password that fails the strength rule. */
export const PROPOSED_PASSWORD_WEAK = "proposed password is weak";

/**
 * Drafts the text of a password to propose. It is the port through which
 * the PasswordGenerator reaches unpredictable choices, which the model
 * cannot make; an adapter implements it.
 */
export interface Drafter {
  draft(): string;
}

/**
 * The domain service that proposes a strong password for a user who has
 * not chosen one, from what its Drafter drafts.
 */
export class PasswordGenerator {
  constructor(private readonly drafter: Drafter) {}

  /** Proposes a strong password. Contract (assertion on generateStrongPassword): not-weak. */
  generateStrongPassword(): Password {
    const proposed = Password.from(this.drafter.draft());
    this.assertNotWeak(proposed);
    return proposed;
  }

  /** Checks the contract assertion not-weak on generateStrongPassword: a proposed password is never weak. */
  assertNotWeak(proposed: Password): void {
    if (proposed.isWeak()) {
      throw new Error(PROPOSED_PASSWORD_WEAK);
    }
  }
}
