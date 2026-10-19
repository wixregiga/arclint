import assert from "node:assert/strict";
import { test } from "node:test";
import { PasswordGenerator } from "./index.js";

test("generateStrongPassword proposes a strong candidate", () => {
  const proposed = new PasswordGenerator({ next: () => "s3cret-Pass" }).generateStrongPassword();
  assert.equal(proposed.reveal(), "s3cret-Pass");
});

test("generateStrongPassword refuses a weak candidate", () => {
  for (const weak of ["", "short1!", "nodigits!!", "n0symbols1"]) {
    assert.throws(() => new PasswordGenerator({ next: () => weak }).generateStrongPassword(), /at least 8 characters/, weak);
  }
});

test("generateStrongPassword reports failing candidates", () => {
  assert.throws(() => new PasswordGenerator({ next: () => { throw new Error("no entropy"); } }).generateStrongPassword(), /no entropy/);
});
