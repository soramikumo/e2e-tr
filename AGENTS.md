# Repository Instructions

## Pull Requests

- PR タイトルは、変更種別を示す prefix と日本語の要約で書く。
- 形式は `<type>: <日本語の要約>` とする。
- type は以下から選ぶ。
  - `feat:` ユーザー向け機能の追加
  - `fix:` 不具合修正
  - `docs:` ドキュメントのみの変更
  - `refactor:` 振る舞いを変えないコード整理
  - `test:` テストのみの変更
  - `chore:` 振る舞いに影響しない保守作業
- 要約は短く、何を変えたかが分かる内容にする。
- 例: `feat: run 履歴を SQLite に永続化し portal に履歴一覧を追加`
- PR 本文は `Why`, `What`, `Notes` を使う。

## Issues

- Issue 本文は `Background` と `Notes` を使う。

## Review

- バグ、振る舞いの退行、セキュリティリスク、テスト不足を優先して指摘する。
- 将来の変更を難しくする可読性や構造の問題も指摘する。
- レビューコメントは具体的で、次の行動に移せる内容にする。
