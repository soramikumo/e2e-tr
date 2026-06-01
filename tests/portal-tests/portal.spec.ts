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

// ── APIモックを使うテスト ──────────────────────────────────────────
// beforeEach で / に遷移済みのため、ルート設定後に再度 goto('/') する

// シナリオとタグが存在するとき「タグで実行」セクションが表示される @ui
test('test execution page shows tag list when scenarios exist @ui', async ({ page }) => {
  await page.route('**/api/tags', route =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ tags: [{ name: 'smoke', color: '#1f883d' }, { name: 'regression', color: '#1d76db' }] }) })
  );
  await page.route('**/api/scenarios', route =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ scenarios: [{ name: 'smoke.spec.ts', modified: new Date().toISOString(), size: 100, tags: [] }] }) })
  );

  await page.goto('/');

  await expect(page.getByRole('heading', { name: 'タグで実行' })).toBeVisible();
  await expect(page.getByRole('button', { name: '@smoke' })).toBeVisible();
  await expect(page.getByRole('button', { name: '@regression' })).toBeVisible();
});

// 実行ボタンをクリックするとテストが開始しログが流れる @smoke
test('clicking run button starts test and shows log stream @smoke', async ({ page }) => {
  await page.route('**/api/tags', route =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ tags: [] }) })
  );
  await page.route('**/api/scenarios', route =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ scenarios: [{ name: 'smoke.spec.ts', modified: new Date().toISOString(), size: 100 }] }) })
  );
  await page.route('**/api/run', route =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ id: 'test-run-id' }) })
  );
  await page.route('**/api/stream*', route =>
    route.fulfill({
      status: 200,
      headers: { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' },
      body: 'data: {"type":"log","message":"[info] テスト開始: smoke.spec.ts"}\n\ndata: {"type":"done","status":"done"}\n\n',
    })
  );

  await page.goto('/');
  await page.getByRole('button', { name: '実行' }).click();

  await expect(page.getByText('[info] テスト開始: smoke.spec.ts')).toBeVisible({ timeout: 5000 });
});

// テストが成功すると「成功」ステータスが表示される @ui
test('completed test shows success status @ui', async ({ page }) => {
  await page.route('**/api/tags', route =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ tags: [] }) })
  );
  await page.route('**/api/scenarios', route =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ scenarios: [{ name: 'smoke.spec.ts', modified: new Date().toISOString(), size: 100 }] }) })
  );
  await page.route('**/api/run', route =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ id: 'run-success' }) })
  );
  await page.route('**/api/stream*', route =>
    route.fulfill({
      status: 200,
      headers: { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' },
      body: 'data: {"type":"done","status":"done"}\n\n',
    })
  );

  await page.goto('/');
  await page.getByRole('button', { name: '実行' }).click();

  await expect(page.getByText('成功')).toBeVisible({ timeout: 5000 });
});

// テストが失敗すると「失敗」ステータスが表示される @ui
test('failed test shows failure status @ui', async ({ page }) => {
  await page.route('**/api/tags', route =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ tags: [] }) })
  );
  await page.route('**/api/scenarios', route =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ scenarios: [{ name: 'smoke.spec.ts', modified: new Date().toISOString(), size: 100 }] }) })
  );
  await page.route('**/api/run', route =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ id: 'run-fail' }) })
  );
  await page.route('**/api/stream*', route =>
    route.fulfill({
      status: 200,
      headers: { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' },
      body: 'data: {"type":"done","status":"failed"}\n\n',
    })
  );

  await page.goto('/');
  await page.getByRole('button', { name: '実行' }).click();

  await expect(page.getByText('失敗')).toBeVisible({ timeout: 5000 });
});

// ── トレース切り替え ──────────────────────────────────────────────

// トレース保存トグルを ON にすると /api/run の body に trace:true が含まれる @ui
test('trace toggle sends trace flag in run request @ui', async ({ page }) => {
  await page.route('**/api/tags', (route) =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ tags: [{ name: 'smoke', color: '#1f883d' }] }) })
  );
  await page.route('**/api/scenarios', (route) =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ scenarios: [] }) })
  );
  let sentTrace: boolean | undefined;
  await page.route('**/api/run', (route) => {
    sentTrace = (route.request().postDataJSON() as { trace?: boolean }).trace;
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ id: 'run-trace' }) });
  });
  await page.route('**/api/stream*', (route) =>
    route.fulfill({
      status: 200,
      headers: { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' },
      body: 'data: {"type":"done","status":"done"}\n\n',
    })
  );

  await page.goto('/');
  await page.getByRole('checkbox', { name: /トレースを保存/ }).check();
  await page.getByRole('button', { name: '@smoke' }).click();

  await expect(page.getByText('成功')).toBeVisible({ timeout: 5000 });
  expect(sentTrace).toBe(true);
});

