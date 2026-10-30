/** Build Studio in scratch space, then write or compare the binary's assets. */
import { execFileSync } from "node:child_process";
import { cpSync, existsSync, mkdtempSync, readdirSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

const web = path.resolve(import.meta.dirname, "..");
const embedded = path.resolve(web, "../internal/delivery/mapserve/static/web");
const scratch = mkdtempSync(path.join(tmpdir(), "arclint-studio-build-"));
const check = process.argv.includes("--check");

function files(dir: string, prefix = ""): string[] {
  if (!existsSync(dir)) return [];
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const name = path.join(prefix, entry.name);
    return entry.isDirectory() ? files(path.join(dir, entry.name), name) : [name];
  }).sort();
}

try {
  execFileSync("npm", ["run", "build", "--", "--outDir", scratch], { cwd: web, stdio: "inherit" });
  if (check) {
    const built = files(scratch);
    const committed = files(embedded);
    if (JSON.stringify(built) !== JSON.stringify(committed) || built.some((name) => !readFileSync(path.join(scratch, name)).equals(readFileSync(path.join(embedded, name))))) {
      throw new Error("Embedded Studio differs from web source. Run make generate ONLY=web and commit the assets.");
    }
    console.log("Embedded Studio matches the current Web build.");
  } else {
    rmSync(embedded, { recursive: true, force: true });
    cpSync(scratch, embedded, { recursive: true });
    console.log("Updated embedded Studio assets.");
  }
} finally {
  rmSync(scratch, { recursive: true, force: true });
}
