import { test, expect } from '@playwright/test';

test('test', async ({ page }) => {
  await page.goto('https://zenn.dev/mikku_mac/scraps/031d472487186c');
  await page.locator('span:nth-child(4)').click();
  await page.getByRole('heading', { name: 'いろんなWebAPIを理解しようの巻' }).click();
  await page.getByText('間違っているとかあれば、指摘をお願いします。 まず、API').click();
  await expect(page.locator('.SidebarUserBio_container__iWemi > a')).toBeVisible();
  await page.locator('#comment-0985adbe18bdc7').getByText('mi_queue9日前に更新').click();
  await page.locator('.LikeButton_container__YlckE.style-large > .LikeButton_button__ZwdG4').click();
  await page.getByRole('dialog').getByRole('button', { name: '閉じる' }).click();
  await expect(page.locator('h1')).toContainText('いろんなWebAPIを理解しようの巻');
  await page.getByText('通信スタイル ├── リクエスト/レスポンス型（1問1').click();
  await page.locator('.SidebarUserBio_container__iWemi > a').click();
  await page.getByRole('link', { name: '🍣 SAAに合格したけど全然身になってなかった話 1' }).click();
  await page.getByRole('link', { name: 'SAA', exact: true }).click();
  await page.getByRole('img', { name: 'SAA' }).click();
  await page.getByRole('heading', { name: 'SAA', exact: true }).click();
  await page.getByRole('banner').filter({ hasText: 'SAAこのトピックを指定するにはsaaと入力フォロー' }).click();
  await page.getByRole('link', { name: 'AWS SAA - bastion host' }).click();
});