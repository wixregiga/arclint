import path from "node:path";
import { realpathSync } from "node:fs";
import { tmpdir } from "node:os";
import { defineConfig, devices } from "@playwright/test";

// The browser suite runs the built Studio against real arclint map
// servers over copies of real recordings; tests/support/studio-host.ts
// builds the binary, prepares the projects and serves dist at several
// bases. Each agent running the suite picks its own port.
const PORT = Number(process.env.STUDIO_E2E_PORT ?? 4421);
const SCRATCH = path.join(realpathSync(tmpdir()), `arclint-studio-e2e-${PORT}`);

export default defineConfig({
  testDir: "tests/browser",
  fullyParallel: false,
  workers: 1,
  reporter: "list",
  timeout: 30_000,
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
    trace: "retain-on-failure",
  },
  projects: [{ name: "desktop", use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } } }],
});
