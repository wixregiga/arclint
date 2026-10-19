import { randomInt } from "node:crypto";
import type { Candidates } from "../../provisioning/index.js";

const LETTERS = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ";
const DIGITS = "23456789";
const SYMBOLS = "!@#$%^&*-_=+?";
const SIZE = 16;

/**
 * Candidates from the operating system's random source. Every candidate
 * holds a letter, a digit, and a symbol: one character of each class is
 * placed at a random position and the rest is drawn from all.
 */
export class RandomCandidates implements Candidates {
  next(): string {
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
    return chars.join("");
  }
}

function pick(alphabet: string): string {
  return alphabet.charAt(randomInt(alphabet.length));
}