// トグル OFF(既定)のときは trace:false で送られる @ui
test('run without trace toggle omits trace @ui', async ({ page }) => {
  await page.route('**/api/tags', (route) =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ tags: [{ name: 'smoke', color: '#1f883d' }] }) })
  );
  await page.route('**/api/scenarios', (route) =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ scenarios: [] }) })
  );
  let sentTrace: boolean | undefined;
  await page.route('**/api/run', (route) => {
    sentTrace = (route.request().postDataJSON() as { trace?: boolean }).trace;
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ id: 'run-no-trace' }) });
  });
  await page.route('**/api/stream*', (route) =>
    route.fulfill({
      status: 200,
      headers: { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' },
      body: 'data: {"type":"done","status":"done"}\n\n',
    })
  );

  await page.goto('/');
  await page.getByRole('button', { name: '@smoke' }).click();

  await expect(page.getByText('成功')).toBeVisible({ timeout: 5000 });
  expect(sentTrace).toBe(false);
});

// シナリオ行ごとの trace チェックボックスが、その行の実行に trace を反映する @ui
test('per-scenario trace checkbox sends trace for that scenario @ui', async ({ page }) => {
  await page.route('**/api/tags', (route) =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ tags: [] }) })
  );
  await page.route('**/api/scenarios', (route) =>
    route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({ scenarios: [{ name: 'a.spec.ts', modified: new Date().toISOString(), size: 1 }] }),
    })
  );
  let sentTrace: boolean | undefined;
  await page.route('**/api/run', (route) => {
    sentTrace = (route.request().postDataJSON() as { trace?: boolean }).trace;
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ id: 'run-a' }) });
  });
  await page.route('**/api/stream*', (route) =>
    route.fulfill({
      status: 200,
      headers: { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' },
      body: 'data: {"type":"done","status":"done"}\n\n',
    })
  );

  await page.goto('/');
  const row = page.locator('.scenario-row', { hasText: 'a.spec.ts' });
  await row.getByRole('checkbox').check();
  await row.getByRole('button', { name: '実行' }).click();

  await expect(page.getByText('成功')).toBeVisible({ timeout: 5000 });
  expect(sentTrace).toBe(true);
});

// ── タグモーダル ───────────────────────────────────────────────────

// シナリオ行の「タグ」ボタンからモーダルを開き、タグを作成して割り当てる @ui
test('tag modal creates a tag and assigns it to the scenario @ui', async ({ page }) => {
  let created: { name?: string; color?: string } | undefined;
  let assigned: { scenario?: string; tags?: string[] } | undefined;
  const tagsState: { name: string; color: string }[] = [];

  await page.route('**/api/tags', (route) => {
    if (route.request().method() === 'POST') {
      created = route.request().postDataJSON();
      tagsState.push({ name: created!.name!, color: created!.color! });
    }
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ tags: tagsState }) });
  });
  await page.route('**/api/scenarios', (route) =>
    route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({ scenarios: [{ name: 'login.spec.ts', modified: new Date().toISOString(), size: 10, tags: [] }] }),
    })
  );
  await page.route('**/api/scenarios/tags', (route) => {
    assigned = route.request().postDataJSON();
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ tags: assigned!.tags }) });
  });

  await page.goto('/');
  await page.locator('.scenario-row', { hasText: 'login.spec.ts' }).getByRole('button', { name: /タグ/ }).click();

  await expect(page.getByText('新規作成')).toBeVisible();
  await page.getByPlaceholder('タグ名').fill('smoke');
  await page.getByRole('button', { name: '作成', exact: true }).click();

  await expect.poll(() => created?.name).toBe('smoke');
  await expect.poll(() => assigned?.tags).toContain('smoke');
});

