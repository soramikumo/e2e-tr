import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  retries: 1,
  reporter: [['html', { open: 'never' }], ['line']],
  use: {
    // PLAYWRIGHT_BASE_URL があれば優先(runner が実行時に注入し dev/prod を切替)。
    // 相対 goto の spec はこの baseURL に解決される。録画 spec は自身の test.use で
    // 録画元 origin を既定に持つため、env 未指定でも元の環境を叩く。
    baseURL: process.env.PLAYWRIGHT_BASE_URL ?? 'https://example.com',
    trace: 'on-first-retry',
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
