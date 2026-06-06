package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"e2e-runner/config"
	"e2e-runner/executor"
	"e2e-runner/handler"
	"e2e-runner/store"
)

// nopRunner は即座に nil を返す。実際のコマンドは実行しない。
type nopRunner struct{}

func (nopRunner) Run(_ context.Context, _ executor.RunOptions) error { return nil }

func newHandlerWithFakeRunner(t *testing.T) *handler.Handler {
	t.Helper()
	cfg := &config.Config{
		TestsDir:          t.TempDir(),
		Port:              ":8080",
		MaxConcurrentRuns: 4,
		RunTimeout:        5 * time.Second,
	}
	h := handler.New(cfg, store.NewMemoryRunStore(), store.NewMemoryCodegenStore(), nil)
	h.Executor.Runner = nopRunner{}
	return h
}

// タグ付きリクエストで実行が開始され、ID が返ることを確認する。
func TestHandleRun_WithTag_ReturnsRunID(t *testing.T) {
	h := newHandlerWithFakeRunner(t)
	h.TagStore.SetScenarioTags("login.spec.ts", []string{"smoke"})

	req := httptest.NewRequest(http.MethodPost, "/api/run", strings.NewReader(`{"tag":"smoke"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Run(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Result().StatusCode)
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(w.Result().Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.ID == "" {
		t.Error("response id is empty")
	}
	if _, ok := h.RunStore.Get(context.Background(), body.ID); !ok {
		t.Errorf("run %q not found in store", body.ID)
	}
}

// trace:true 付きリクエストで Run.Trace が立つことを確認する。
func TestHandleRun_WithTrace_SetsTraceOnRun(t *testing.T) {
	h := newHandlerWithFakeRunner(t)
	h.TagStore.SetScenarioTags("login.spec.ts", []string{"smoke"})

	req := httptest.NewRequest(http.MethodPost, "/api/run", strings.NewReader(`{"tag":"smoke","trace":true}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Run(w, req)

	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(w.Result().Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	run, ok := h.RunStore.Get(context.Background(), body.ID)
	if !ok {
		t.Fatalf("run %q not found in store", body.ID)
	}
	if !run.Trace {
		t.Error("run.Trace = false, want true")
	}
}

// ファイル指定のリクエストで実行が開始され、ID が返ることを確認する。
func TestHandleRun_WithFile_ReturnsRunID(t *testing.T) {
	h := newHandlerWithFakeRunner(t)

	req := httptest.NewRequest(http.MethodPost, "/api/run", strings.NewReader(`{"file":"login"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Run(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Result().StatusCode)
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(w.Result().Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.ID == "" {
		t.Error("response id is empty")
	}
}

// 割当のないタグで実行すると 400 になることを確認する(空の playwright 実行を防ぐ)。
func TestHandleRun_TagWithNoScenarios_Returns400(t *testing.T) {
	h := newHandlerWithFakeRunner(t)

	req := httptest.NewRequest(http.MethodPost, "/api/run", strings.NewReader(`{"tag":"ghost"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Run(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Result().StatusCode)
	}
}

// 同時実行数の上限に達しているとき 429 が返ることを確認する。
// MaxConcurrentRuns = 0 にするとセマフォが常に満杯になる。
func TestHandleRun_ConcurrencyLimitReached_Returns429(t *testing.T) {
	cfg := &config.Config{
		TestsDir:          t.TempDir(),
		Port:              ":8080",
		MaxConcurrentRuns: 0,
		RunTimeout:        5 * time.Second,
	}
	h := handler.New(cfg, store.NewMemoryRunStore(), store.NewMemoryCodegenStore(), nil)
	h.Executor.Runner = nopRunner{}

	req := httptest.NewRequest(http.MethodPost, "/api/run", strings.NewReader(`{"file":"login"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Run(w, req)

	if w.Result().StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", w.Result().StatusCode)
	}
}

// baseURL に不正値を渡すと 400 で弾かれること(env へそのまま流さない)。
func TestHandleRun_InvalidBaseURL_Rejected(t *testing.T) {
	h := newHandlerWithFakeRunner(t)

	req := httptest.NewRequest(http.MethodPost, "/api/run",
		strings.NewReader(`{"file":"a.spec.ts","baseURL":"javascript:alert(1)"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Run(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Result().StatusCode)
	}
}

// baseURL に http/https を渡すと受理され、Run に保持されること。
func TestHandleRun_ValidBaseURL_Accepted(t *testing.T) {
	h := newHandlerWithFakeRunner(t)

	req := httptest.NewRequest(http.MethodPost, "/api/run",
		strings.NewReader(`{"file":"a.spec.ts","baseURL":"https://staging.example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Run(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Result().StatusCode)
	}
	var body struct {
		ID string `json:"id"`
	}
	json.NewDecoder(w.Result().Body).Decode(&body)
	run, ok := h.RunStore.Get(context.Background(), body.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if run.BaseURL != "https://staging.example.com" {
		t.Errorf("BaseURL = %q, want staging", run.BaseURL)
	}
}
