import { randomInt } from "node:crypto";
import type { RandomSource } from "../../provisioning/index.js";

/** RandomSource with the operating system's cryptographic random source. */
export class SystemRandom implements RandomSource {
  nextInt(bound: number): number {
    return randomInt(bound);
  }
}
