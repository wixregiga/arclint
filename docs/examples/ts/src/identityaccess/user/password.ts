/** A plain-text password on its way into the model; nothing stores it. */
export class Password {
  private constructor(private readonly value: string) {}

  /** The one door: invariant not-weak, at least 8 characters with a letter, a digit, and a symbol. */
  static from(text: string): Password {
    const chars = [...text];
    if (chars.length < 8) {
      throw new Error("password must be at least 8 characters and mix letters, digits, and symbols");
    }
    const letter = /\p{L}/u.test(text);
    const digit = /\p{Nd}/u.test(text);
    const symbol = /[\p{P}\p{S}]/u.test(text);
    if (!letter || !digit || !symbol) {
      throw new Error("password must be at least 8 characters and mix letters, digits, and symbols");
    }
    return new Password(text);
  }

  /** Hands the plain text to an Encrypter; it is the only reader. */
  reveal(): string {
    return this.value;
  }

  equals(other: Password): boolean {
    return this.value === other.value;
  }
}

/** The stored form of a password; only an Encrypter produces or verifies one. */
export class EncryptedPassword {
  private constructor(private readonly value: string) {}

  /** The one door: invariant not-empty. */
  static from(text: string): EncryptedPassword {
    if (text === "") {
      throw new Error("encrypted password must not be empty");
    }
    return new EncryptedPassword(text);
  }

  toString(): string {
    return this.value;
  }
}

/**
 * The port through which the Encrypter derives the stored text of a
 * password and compares a password with stored text. Infrastructure
 * implements it with a key derivation function.
 */
export interface KeyDerivation {
  derive(plain: string): string;
  matches(plain: string, stored: string): boolean;
}

/**
 * The domain service that turns a Password into its stored form and
 * verifies a Password against one. It decides what a stored password is;
 * the KeyDerivation it is given does the cryptography.
 */
export class Encrypter {
  constructor(private readonly derivation: KeyDerivation) {}

  /** Derives the stored form. Assertion not-empty: the stored form it returns is never empty. */
  encrypt(password: Password): EncryptedPassword {
    const encrypted = EncryptedPassword.from(this.derivation.derive(password.reveal()));
    this.assertNotEmpty(encrypted);
    return encrypted;
  }

  /** Checks not-empty on encrypt. */
  assertNotEmpty(encrypted: EncryptedPassword): void {
    if (encrypted.toString() === "") {
      throw new Error("encrypted password must not be empty");
    }
  }

  /** Whether the password is the one the stored form protects. */
  matches(password: Password, stored: EncryptedPassword): boolean {
    return this.derivation.matches(password.reveal(), stored.toString());
  }
}
