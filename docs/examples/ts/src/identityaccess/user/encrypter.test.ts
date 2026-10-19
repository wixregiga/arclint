import assert from "node:assert/strict";
import { test } from "node:test";
import { Encrypter, type KeyDerivation, Password } from "./index.js";

/** Derives a stored form by reversing the text, or fails; it stands in for a key derivation function. */
function reversing(derived?: () => string): KeyDerivation {
  const reverse = (plain: string) => [...plain].reverse().join("");
  return {
    derive: derived ?? reverse,
    verify: (plain, stored) => reverse(plain) === stored,
  };
}

test("encrypt stores what the key derivation derives", () => {
  const encrypter = new Encrypter(reversing());
  const password = Password.from("s3cret-Pass");
  const encrypted = encrypter.encrypt(password);
  assert.equal(encrypted.toString(), "ssaP-terc3s");
  assert.ok(encrypter.matches(password, encrypted));
  assert.ok(!encrypter.matches(Password.from("an0ther-Pass"), encrypted));
});

test("encrypt refuses an empty stored form", () => {
  assert.throws(() => new Encrypter(reversing(() => "")).encrypt(Password.from("s3cret-Pass")), /must not be empty/);
});

test("encrypt reports a failing key derivation", () => {
  assert.throws(() => new Encrypter(reversing(() => { throw new Error("no entropy"); })).encrypt(Password.from("s3cret-Pass")), /no entropy/);
});
