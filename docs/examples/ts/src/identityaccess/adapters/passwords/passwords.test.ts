import assert from "node:assert/strict";
import { test } from "node:test";
import { PasswordGenerator } from "../../provisioning/index.js";
import { Encrypter, Password } from "../../user/index.js";
import { Pbkdf2Scheme, RandomDrafter } from "./index.js";

test("the encrypter protects and matches through PBKDF2", () => {
  const encrypter = new Encrypter(new Pbkdf2Scheme());
  const chosen = Password.from("correct-H0rse-battery");
  const encrypted = encrypter.encrypt(chosen);
  assert.notEqual(encrypted.toString(), "");
  assert.notEqual(encrypted.toString(), chosen.reveal());
  assert.equal(encrypter.matches(chosen, encrypted), true);
  assert.equal(encrypter.matches(Password.from("other-H0rse-battery"), encrypted), false);
  assert.notEqual(encrypter.encrypt(chosen).toString(), encrypted.toString(), "each encryption must use a fresh salt");
});

test("the generator proposes distinct strong passwords", () => {
  const generator = new PasswordGenerator(new RandomDrafter());
  const seen = new Set<string>();
  for (let i = 0; i < 200; i++) {
    const proposed = generator.generateStrongPassword();
    assert.equal(proposed.isWeak(), false);
    assert.equal(proposed.reveal().length, 16);
    seen.add(proposed.reveal());
  }
  assert.equal(seen.size, 200);
});
