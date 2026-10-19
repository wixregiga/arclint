import assert from "node:assert/strict";
import { randomInt } from "node:crypto";
import { test } from "node:test";
import { Password } from "../user/index.js";
import { PasswordGenerator, type RandomSource } from "./index.js";

const crypto: RandomSource = { nextInt: (bound) => randomInt(bound) };

test("generateStrongPassword proposes strong passwords", () => {
  const generator = new PasswordGenerator(crypto);
  for (let i = 0; i < 5000; i++) {
    const password = generator.generateStrongPassword();
    assert.equal(password.reveal().length, 16);
    assert.doesNotThrow(() => Password.from(password.reveal()));
  }
});

test("generateStrongPassword refuses a broken random source", () => {
  for (const source of [{ nextInt: (bound: number) => bound }, { nextInt: () => -1 }, { nextInt: () => 0.5 }]) {
    assert.throws(() => new PasswordGenerator(source).generateStrongPassword(), /outside/);
  }
});

test("assertNotWeak accepts a strong password", () => {
  assert.doesNotThrow(() => new PasswordGenerator(crypto).assertNotWeak(Password.from("c0rrect-Horse")));
});
