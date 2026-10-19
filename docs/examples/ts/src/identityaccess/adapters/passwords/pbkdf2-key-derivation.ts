import { pbkdf2Sync, randomBytes, timingSafeEqual } from "node:crypto";
import type { KeyDerivation } from "../../user/index.js";

const ITERATIONS = 600_000;
const KEY_LENGTH = 32;
const SALT_LENGTH = 16;
const SCHEME = "pbkdf2-sha256";

/** KeyDerivation with PBKDF2-HMAC-SHA256; stored text "pbkdf2-sha256$<iterations>$<salt>$<key>". */
export class Pbkdf2KeyDerivation implements KeyDerivation {
  derive(plain: string): string {
    const salt = randomBytes(SALT_LENGTH);
    const key = pbkdf2Sync(plain, salt, ITERATIONS, KEY_LENGTH, "sha256");
    return [SCHEME, String(ITERATIONS), salt.toString("base64"), key.toString("base64")].join("$");
  }

  matches(plain: string, stored: string): boolean {
    const parts = stored.split("$");
    const [scheme, iterationsText, saltText, keyText] = parts;
    if (parts.length !== 4 || scheme !== SCHEME || iterationsText === undefined || saltText === undefined || keyText === undefined) {
      return false;
    }
    const iterations = Number.parseInt(iterationsText, 10);
    if (!Number.isInteger(iterations) || iterations <= 0) {
      return false;
    }
    const expected = Buffer.from(keyText, "base64");
    if (expected.length === 0) {
      return false;
    }
    const key = pbkdf2Sync(plain, Buffer.from(saltText, "base64"), iterations, expected.length, "sha256");
    return key.length === expected.length && timingSafeEqual(key, expected);
  }
}
