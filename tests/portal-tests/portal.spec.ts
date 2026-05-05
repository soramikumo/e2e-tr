import { test, expect } from '@playwright/test';

// ── ナビゲーション共通確認 ─────────────────────────────────────────
test.beforeEach(async ({ page }) => {
  await page.goto('/');
});

test('ページタイトルが表示される @smoke', async ({ page }) => {
  await expect(page).toHaveTitle('E2E Test Portal');
});

test('ナビバーにブランドとリンクが表示される @smoke', async ({ page }) => {
  await expect(page.getByText('E2E Portal')).toBeVisible();
  await expect(page.getByRole('link', { name: 'テスト実行' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'シナリオ作成' })).toBeVisible();
});

// ── テスト実行ページ ───────────────────────────────────────────────
test('テスト実行ページ: 見出しが表示される @smoke', async ({ page }) => {
  await expect(page.getByRole('heading', { name: 'テスト実行' })).toBeVisible();
});

test('テスト実行ページ: シナリオセクションが表示される @ui', async ({ page }) => {
  await expect(page.getByRole('heading', { name: 'シナリオで実行' })).toBeVisible();
});

test('テスト実行ページ: シナリオがない場合に案内テキストが表示される @ui', async ({ page }) => {
  // runner が空の場合、空メッセージが表示される（APIが返すシナリオ次第）
  // タグセクションはシナリオ存在時のみ表示のため条件チェック
  const heading = page.getByRole('heading', { name: 'タグで実行' });
  const empty = page.getByText('シナリオがありません');

  // どちらかが表示されていればOK
  const tagVisible = await heading.isVisible().catch(() => false);
  const emptyVisible = await empty.isVisible().catch(() => false);
  expect(tagVisible || emptyVisible).toBe(true);
});

// ── シナリオ作成ページ ─────────────────────────────────────────────
test('シナリオ作成ページ: ナビリンクで遷移できる @smoke', async ({ page }) => {
  await page.getByRole('link', { name: 'シナリオ作成' }).click();
  await expect(page).toHaveURL(/\/create/);
  await expect(page.getByRole('heading', { name: 'シナリオ作成' })).toBeVisible();
});

test('シナリオ作成ページ: URLフォームと記録ボタンが表示される @ui', async ({ page }) => {
  await page.goto('/create');

  const urlInput = page.getByPlaceholder('https://example.com');
  await expect(urlInput).toBeVisible();

  const recordBtn = page.getByRole('button', { name: '記録開始' });
  await expect(recordBtn).toBeVisible();
  // URL未入力なのでボタンは disabled
  await expect(recordBtn).toBeDisabled();
});

test('シナリオ作成ページ: URLを入力すると記録ボタンが有効になる @ui', async ({ page }) => {
  await page.goto('/create');

  await page.getByPlaceholder('https://example.com').fill('https://example.com');
  const recordBtn = page.getByRole('button', { name: '記録開始' });
  await expect(recordBtn).toBeEnabled();
});

test('シナリオ作成ページ: 使い方セクションが表示される @ui', async ({ page }) => {
  await page.goto('/create');
  await expect(page.getByRole('heading', { name: '使い方' })).toBeVisible();
});

// ── ナビゲーション往復 ─────────────────────────────────────────────
test('テスト実行 ↔ シナリオ作成 を往復できる @smoke', async ({ page }) => {
  await page.getByRole('link', { name: 'シナリオ作成' }).click();
  await expect(page).toHaveURL(/\/create/);

  await page.getByRole('link', { name: 'テスト実行' }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole('heading', { name: 'テスト実行' })).toBeVisible();
});
