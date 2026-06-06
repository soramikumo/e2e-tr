# e2e-tr タスクランナー（just）
# 使い方: `just` で一覧、`just <recipe>` で実行。
# Docker のビルドは compose 経由（runner + portal を一括）。

# 引数なしで叩いたらレシピ一覧を表示する。
default:
    @just --list

# ── Docker ──────────────────────────────────────────────

# ビルドして起動（これが「毎回忘れるやつ」。--build で常に作り直す）。
up:
    docker compose up -d --build

# ビルドだけ（起動はしない）。
build:
    docker compose build

# キャッシュを使わず作り直す（Dockerfile やベースイメージ更新後に）。
rebuild:
    docker compose build --no-cache

# 片方だけビルド: `just build-one runner` / `just build-one portal`
build-one service:
    docker compose build {{service}}

# 停止。
down:
    docker compose down

# 停止 + ボリューム削除（クリーンに作り直したいとき）。
down-v:
    docker compose down -v

# 再起動（落として作り直して起動）。
restart: down up

# ログ追尾（全サービス）。`just logs runner` で個別も可。
logs service="":
    docker compose logs -f {{service}}

# 起動中サービスの状態。
ps:
    docker compose ps

# ── Tests ───────────────────────────────────────────────

# Go ユニットテスト。
test-runner:
    cd runner && go test ./... -v

# ポータル E2E（要: just up）。
test-portal:
    cd tests && npx playwright test --config=playwright.portal.config.ts

# ユーザーシナリオ E2E（要: just up）。
test-e2e:
    cd tests && npx playwright test

# 全テスト: runner ユニット → スタック起動 → ヘルスチェック → portal E2E → 停止。
test: test-runner up
    @echo "Waiting for services..."
    @for i in $(seq 1 30); do \
        curl -sf http://localhost:3000 > /dev/null 2>&1 && \
        curl -sf http://localhost:8080/api/tags > /dev/null 2>&1 && \
        echo "Services ready" && break; \
        if [ $i -eq 30 ]; then echo "Services did not start in time" && exit 1; fi; \
        echo "Waiting... ($i/30)"; sleep 2; \
    done
    just test-portal
    just down
