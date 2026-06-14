package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"e2e-runner/internal/config"
	"e2e-runner/internal/domain"
	"e2e-runner/internal/handler"
	"e2e-runner/internal/store"
)

// newCodegenHandler は TestsDir を一時ディレクトリに向けた Handler と、その TestsDir を返す。
func newCodegenHandler(t *testing.T) (*handler.Handler, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		TestsDir:          dir,
		Port:              ":8080",
		MaxConcurrentRuns: 4,
		RunTimeout:        5 * time.Second,
	}
	h := handler.New(cfg, store.NewMemoryRunStore(), store.NewMemoryCodegenStore(), nil)
	return h, dir
}

func getCode(t *testing.T, h *handler.Handler, id string) (*http.Response, struct {
	Code   string `json:"code"`
	Status string `json:"status"`
}) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/codegen/code?id="+id, nil)
	w := httptest.NewRecorder()
	h.CodegenCode(w, req)
	res := w.Result()
	var body struct {
		Code   string `json:"code"`
		Status string `json:"status"`
	}
	if res.StatusCode == http.StatusOK {
		if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
			t.Fatalf("decode error: %v", err)
		}
	}
	return res, body
}

// 存在しない ID で 404 が返る。
func TestCodegenCode_UnknownID_Returns404(t *testing.T) {
	h, _ := newCodegenHandler(t)

	res, _ := getCode(t, h, "does-not-exist")

	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.StatusCode)
	}
}

// ファイル生成前（記録開始直後）は空コードで 200 が返る。
func TestCodegenCode_BeforeFileExists_ReturnsEmptyCode(t *testing.T) {
	h, _ := newCodegenHandler(t)
	c := domain.NewCodegen("https://example.com", "pending")
	h.CodegenStore.Save(c)

	res, body := getCode(t, h, c.ID)

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if body.Code != "" {
		t.Errorf("code = %q, want empty", body.Code)
	}
}

// 記録中セッションの spec ファイル内容が code フィールドで返る。
func TestCodegenCode_WhileRecording_ReturnsSpecContent(t *testing.T) {
	h, dir := newCodegenHandler(t)
	c := domain.NewCodegen("https://example.com", "recorded")
	h.CodegenStore.Save(c)

	// codegen が逐次書く spec ファイルを用意する。
	want := "import { test } from '@playwright/test';\n"
	path := filepath.Join(dir, "tests", domain.SanitizeName(c.Name)+".spec.ts")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}

	res, body := getCode(t, h, c.ID)

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if body.Code != want {
		t.Errorf("code = %q, want %q", body.Code, want)
	}
}
