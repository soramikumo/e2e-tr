import { test, expect } from '@playwright/test';

test('@search トップページが表示される', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveTitle(/Example Domain/);
});

test('@search ページにメインの見出しが存在する', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Example Domain' })).toBeVisible();
});

test('@search ページにテキストが存在する', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByText('This domain is for use in illustrative examples')).toBeVisible();
});
