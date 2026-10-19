import assert from "node:assert/strict";
import { test } from "node:test";
import { PasswordGenerator } from "../../provisioning/index.js";
import { Encrypter, Password } from "../../user/index.js";
import { Pbkdf2KeyDerivation, SystemRandom } from "./index.js";

test("PBKDF2 protects the password the Encrypter is given", () => {
  const encrypter = new Encrypter(new Pbkdf2KeyDerivation());
  const password = Password.from("c0rrect-Horse");
  const stored = encrypter.encrypt(password);
  assert.notEqual(stored.toString(), password.reveal());
  assert.equal(encrypter.matches(password, stored), true);
  assert.equal(encrypter.matches(Password.from("wr0ng-Horse"), stored), false);
  assert.notEqual(encrypter.encrypt(password).toString(), stored.toString());
});

test("PBKDF2 refuses a malformed stored text", () => {
  for (const stored of ["", "plain", "pbkdf2-sha256$x$c2FsdA$a2V5", "md5$1$c2FsdA$a2V5", "pbkdf2-sha256$1$c2FsdA$"]) {
    assert.equal(new Pbkdf2KeyDerivation().matches("c0rrect-Horse", stored), false, stored);
  }
});

test("the system random source generates strong passwords", () => {
  const generator = new PasswordGenerator(new SystemRandom());
  for (let i = 0; i < 200; i++) {
    assert.doesNotThrow(() => Password.from(generator.generateStrongPassword().reveal()));
  }
});
