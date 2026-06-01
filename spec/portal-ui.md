# Portal UI テスト仕様

Playwright E2E テストの仕様。対象: `http://localhost:3000`
- ✅ 実装済み
- 📝 未実装（追加したいケース）

タグ: `@smoke`（主要フロー）、`@ui`（UI表示確認）

---

## 共通レイアウト

### ✅ page title is displayed @smoke
ページタイトルが `E2E Test Portal` である

### ✅ navbar shows brand and navigation links @smoke
ナビバーに `E2E Portal` ブランドと `テスト実行` `シナリオ作成` リンクが表示される

---

## テスト実行ページ（`/`）

### ✅ test execution page shows heading @smoke
`テスト実行` 見出しが表示される

### ✅ test execution page shows scenario section @ui
`シナリオで実行` セクションが表示される

### ✅ test execution page shows guidance when no scenarios exist @ui
シナリオがないとき `シナリオがありません` が表示される（またはタグセクションが表示される）

### 📝 test execution page shows tag list when scenarios exist @ui
シナリオが存在するとき `タグで実行` セクションにタグ一覧が表示される

### 📝 clicking run button starts test and shows log stream @smoke
実行ボタンを押すと SSE でログが流れてくる

### 📝 completed test shows success status @ui
テストが成功すると `成功` ステータスが表示される

### 📝 failed test shows failure status @ui
テストが失敗すると `失敗` ステータスが表示される

---

## シナリオ作成ページ（`/create`）

### ✅ can navigate to create page via nav link @smoke
`シナリオ作成` リンクをクリックすると `/create` に遷移する

### ✅ create page shows URL input and disabled record button @ui
URL フォームと無効化された `記録開始` ボタンが表示される

### ✅ record button becomes enabled when URL is entered @ui
URL を入力すると `記録開始` ボタンが有効になる

### ✅ create page shows usage instructions section @ui
`使い方` セクションが表示される

### 📝 record button is disabled when invalid URL is entered @ui
無効な URL（`http` でないもの）を入力するとボタンが有効にならない

### 📝 clicking record button starts codegen and shows stream @smoke
`記録開始` を押すと録画が開始されストリームが表示される

---

## 記録中のビューアとコードパネル（`/create`）

ブラウザはサーバー側の仮想画面で動き、noVNC(KasmVNC)経由でビューアに映る。
Playwright Inspector はオフスクリーンに押し出し、生成コードはポータルのパネルで見せる。

### ✅ recording shows live browser viewer @smoke
記録を開始すると noVNC ビューア（iframe）が表示され、操作対象ブラウザが枠いっぱいに映る

### ✅ code toggle button appears during recording @ui
記録中はビューアの下に `コード表示 ▾` トグルボタンが表示される

### ✅ toggling code button shows and hides generated code panel @ui
`コード表示` を押すとコードパネルが開き、もう一度押すと閉じる

### ✅ code panel shows placeholder before any action @ui
まだコードが無いとき `// まだコードがありません...` のプレースホルダが表示される

### ✅ code panel updates live as user records @smoke
ブラウザで操作するたび、コードパネルの内容が更新される（`/api/codegen/code` を定期取得）

### ✅ completed recording shows saved file path and next action @ui
記録完了時に `保存先: tests/tests/<file>` と `テスト実行ページへ` ボタンが表示される

---

## ナビゲーション

### ✅ can navigate back and forth between test execution and scenario creation @smoke
テスト実行 → シナリオ作成 → テスト実行 と往復できる

### 📝 direct URL access to unknown route shows appropriate fallback @ui
存在しないパスにアクセスしたとき適切なフォールバックが表示される
