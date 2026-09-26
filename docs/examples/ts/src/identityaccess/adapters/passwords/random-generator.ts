import { randomInt } from "node:crypto";
import type { PasswordGenerator } from "../../provisioning/index.js";
import { Password } from "../../user/index.js";

const LETTERS = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ";
const DIGITS = "23456789";
const SYMBOLS = "!@#$%^&*-_=+?";
const SIZE = 16;

/**
 * Proposes passwords from the operating system's random source. Every
 * password satisfies the Password strength rule by construction.
 */
export class RandomPasswordGenerator implements PasswordGenerator {
  generateStrongPassword(): Password {
    const all = LETTERS + DIGITS + SYMBOLS;
    const chars = Array.from({ length: SIZE }, () => pick(all));
    const positions = new Set<number>();
    while (positions.size < 3) {
      positions.add(randomInt(SIZE));
    }
    const classes = [LETTERS, DIGITS, SYMBOLS];
    [...positions].forEach((position, i) => {
      chars[position] = pick(classes[i] ?? all);
    });
    return Password.from(chars.join(""));
  }
}

function pick(alphabet: string): string {
  return alphabet.charAt(randomInt(alphabet.length));
}
