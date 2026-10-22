import { defineConfig, devices } from '@playwright/test'

// End-to-end tests run against a running app: the real image in CI, or any
// local server via BASE_URL (see the README). They log in as an account that
// must exist on that server: E2E_USERNAME / E2E_PASSWORD (see tests/account.ts).
export default defineConfig({
  testDir: './tests',
  fullyParallel: true, // tests create uniquely named recipes, so they don't collide
  forbidOnly: !!process.env.CI,
  retries: 0, // a flaky test should fail loudly, not pass on a retry
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : [['list']],
  use: {
    baseURL: process.env.BASE_URL ?? 'http://localhost:8080',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    // Logs in once; the other projects start from that session.
    { name: 'login', testMatch: /login\.setup\.ts/ },
    // The app is meant for phones first.
    {
      name: 'android',
      use: { ...devices['Pixel 8'], storageState: '.auth/session.json' },
      dependencies: ['login'],
    },
    {
      name: 'iphone',
      use: { ...devices['iPhone 15'], storageState: '.auth/session.json' },
      dependencies: ['login'],
    },
  ],
})
