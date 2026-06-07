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
    // Environment 経由で basic auth を渡すと runner が env として注入する。
    // ユーザー名のみ指定で空 PW も許容(httpCredentials は両方文字列でOK)。
    httpCredentials: process.env.PLAYWRIGHT_HTTP_AUTH_USER
      ? {
          username: process.env.PLAYWRIGHT_HTTP_AUTH_USER,
          password: process.env.PLAYWRIGHT_HTTP_AUTH_PASS ?? '',
        }
      : undefined,
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