// モーダルを開き直したとき既存割当が保持され、追加割当で前のタグが消えない @ui
// (サーバを真実の源とする stateful モックで、再オープン→全置換 PUT のデータ損失を防ぐ)
test('reopening tag modal preserves prior assignment @ui', async ({ page }) => {
  const allTags = [
    { name: 'smoke', color: '#1f883d' },
    { name: 'critical', color: '#b60205' },
  ];
  let serverAssigned: string[] = ['smoke']; // 既に smoke 割当済みの状態から開始
  let lastPut: string[] | undefined;

  await page.route('**/api/tags', (route) =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ tags: allTags }) })
  );
  await page.route('**/api/scenarios', (route) =>
    route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({ scenarios: [{ name: 'login.spec.ts', modified: new Date().toISOString(), size: 10, tags: serverAssigned }] }),
    })
  );
  await page.route('**/api/scenarios/tags', (route) => {
    lastPut = (route.request().postDataJSON() as { tags: string[] }).tags;
    serverAssigned = lastPut; // サーバ状態を更新(永続化を模倣)
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ tags: lastPut }) });
  });

  await page.goto('/');
  await page.locator('.scenario-row', { hasText: 'login.spec.ts' }).getByRole('button', { name: /タグ/ }).click();

  // 既存割当 smoke がチェック済みで開く。
  const smokeRow = page.locator('.tag-modal-row', { hasText: 'smoke' });
  await expect(smokeRow.getByRole('checkbox')).toBeChecked();

  // critical を追加 → smoke が消えず両方が送られる。
  await page.locator('.tag-modal-row', { hasText: 'critical' }).getByRole('checkbox').check();
  await expect.poll(() => lastPut).toEqual(['smoke', 'critical']);
});

// ── 並列実行 ───────────────────────────────────────────────────────

// 複数シナリオを同時に実行でき、両方が実行中になる @smoke
test('multiple scenarios run in parallel @smoke', async ({ page }) => {
  await page.route('**/api/tags', (route) =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ tags: [] }) })
  );
  await page.route('**/api/scenarios', (route) =>
    route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({
        scenarios: [
          { name: 'a.spec.ts', modified: new Date().toISOString(), size: 1 },
          { name: 'b.spec.ts', modified: new Date().toISOString(), size: 1 },
        ],
      }),
    })
  );
  let n = 0;
  await page.route('**/api/run', (route) => {
    n += 1;
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ id: `run-${n}` }) });
  });
  // 解決しない＝両方の run が実行中のまま維持される。
  await page.route('**/api/stream*', () => {
    /* keep SSE pending */
  });

  await page.goto('/');
  await page.locator('.scenario-row', { hasText: 'a.spec.ts' }).getByRole('button', { name: '実行' }).click();
  await page.locator('.scenario-row', { hasText: 'b.spec.ts' }).getByRole('button', { name: '実行' }).click();

  await expect(page.getByText('実行中...')).toHaveCount(2);
});

// ── シナリオ作成ページ（追加） ─────────────────────────────────────

// http/https 以外の URL を入力しても記録ボタンが有効にならない @ui
// NOTE: フロントエンドは url が空かどうかのみチェックしているため、このテストは
// 現状失敗する。フロントでのプロトコルバリデーション追加が必要。
test('record button is disabled when invalid URL is entered @ui', async ({ page }) => {
  await page.goto('/create');

  await page.getByPlaceholder('https://example.com').fill('ftp://example.com');
  await expect(page.getByRole('button', { name: '記録開始' })).toBeDisabled();
});

// 記録開始ボタンをクリックすると codegen が開始しステータスが表示される @smoke
test('clicking record button starts codegen and shows stream @smoke', async ({ page }) => {
  await page.route('**/api/codegen/start', route =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ id: 'codegen-001', name: 'test-scenario', noVNCPort: null }) })
  );
  await page.route('**/api/codegen/stream*', route =>
    route.fulfill({
      status: 200,
      headers: { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' },
      // done イベントで完結させる（onerror 後も state が変わらないため安定）
      body: 'data: {"type":"done","file":"test-scenario.spec.ts"}\n\n',
    })
  );

  await page.goto('/create');
  await page.getByPlaceholder('https://example.com').fill('https://example.com');
  await page.getByRole('button', { name: '記録開始' }).click();

  await expect(page.getByText('保存完了', { exact: true })).toBeVisible({ timeout: 5000 });
});

// ── 記録中のビューアとコードパネル ─────────────────────────────────

