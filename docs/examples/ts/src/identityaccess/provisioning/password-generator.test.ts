import assert from "node:assert/strict";
import { test } from "node:test";
import { Password, PASSWORD_WEAK } from "../user/index.js";
import { PasswordGenerator, PROPOSED_PASSWORD_WEAK, type Drafter } from "./index.js";

/** A drafter that drafts the same text every time. */
class FixedDrafter implements Drafter {
  constructor(private readonly text: string) {}

  draft(): string {
    return this.text;
  }
}

test("the generator refuses a weak draft", () => {
  assert.throws(() => new PasswordGenerator(new FixedDrafter("short")).generateStrongPassword(), { message: PASSWORD_WEAK });
});

test("the generator proposes a strong draft", () => {
  assert.equal(new PasswordGenerator(new FixedDrafter("k7#Rw9!qPz2$")).generateStrongPassword().reveal(), "k7#Rw9!qPz2$");
});

test("the not-weak check refuses a password that fails the strength rule", () => {
  const weak = Object.create(Password.prototype) as Password;
  Object.defineProperty(weak, "value", { value: "" });
  assert.throws(() => new PasswordGenerator(new FixedDrafter("k7#Rw9!qPz2$")).assertNotWeak(weak), { message: PROPOSED_PASSWORD_WEAK });
});
