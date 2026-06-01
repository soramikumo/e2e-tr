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
`tag` 付きのリクエストで、そのタグが割り当てられた spec 群を解決して実行が開始され、ID が返る

### ✅ POST /api/run with tag having no scenarios returns 400
割当シナリオが無いタグでの実行は 400（空の `playwright test` で全件暴発させない）

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

### ✅ GET /api/tags returns color-tagged definitions
`.tags.json`（メタデータ）の色付きタグ定義一覧 `{name, color}` が返る。未生成時は既存 spec の `@tag` を初期色で取り込む

### ✅ POST /api/tags creates or updates a tag
`{name, color}` でタグを作成。同名なら色を更新。`color` は `#rrggbb` 必須

### ✅ DELETE /api/tags removes a tag and cascades
タグ定義を削除し、全シナリオの割当からも取り除く（stale 割当を残さない）

### ✅ PUT /api/scenarios/tags replaces a scenario's assignments
`{scenario, tags}` で指定シナリオのタグ割当を丸ごと置き換える

### ✅ GET /api/scenarios returns list of spec files
`tests/` のシナリオ一覧が、各シナリオの割当タグ付きで返る

### ✅ POST /api/run with trace flag sets Trace on the run
`trace:true` のリクエストで `Run.Trace` が true になる

### 📝 POST /api/codegen/start returns id and noVNCPort
記録開始で `id` と `noVNCPort` が返り、VNC セッションが起動する

### ✅ GET /api/codegen/code with unknown ID returns 404
存在しない ID で 404 が返る

### ✅ GET /api/codegen/code returns current spec content while recording
記録中セッションの spec ファイル内容が `code` フィールドで返る

### ✅ GET /api/codegen/code before file exists returns empty code with 200
ファイル生成前は空の `code` で 200 が返る（ポーリング側を単純化するため）

### ✅ DELETE /api/scenarios deletes the spec file
指定したシナリオファイルが削除される

### 📝 DELETE /api/scenarios with unknown name returns 404
存在しないファイルを削除しようとすると 404 が返る

### 📝 DELETE /api/scenarios with invalid name returns 400
`.spec.ts` でない名前を指定すると 400 が返る（現状は 404）

---

## Executor（executor）

### ✅ execute test with tag passes correct args to runner
タグ実行のとき、割当 spec 群を `playwright test tests/a.spec.ts tests/b.spec.ts ...` として複数ファイル指定で渡す

### ✅ execute test with file appends spec ts suffix
`.spec.ts` なしのファイル名を渡したとき、自動で付与される

### ✅ execute test with trace enabled appends --trace on
`Run.Trace` が true のとき `--trace on` が引数末尾に加わる

### ✅ execute test without trace omits --trace flag
`Run.Trace` が false（既定）のとき `--trace` は加わらない

### ✅ execute codegen under VNC passes viewport-size to fill framebuffer
VNC セッション配下のとき `--viewport-size=1600,820` が引数に含まれる（ブラウザを画面いっぱいに開き Inspector をオフスクリーンに追い出すため）

### ✅ execute codegen under VNC sets DISPLAY env without disabling inspector
`DISPLAY` 環境変数が設定され、`PW_CODEGEN_NO_INSPECTOR` は付与されない（記録・保存を生かすため）

### ✅ execute codegen without VNC omits viewport-size and DISPLAY
VNC を使わないとき `--viewport-size` も `DISPLAY` も付与されない

### 📝 execute test with file already having spec ts suffix does not duplicate it
すでに `.spec.ts` がついているファイル名を渡したとき、二重にならない

### 📝 execute test on timeout marks run as failed
タイムアウトしたとき Run のステータスが failed になる

### 📝 execute test on runner error marks run as failed
Runner がエラーを返したとき Run のステータスが failed になる

### 📝 execute test on success marks run as done
Runner が成功を返したとき Run のステータスが done になる
