import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright config for the hpay-admin E2E suite (e2e/), per
 * docs/plans/e2e-playwright-suite/implementation-plan.html §3.
 *
 * Runs full-stack: Chromium -> vite preview (:4173, proxying /v1 to the real
 * backend on :8080) -> Postgres/Redis. No mocking except the two deliberate
 * route-interception edge cases (A04, A05 interceptor).
 */
export default defineConfig({
  testDir: './e2e',
  globalSetup: './e2e/global-setup.ts',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  // Capped everywhere (not just CI): POST /v1/admin/auth/login is limited to
  // 5 req/min/IP server-side (hpay-backend router.go) and the auth fixture
  // only logs in once per worker process per credential — too many workers
  // still means too many first-time logins landing in the same window.
  workers: process.env.CI ? 2 : 3,
  reporter: process.env.CI
    ? [['html', { open: 'never' }], ['github'], ['junit', { outputFile: 'playwright-report/junit.xml' }]]
    : [['html', { open: 'never' }]],
  use: {
    baseURL: process.env.E2E_BASE_URL ?? 'http://localhost:4173',
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
