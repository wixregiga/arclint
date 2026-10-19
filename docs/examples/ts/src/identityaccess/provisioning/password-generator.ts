import { Password } from "../user/index.js";

/**
 * Supplies the text of the passwords a PasswordGenerator proposes. It is
 * the port through which the generator reaches unpredictable choices,
 * which the model cannot make; an adapter implements it.
 */
export interface Candidates {
  next(): string;
}

/**
 * The domain service that proposes a strong password for a user who has
 * not chosen one yet, from its Candidates.
 */
export class PasswordGenerator {
  constructor(private readonly candidates: Candidates) {}

  /** Contract (assertion on generateStrongPassword): not-weak. */
  generateStrongPassword(): Password {
    const proposed = Password.from(this.candidates.next());
    this.assertNotWeak(proposed);
    return proposed;
  }

  /** Checks the contract assertion not-weak on generateStrongPassword: a proposed password is never weak. */
  assertNotWeak(proposed: Password): void {
    Password.from(proposed.reveal());
  }
}
