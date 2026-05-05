.PHONY: help up down test test-runner test-portal test-e2e

help:
	@echo "使い方:"
	@echo "  make up           - docker compose でスタック起動"
	@echo "  make down         - スタック停止"
	@echo "  make test         - 全テスト実行（runner + portal E2E）"
	@echo "  make test-runner  - Go ユニットテストのみ"
	@echo "  make test-portal  - ポータル E2E テストのみ（要: make up）"
	@echo "  make test-e2e     - ユーザーシナリオテストのみ（要: make up）"

up:
	docker compose up -d --build

down:
	docker compose down

test-runner:
	cd runner && go test ./... -v

test-portal:
	cd tests && npx playwright test --config=playwright.portal.config.ts

test-e2e:
	cd tests && npx playwright test

test: test-runner up
	@echo "Waiting for services..."
	@for i in $$(seq 1 30); do \
		curl -sf http://localhost:3000 > /dev/null 2>&1 && \
		curl -sf http://localhost:8080/api/tags > /dev/null 2>&1 && \
		echo "Services ready" && break; \
		echo "Waiting... ($$i/30)"; sleep 2; \
	done
	$(MAKE) test-portal
	$(MAKE) down
