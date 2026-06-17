package handler_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"e2e-runner/internal/config"
	"e2e-runner/internal/handler"
	"e2e-runner/internal/store"
)

// newCodeHandler は既知の TestsDir を持つ Handler を返し、その dir も渡す
// (ScenarioCode はファイル I/O をするため、テスト側で対象ファイルを用意する)。
func newCodeHandler(t *testing.T) (*handler.Handler, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{TestsDir: dir, Port: ":8080", MaxConcurrentRuns: 4}
	return handler.New(cfg, store.NewMemoryRunStore(), store.NewMemoryCodegenStore(), nil), dir
}

func writeSpec(t *testing.T, dir, name, body string) {
	t.Helper()
	testsDir := filepath.Join(dir, "tests")
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testsDir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScenarioCode_GetReturnsSource(t *testing.T) {
	h, dir := newCodeHandler(t)
	writeSpec(t, dir, "foo.spec.ts", "// hello")

	req := httptest.NewRequest(http.MethodGet, "/api/scenarios/code?name=foo.spec.ts", nil)
	w := httptest.NewRecorder()
	h.ScenarioCode(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "// hello") {
		t.Errorf("body に code が含まれない: %s", w.Body.String())
	}
}

func TestScenarioCode_PutWritesSource(t *testing.T) {
	h, dir := newCodeHandler(t)
	writeSpec(t, dir, "foo.spec.ts", "old")

	req := httptest.NewRequest(http.MethodPut, "/api/scenarios/code?name=foo.spec.ts",
		strings.NewReader(`{"code":"new content"}`))
	w := httptest.NewRecorder()
	h.ScenarioCode(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "tests", "foo.spec.ts"))
	if string(got) != "new content" {
		t.Errorf("ファイル内容 = %q, want %q", got, "new content")
	}
}

func TestScenarioCode_PutRejectsNonExistent(t *testing.T) {
	h, _ := newCodeHandler(t)

	req := httptest.NewRequest(http.MethodPut, "/api/scenarios/code?name=ghost.spec.ts",
		strings.NewReader(`{"code":"x"}`))
	w := httptest.NewRecorder()
	h.ScenarioCode(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (新規作成は不可)", w.Code)
	}
}

func TestScenarioCode_RejectsNonSpecName(t *testing.T) {
	h, _ := newCodeHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/scenarios/code?name=evil.ts", nil)
	w := httptest.NewRecorder()
	h.ScenarioCode(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
