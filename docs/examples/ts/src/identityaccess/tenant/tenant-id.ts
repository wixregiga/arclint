import { randomBytes } from "node:crypto";

const ALPHABET = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
const LENGTH = 26;

/** The globally unique identity of a Tenant; other aggregates refer to a tenant through it. */
export class TenantID {
  private constructor(private readonly value: string) {}

  /** Mints a fresh identity from the operating system's random source. */
  static next(): TenantID {
    const bytes = randomBytes(LENGTH);
    let text = "";
    for (const b of bytes) {
      text += ALPHABET[b % ALPHABET.length];
    }
    return new TenantID(text);
  }

  /** The one door through which a text becomes a TenantID. */
  static parse(text: string): TenantID {
    if (text.length !== LENGTH || [...text].some((c) => !ALPHABET.includes(c))) {
      throw new Error("tenant id must be 26 base32 characters");
    }
    return new TenantID(text);
  }

  equals(other: TenantID): boolean {
    return this.value === other.value;
  }

  toString(): string {
    return this.value;
  }
}
