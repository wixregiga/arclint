/** The message of a password that fails the strength rule. */
export const PASSWORD_WEAK = "password must be at least 8 characters and mix letters, digits, and symbols";

/** The message of an encrypted password that is empty. */
export const ENCRYPTED_PASSWORD_EMPTY = "encrypted password must not be empty";

/** The strength rule: at least 8 characters holding a letter, a digit, and a symbol. */
function strong(text: string): boolean {
  return [...text].length >= 8 && /\p{L}/u.test(text) && /\p{Nd}/u.test(text) && /[\p{P}\p{S}]/u.test(text);
}

/** A plain-text password on its way into the model; nothing stores it. */
export class Password {
  private constructor(private readonly value: string) {}

  /** The one door: invariant not-weak, at least 8 characters with a letter, a digit, and a symbol. */
  static from(text: string): Password {
    if (!strong(text)) {
      throw new Error(PASSWORD_WEAK);
    }
    return new Password(text);
  }

  /** Whether the password fails the strength rule. */
  isWeak(): boolean {
    return !strong(this.value);
  }

  /** Hands the plain text to the ProtectionScheme; it is the only reader. */
  reveal(): string {
    return this.value;
  }

  equals(other: Password): boolean {
    return this.value === other.value;
  }
}

/** The stored form of a password; only the Encrypter produces or verifies one. */
export class EncryptedPassword {
  private constructor(private readonly value: string) {}

  /** The one door: invariant not-empty. */
  static from(text: string): EncryptedPassword {
    if (text === "") {
      throw new Error(ENCRYPTED_PASSWORD_EMPTY);
    }
    return new EncryptedPassword(text);
  }

  toString(): string {
    return this.value;
  }
}

/**
 * Derives the protected form of a Password and tells whether a Password
 * matches a protected form. It is the port through which the Encrypter
 * reaches the derivation, which needs randomness and a key derivation
 * function the model does not hold; an adapter implements it.
 */
export interface ProtectionScheme {
  protect(password: Password): string;
  matches(password: Password, protectedForm: string): boolean;
}

/**
 * The domain service that turns a Password into its stored form and
 * verifies a Password against one, through its ProtectionScheme.
 */
export class Encrypter {
  constructor(private readonly scheme: ProtectionScheme) {}

  /** Turns a Password into its stored form. Contract (assertion on encrypt): not-empty. */
  encrypt(password: Password): EncryptedPassword {
    const encrypted = EncryptedPassword.from(this.scheme.protect(password));
    this.assertNotEmpty(encrypted);
    return encrypted;
  }

  /** Tells whether the password is the one stored. */
  matches(password: Password, stored: EncryptedPassword): boolean {
    return this.scheme.matches(password, stored.toString());
  }

  /** Checks the contract assertion not-empty on encrypt: an encrypted password is never empty. */
  assertNotEmpty(encrypted: EncryptedPassword): void {
    if (encrypted.toString() === "") {
      throw new Error(ENCRYPTED_PASSWORD_EMPTY);
    }
  }
}
