package domain

import "strings"

import "testing"

const recordedSpec = `import { test, expect } from '@playwright/test';

test.use({
  viewport: {
    height: 820,
    width: 1600
  }
});

test('test', async ({ page }) => {
  await page.goto('https://app.example.com/cart?x=1&y=2');
  await page.getByText('製品').click();
  await page.goto('https://other.example.com/x');
});
`

func TestRelativizeFirstGoto_RewritesEntryAndInjectsBaseURL(t *testing.T) {
	out := RelativizeFirstGoto(recordedSpec)

	if !strings.Contains(out, "page.goto('/cart?x=1&y=2')") {
		t.Errorf("最初の goto が相対化されていない:\n%s", out)
	}
	if !strings.Contains(out, "baseURL: process.env.PLAYWRIGHT_BASE_URL ?? 'https://app.example.com'") {
		t.Errorf("baseURL 既定が注入されていない:\n%s", out)
	}
	// 2 つ目の goto は触らない(エントリポイントのみ対象)。
	if !strings.Contains(out, "page.goto('https://other.example.com/x')") {
		t.Errorf("2 つ目の絶対 goto が誤って書き換えられた:\n%s", out)
	}
	// 既存の test.use(viewport) は残す。
	if !strings.Contains(out, "width: 1600") {
		t.Errorf("既存の test.use が壊れた:\n%s", out)
	}
}

func TestRelativizeFirstGoto_NoAbsoluteGoto_Unchanged(t *testing.T) {
	src := "test('t', async ({ page }) => { await page.goto('/already-relative'); });"
	if out := RelativizeFirstGoto(src); out != src {
		t.Errorf("絶対 goto が無いのに変更された:\n%s", out)
	}
}

func TestRelativizeFirstGoto_RootPath(t *testing.T) {
	src := "import { test } from '@playwright/test';\nawait page.goto('https://example.com');"
	out := RelativizeFirstGoto(src)
	if !strings.Contains(out, "page.goto('/')") {
		t.Errorf("ルート URL が '/' に相対化されていない:\n%s", out)
	}
}
