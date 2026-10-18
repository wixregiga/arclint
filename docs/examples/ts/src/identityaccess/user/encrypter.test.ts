import assert from "node:assert/strict";
import { test } from "node:test";
import { ENCRYPTED_PASSWORD_EMPTY, EncryptedPassword, Encrypter, Password, type ProtectionScheme } from "./index.js";

/** A protection scheme that yields an empty protected form, the case the not-empty assertion refuses. */
class EmptyScheme implements ProtectionScheme {
  protect(): string {
    return "";
  }

  matches(): boolean {
    return true;
  }
}

/** A protection scheme whose derivation fails. */
class FailingScheme implements ProtectionScheme {
  protect(): string {
    throw new Error("derivation failed");
  }

  matches(): boolean {
    return false;
  }
}

test("the encrypter refuses an empty protected form", () => {
  assert.throws(() => new Encrypter(new EmptyScheme()).encrypt(Password.from("correct-H0rse-battery")), { message: ENCRYPTED_PASSWORD_EMPTY });
});

test("the encrypter fails with its scheme", () => {
  assert.throws(() => new Encrypter(new FailingScheme()).encrypt(Password.from("correct-H0rse-battery")), { message: "derivation failed" });
});

test("the not-empty check refuses an encrypted password whose form is empty", () => {
  const empty = Object.create(EncryptedPassword.prototype) as EncryptedPassword;
  Object.defineProperty(empty, "value", { value: "" });
  assert.throws(() => new Encrypter(new EmptyScheme()).assertNotEmpty(empty), { message: ENCRYPTED_PASSWORD_EMPTY });
});
