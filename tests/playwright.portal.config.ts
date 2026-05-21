import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './portal-tests',
  fullyParallel: false,
  retries: 1,
  reporter: [['html', { open: 'never', outputFolder: 'playwright-report' }], ['line']],
  use: {
    baseURL: process.env.PORTAL_URL ?? 'http://localhost:3000',
    trace: 'on',
    screenshot: 'only-on-failure',
    video: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
