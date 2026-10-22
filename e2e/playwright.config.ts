import { defineConfig, devices } from '@playwright/test'

// End-to-end tests run against a running app: the real image in CI, or any
// local server via BASE_URL (see README.md in this folder).
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
  // The app is meant for phones first.
  projects: [
    { name: 'android', use: { ...devices['Pixel 8'] } },
    { name: 'iphone', use: { ...devices['iPhone 15'] } },
  ],
})
