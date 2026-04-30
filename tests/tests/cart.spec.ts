import { test, expect } from '@playwright/test';

test('@cart ページにリンクが存在する', async ({ page }) => {
  await page.goto('/');
  const link = page.getByRole('link', { name: 'More information...' });
  await expect(link).toBeVisible();
});

test('@cart リンクが正しいURLを持つ', async ({ page }) => {
  await page.goto('/');
  const link = page.getByRole('link', { name: 'More information...' });
  await expect(link).toHaveAttribute('href', /iana\.org/);
});
