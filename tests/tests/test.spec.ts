import { test, expect } from '@playwright/test';

test('test', async ({ page }) => {
  await page.goto('https://jobs.interviewcat.dev/positions/sansan-118904-f30285be');
  await page.getByRole('link', { name: 'InterviewCat Jobs' }).click();
  await page.getByRole('button', { name: 'ログイン' }).click();
});