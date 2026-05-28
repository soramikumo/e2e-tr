# Runner テスト仕様

Go ユニット・インテグレーションテストの仕様。
- ✅ 実装済み
- 📝 未実装（追加したいケース）

---

## ユーティリティ（domain）

### ✅ sanitize name removes path traversal characters
`../etc/passwd` のような危険な文字列からスラッシュや `..` が除去される

### ✅ sanitize name with empty string returns non-empty fallback
空文字を渡したとき、パス区切り文字を含まないフォールバック名が返る

### ✅ random ID returns 12 hex characters
12文字の16進数文字列が返る

### ✅ random ID returns unique values each call
2回呼んだとき同じ値にならない

---

## シナリオスキャン（domain）

### ✅ scan tags finds all unique tags across spec files
複数ファイルに同じタグが存在しても、重複なく返る

### ✅ scan tags returns empty list when no spec files exist
`tests/` ディレクトリが空のとき空リストが返る

### ✅ list scenarios returns all spec files with metadata
`tests/*.spec.ts` ファイルが名前・更新日時・サイズ付きで返る

### 📝 list scenarios returns files sorted by modified date descending
新しいファイルが先頭に来る順番で返る

### 📝 scan tags ignores non-spec files
`tests/` 以下の `.spec.ts` 以外のファイルはタグスキャン対象外

---

## Run ドメインモデル（domain）

### ✅ subscribe before finish delivers existing logs
Subscribeより前に追加されたログが、チャンネルに届く

### ✅ subscribe after add log delivers new logs
Subscribe後に追加されたログが、チャンネルに届く

### ✅ subscribe after finish closes channel immediately
Finish後にSubscribeすると、既存ログを受け取ったあとチャンネルが閉じる

### ✅ cancel one subscriber does not affect other subscribers
1つのサブスクライバーをキャンセルしても、他のサブスクライバーはログを受け取れる

### ✅ add log after finish does not panic
Finish後にAddLogを呼んでもパニックしない。ログは内部に保持される

### ✅ concurrent add log and subscribe has no race conditions
複数 goroutine から AddLog・Subscribe・cancel を同時実行しても競合しない

### ✅ finish with success sets status to done
`Finish(true)` 後のステータスが `done` になる

### ✅ finish with failure sets status to failed
`Finish(false)` 後のステータスが `failed` になる

### ✅ finish closes all active subscriber channels
Finish時に購読中のチャンネルが全て閉じる

---

## HTTP ハンドラー（handler）

### ✅ POST /api/run without tag or file returns 400
`tag` も `file` も空のリクエストで 400 が返る

### 📝 POST /api/run with tag starts execution and returns run ID
`tag` 付きのリクエストで実行が開始され、ID が返る

### 📝 POST /api/run with file starts execution and returns run ID
`file` 付きのリクエストで実行が開始され、ID が返る

### 📝 POST /api/run when concurrency limit reached returns 429
同時実行数の上限に達しているとき 429 が返る

### ✅ GET /api/stream with unknown ID returns 404
存在しない ID で 404 が返る

### ✅ GET /api/stream for finished run streams logs then done event
完了済みの Run に接続すると、ログイベントが届いたあと done イベントが届く

### ✅ GET /api/stream client disconnect does not leak goroutine
クライアントが切断してもgoroutineが残らない

### ✅ GET /api/tags returns tags found in spec files
`tests/*.spec.ts` に含まれるタグ一覧が返る

### ✅ GET /api/scenarios returns list of spec files
`tests/` のシナリオ一覧が返る

### ✅ DELETE /api/scenarios deletes the spec file
指定したシナリオファイルが削除される

### 📝 DELETE /api/scenarios with unknown name returns 404
存在しないファイルを削除しようとすると 404 が返る

### 📝 DELETE /api/scenarios with invalid name returns 400
`.spec.ts` でない名前を指定すると 400 が返る（現状は 404）

---

## Executor（executor）

### ✅ execute test with tag passes correct args to runner
タグ指定のとき `--grep @タグ名` が引数に含まれる

### ✅ execute test with file appends spec ts suffix
`.spec.ts` なしのファイル名を渡したとき、自動で付与される

### 📝 execute test with file already having spec ts suffix does not duplicate it
すでに `.spec.ts` がついているファイル名を渡したとき、二重にならない

### 📝 execute test on timeout marks run as failed
タイムアウトしたとき Run のステータスが failed になる

### 📝 execute test on runner error marks run as failed
Runner がエラーを返したとき Run のステータスが failed になる

### 📝 execute test on success marks run as done
Runner が成功を返したとき Run のステータスが done になる
