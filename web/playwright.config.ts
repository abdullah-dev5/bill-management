import { defineConfig, devices } from '@playwright/test'

/**
 * End-to-end tests run the real stack: Go API (with Postgres) + Vite dev server.
 * Locally, servers that are already running are reused.
 * E2E_DATABASE_URL selects the database (default: local bill_dev).
 */
const databaseURL = process.env.E2E_DATABASE_URL ?? 'postgres://postgres:postgres@localhost:5432/bill_dev?sslmode=disable'

export default defineConfig({
  testDir: './e2e',
  outputDir: './e2e/.results',
  timeout: 60_000,
  fullyParallel: false,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'retain-on-failure',
  },
  projects: [
    { name: 'mobile', use: { ...devices['Pixel 7'] } },
    { name: 'desktop', use: { ...devices['Desktop Chrome'], viewport: { width: 1366, height: 900 } } },
  ],
  webServer: [
    {
      command: 'go run ./cmd/app serve',
      cwd: '../backend',
      url: 'http://localhost:8080/healthz',
      reuseExistingServer: true,
      env: { DATABASE_URL: databaseURL, TICK_TOKEN: 'e2e-tick-token-0123456789abcdef0123' },
      timeout: 120_000,
    },
    {
      command: 'pnpm exec vite --port 5173 --strictPort',
      url: 'http://localhost:5173',
      reuseExistingServer: true,
      timeout: 60_000,
    },
  ],
})
