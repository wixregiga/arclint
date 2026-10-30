import { execFileSync } from "node:child_process";
import { existsSync } from "node:fs";
import path from "node:path";

/** The repository root: web/ sits directly under it. */
export const REPO_ROOT = path.resolve(import.meta.dirname, "../../..");

/**
 * The arclint binary the browser suite runs: ARCLINT_BIN when set, else
 * one built from this checkout into the scratch folder, so the suite
 * always exercises the Go server as it is here.
 */
export function arclintBinary(scratch: string): string {
  const given = process.env.ARCLINT_BIN;
  if (given && existsSync(given)) return given;
  const out = path.join(scratch, "arclint");
  execFileSync("go", ["build", "-o", out, "./cmd/arclint"], { cwd: REPO_ROOT, stdio: "inherit" });
  return out;
}
