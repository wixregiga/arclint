import { pbkdf2Sync, randomBytes, timingSafeEqual } from "node:crypto";
import { EncryptedPassword, type Encrypter, type Password } from "../../user/index.js";

const ITERATIONS = 600_000;
const KEY_LENGTH = 32;
const SALT_LENGTH = 16;
const SCHEME = "pbkdf2-sha256";

/** Encrypter with PBKDF2-HMAC-SHA256; stored form "pbkdf2-sha256$<iterations>$<salt>$<key>". */
export class Pbkdf2Encrypter implements Encrypter {
  encrypt(password: Password): EncryptedPassword {
    const salt = randomBytes(SALT_LENGTH);
    const key = pbkdf2Sync(password.reveal(), salt, ITERATIONS, KEY_LENGTH, "sha256");
    return EncryptedPassword.from([SCHEME, String(ITERATIONS), salt.toString("base64"), key.toString("base64")].join("$"));
  }

  matches(password: Password, stored: EncryptedPassword): boolean {
    const parts = stored.toString().split("$");
    const [scheme, iterationsText, saltText, keyText] = parts;
    if (parts.length !== 4 || scheme !== SCHEME || iterationsText === undefined || saltText === undefined || keyText === undefined) {
      return false;
    }
    const iterations = Number.parseInt(iterationsText, 10);
    if (!Number.isInteger(iterations) || iterations <= 0) {
      return false;
    }
    const expected = Buffer.from(keyText, "base64");
    const key = pbkdf2Sync(password.reveal(), Buffer.from(saltText, "base64"), iterations, expected.length, "sha256");
    return key.length === expected.length && timingSafeEqual(key, expected);
  }
}
