import assert from "node:assert/strict";
import { test } from "node:test";
import { EncryptedPassword, Encrypter, Password, type KeyDerivation } from "./index.js";

/** A KeyDerivation that answers with fixed text. */
function derivationOf(text: string): KeyDerivation {
  return { derive: () => text, matches: (_plain, stored) => stored === text };
}

test("encrypt stores what the derivation produces", () => {
  const encrypter = new Encrypter(derivationOf("derived"));
  const password = Password.from("c0rrect-Horse");
  const stored = encrypter.encrypt(password);
  assert.equal(stored.toString(), "derived");
  assert.equal(encrypter.matches(password, stored), true);
});

test("encrypt refuses an empty stored form", () => {
  assert.throws(() => new Encrypter(derivationOf("")).encrypt(Password.from("c0rrect-Horse")), /must not be empty/);
});

test("assertNotEmpty accepts a stored form with text", () => {
  assert.doesNotThrow(() => new Encrypter(derivationOf("stored")).assertNotEmpty(EncryptedPassword.from("stored")));
});
