import assert from "node:assert/strict";
import { test } from "node:test";
import { PasswordGenerator } from "../../provisioning/index.js";
import { Encrypter, Password } from "../../user/index.js";
import { Pbkdf2KeyDerivation, RandomCandidates } from "./index.js";

test("PBKDF2 round trip through the Encrypter", () => {
  const encrypter = new Encrypter(new Pbkdf2KeyDerivation());
  const password = Password.from("s3cret-Pass");
  const first = encrypter.encrypt(password);
  const second = encrypter.encrypt(password);
  assert.match(first.toString(), /^pbkdf2-sha256\$600000\$/);
  assert.notEqual(first.toString(), second.toString(), "two encryptions share a salt");
  assert.ok(encrypter.matches(password, first));
  assert.ok(encrypter.matches(password, second));
  assert.ok(!encrypter.matches(Password.from("an0ther-Pass"), first));
  assert.ok(!new Pbkdf2KeyDerivation().verify("s3cret-Pass", "md5$abc"));
});

test("random candidates are strong and distinct", () => {
  const generator = new PasswordGenerator(new RandomCandidates());
  const seen = new Set<string>();
  for (let i = 0; i < 200; i++) {
    const proposed = generator.generateStrongPassword().reveal();
    assert.equal(proposed.length, 16);
    assert.ok(!seen.has(proposed), `${proposed} proposed twice`);
    seen.add(proposed);
  }
});
