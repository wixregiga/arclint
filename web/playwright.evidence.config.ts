import path from "node:path";
import { realpathSync } from "node:fs";
import { tmpdir } from "node:os";
import { defineConfig, devices } from "@playwright/test";

// The pull request's screenshot and video capture: the same host the
// browser suite runs (tests/support/serve-studio.ts), over the same real
// recordings, driven by tests/evidence. ARCLINT_BIN picks the binary, so
// the capture runs once against main's binary (before) and once against
// this checkout's (after). EVIDENCE_DIR receives the files; point it
// outside the repository or at a git-ignored folder such as .tmp/.
const PORT = Number(process.env.STUDIO_E2E_PORT ?? 4431);
const SCRATCH = path.join(realpathSync(tmpdir()), `arclint-studio-evidence-${PORT}`);

export default defineConfig({
  testDir: "tests/evidence",
  fullyParallel: false,
  workers: 1,
  reporter: "list",
  timeout: 120_000,
  outputDir: path.join(process.env.EVIDENCE_DIR ?? path.join(SCRATCH, "evidence"), "playwright"),
  webServer: {
    command: "node tests/support/serve-studio.ts",
    url: `http://localhost:${PORT}/healthz`,
    reuseExistingServer: false,
    timeout: 180_000,
    env: { STUDIO_E2E_PORT: String(PORT), STUDIO_E2E_SCRATCH: SCRATCH },
    gracefulShutdown: { signal: "SIGTERM", timeout: 10_000 },
  },
  use: {
    baseURL: `http://localhost:${PORT}`,
  },
  projects: [{ name: "desktop", use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } } }],
});
