import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  testMatch: '**/*.spec.ts',
  fullyParallel: false,
  workers: 1,
  timeout: 30_000,
  expect: { timeout: 10_000 },
  use: {
    baseURL: 'http://127.0.0.1:5174',
    viewport: { width: 1440, height: 900 },
    headless: true,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    launchOptions: { args: ['--use-angle=swiftshader', '--enable-unsafe-swiftshader'] },
  },
  webServer: {
    command: 'npm run build && npm run preview -- --port 5174',
    url: 'http://127.0.0.1:5174',
    reuseExistingServer: false,
    timeout: 60_000,
  },
});