// 記録状態を維持して /create を開く。SSE(stream) を開いたまま(done/error を送らない)に
// することで、実 VNC を起動せずに isRecording=true の画面を再現する。
async function startRecording(page: import('@playwright/test').Page) {
  await page.route('**/api/codegen/start', (route) =>
    route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({ id: 'cg-1', name: 'scenario', noVNCPort: 6080 }),
    })
  );
  // 解決しない＝EventSource は接続中のまま。onerror が発火せず recording が維持される。
  await page.route('**/api/codegen/stream*', () => {
    /* keep SSE pending */
  });
  // ビューア iframe を実ネットワークに出さないためのスタブ。
  await page.route('**/vnc.html*', (route) =>
    route.fulfill({ contentType: 'text/html', body: '<html></html>' })
  );

  await page.goto('/create');
  await page.getByPlaceholder('https://example.com').fill('https://example.com');
  await page.getByRole('button', { name: '記録開始' }).click();
}

// 記録を開始すると noVNC ビューア(iframe)が表示される @smoke
test('recording shows live browser viewer @smoke', async ({ page }) => {
  await startRecording(page);
  await expect(page.locator('iframe.codegen-viewer')).toBeVisible();
});

// 記録中はビューアの下に「コード表示」トグルボタンが表示される @ui
test('code toggle button appears during recording @ui', async ({ page }) => {
  await startRecording(page);
  await expect(page.getByRole('button', { name: /コード表示/ })).toBeVisible();
});

// 「コード表示」を押すとコードパネルが開き、もう一度押すと閉じる @ui
test('toggling code button shows and hides generated code panel @ui', async ({ page }) => {
  await page.route('**/api/codegen/code*', (route) =>
    route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({ code: "import { test } from '@playwright/test';", status: 'recording' }),
    })
  );
  await startRecording(page);

  const panel = page.locator('pre.codegen-code');
  await expect(panel).toBeHidden();
  await page.getByRole('button', { name: /コード表示/ }).click();
  await expect(panel).toContainText('import { test }');
  await page.getByRole('button', { name: /コードを隠す/ }).click();
  await expect(panel).toBeHidden();
});

// まだコードが無いときプレースホルダが表示される @ui
test('code panel shows placeholder before any action @ui', async ({ page }) => {
  await page.route('**/api/codegen/code*', (route) =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ code: '', status: 'recording' }) })
  );
  await startRecording(page);
  await page.getByRole('button', { name: /コード表示/ }).click();
  await expect(page.locator('pre.codegen-code')).toContainText('まだコードがありません');
});

// ブラウザ操作するたびコードパネルが更新される(ポーリング) @smoke
test('code panel updates live as user records @smoke', async ({ page }) => {
  let calls = 0;
  await page.route('**/api/codegen/code*', (route) => {
    calls++;
    const code =
      calls < 2
        ? "await page.goto('https://example.com/');"
        : "await page.goto('https://example.com/');\nawait page.getByRole('button').click();";
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ code, status: 'recording' }) });
  });
  await startRecording(page);
  await page.getByRole('button', { name: /コード表示/ }).click();

  const panel = page.locator('pre.codegen-code');
  await expect(panel).toContainText('goto');
  await expect(panel).toContainText('getByRole', { timeout: 5000 });
});

// 記録完了時に保存先と次アクションが表示される @ui
test('completed recording shows saved file path and next action @ui', async ({ page }) => {
  await page.route('**/api/codegen/start', (route) =>
    route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({ id: 'cg-2', name: 'scenario', noVNCPort: null }),
    })
  );
  await page.route('**/api/codegen/stream*', (route) =>
    route.fulfill({
      status: 200,
      headers: { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' },
      body: 'data: {"type":"done","file":"scenario.spec.ts"}\n\n',
    })
  );

  await page.goto('/create');
  await page.getByPlaceholder('https://example.com').fill('https://example.com');
  await page.getByRole('button', { name: '記録開始' }).click();

  await expect(page.getByText('保存先: tests/tests/scenario.spec.ts')).toBeVisible({ timeout: 5000 });
  await expect(page.getByRole('button', { name: /テスト実行ページへ/ })).toBeVisible();
});

// ── ナビゲーション（追加） ─────────────────────────────────────────

// 存在しないパスにアクセスしたとき 404 ページが表示される @ui
test('direct URL access to unknown route shows 404 fallback @ui', async ({ page }) => {
  await page.goto('/this-page-does-not-exist');
  await expect(page.getByText('404')).toBeVisible();
});
