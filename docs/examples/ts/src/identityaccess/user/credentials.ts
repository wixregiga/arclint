import type { Password } from "./password.js";
import type { Username } from "./username.js";

/** A username paired with the plain password chosen for it. */
export class Credentials {
  private constructor(readonly username: Username, readonly password: Password) {}

  /** The one door: invariant password-differs-from-username. */
  static of(username: Username, password: Password): Credentials {
    if (password.reveal() === username.toString()) {
      throw new Error("password must not equal the username");
    }
    return new Credentials(username, password);
  }
}
