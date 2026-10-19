import { Password } from "../user/index.js";

const LETTERS = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ";
const DIGITS = "23456789";
const SYMBOLS = "!@#$%^&*-_=+?";
const SIZE = 16;

/**
 * The port through which the PasswordGenerator draws its choices: nextInt
 * returns a uniformly random integer in [0, bound). Infrastructure
 * implements it with the operating system's random source.
 */
export interface RandomSource {
  nextInt(bound: number): number;
}

/**
 * The domain service that proposes a strong password for a user who has
 * not chosen one. It decides what the password is made of; the
 * RandomSource it is given makes the choices.
 */
export class PasswordGenerator {
  constructor(private readonly random: RandomSource) {}

  /**
   * Proposes a 16 character password: one letter, one digit, and one
   * symbol at distinct random positions, the rest drawn from all three
   * classes. Assertion not-weak: the password it returns is never weak.
   */
  generateStrongPassword(): Password {
    const all = LETTERS + DIGITS + SYMBOLS;
    const chars = Array.from({ length: SIZE }, () => this.pick(all));
    const positions = new Set<number>();
    while (positions.size < 3) {
      positions.add(this.draw(SIZE));
    }
    const classes = [LETTERS, DIGITS, SYMBOLS];
    [...positions].forEach((position, i) => {
      chars[position] = this.pick(classes[i] ?? all);
    });
    const password = Password.from(chars.join(""));
    this.assertNotWeak(password);
    return password;
  }

  /** Checks not-weak on generateStrongPassword, through the Password strength rule. */
  assertNotWeak(password: Password): void {
    Password.from(password.reveal());
  }

  private pick(alphabet: string): string {
    return alphabet.charAt(this.draw(alphabet.length));
  }

  private draw(bound: number): number {
    const n = this.random.nextInt(bound);
    if (!Number.isInteger(n) || n < 0 || n >= bound) {
      throw new Error(`random source returned ${n} outside [0, ${bound})`);
    }
    return n;
  }
}
