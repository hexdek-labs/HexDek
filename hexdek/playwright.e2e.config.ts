import { defineConfig } from '@playwright/test'

// E2E smoke suite (r60) — runs against a locally-launched Vite dev server.
// Tests live in ./e2e/, screenshots in ./e2e/screenshots/.
//
// Usage:
//   cd hexdek && npx playwright test --config=playwright.e2e.config.ts
//
// The dev server is auto-started + torn down by Playwright. VITE_API_URL is
// pointed at the production backend so screens can populate real data;
// override with HEXDEK_API_URL=... if you want to hit a different backend.
const API_URL = process.env.HEXDEK_API_URL || 'https://hexdek.dev'
const DEV_PORT = 5180

export default defineConfig({
  testDir: './e2e',
  outputDir: './e2e/test-results',
  reporter: [['list'], ['html', { outputFolder: './e2e/report', open: 'never' }]],
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 60_000,
  expect: { timeout: 15_000 },
  use: {
    baseURL: `http://localhost:${DEV_PORT}`,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    ignoreHTTPSErrors: true,
  },
  projects: [
    {
      name: 'desktop',
      use: { viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1 },
    },
    {
      name: 'mobile',
      // iPhone 14 viewport (390x844). Use explicit viewport instead of the
      // device preset so the mobile project doesn't pull in a webkit-only
      // engine — chromium is fine for smoke shape.
      use: {
        viewport: { width: 390, height: 844 },
        deviceScaleFactor: 3,
        isMobile: true,
        hasTouch: true,
        userAgent:
          'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1',
      },
    },
  ],
  webServer: {
    command: `VITE_API_URL=${API_URL} npx vite --port ${DEV_PORT} --strictPort`,
    url: `http://localhost:${DEV_PORT}`,
    reuseExistingServer: !process.env.CI,
    // 120s: a cold `npx vite` start (no warm node_modules/.vite cache,
    // CI runner or busy laptop) was observed blowing the previous 60s
    // budget and failing the whole run before any test executed.
    timeout: 120_000,
    stdout: 'pipe',
    stderr: 'pipe',
  },
})
