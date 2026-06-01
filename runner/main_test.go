package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"e2e-runner/config"
	"e2e-runner/domain"
	"e2e-runner/handler"
	"e2e-runner/store"
)

func newTestHandler(t *testing.T, testsDir string) *handler.Handler {
	t.Helper()
	cfg := &config.Config{TestsDir: testsDir, Port: ":8080", DBPath: "", MaxConcurrentRuns: 4, RunTimeout: 5 * time.Second}
	return handler.New(cfg, store.NewMemoryRunStore(), store.NewMemoryCodegenStore(), nil)
}

// ── domain.SanitizeName ───────────────────────────────────────────────────────

func TestSanitizeName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"my-scenario", "my-scenario"},
		{"../etc/passwd", "etcpasswd"},
		{"foo/bar", "foobar"},
		{"foo\\bar", "foobar"},
		{"  spaced  ", "spaced"},
	}
	for _, tc := range cases {
		got := domain.SanitizeName(tc.input)
		if got != tc.want {
			t.Errorf("SanitizeName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSanitizeNameEmpty(t *testing.T) {
	got := domain.SanitizeName("")
	if got == "" {
		t.Error("SanitizeName(\"\") should return a non-empty fallback name")
	}
	if strings.Contains(got, "/") || strings.Contains(got, "\\") {
		t.Errorf("SanitizeName(\"\") fallback contains path separator: %q", got)
	}
}

// ── domain.RandomID ───────────────────────────────────────────────────────────

func TestRandomID(t *testing.T) {
	id := domain.RandomID()
	if len(id) != 12 {
		t.Errorf("RandomID() length = %d, want 12", len(id))
	}
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("RandomID() contains non-hex char: %q", c)
		}
	}
	if id2 := domain.RandomID(); id == id2 {
		t.Error("RandomID() returned identical values twice (improbable, re-run to confirm)")
	}
}

// ── domain.ScanTags ───────────────────────────────────────────────────────────

func TestScanTags(t *testing.T) {
	dir := t.TempDir()
	testsSubDir := filepath.Join(dir, "tests")
	os.MkdirAll(testsSubDir, 0755)
	writeSpec(t, testsSubDir, "a.spec.ts", "test('foo @smoke', ...);\ntest('bar @regression', ...);")
	writeSpec(t, testsSubDir, "b.spec.ts", "test('baz @smoke', ...);")

	tags := domain.ScanTags(dir)
	if len(tags) != 2 {
		t.Errorf("ScanTags() = %v, want 2 unique tags", tags)
	}
	tagSet := map[string]bool{}
	for _, tag := range tags {
		tagSet[tag] = true
	}
	if !tagSet["smoke"] {
		t.Error("expected tag 'smoke' not found")
	}
	if !tagSet["regression"] {
		t.Error("expected tag 'regression' not found")
	}
}

func TestScanTagsEmpty(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "tests"), 0755)
	if tags := domain.ScanTags(dir); len(tags) != 0 {
		t.Errorf("ScanTags() on empty dir = %v, want []", tags)
	}
}

// ── domain.ListScenarios ──────────────────────────────────────────────────────

func TestListScenarios(t *testing.T) {
	dir := t.TempDir()
	testsSubDir := filepath.Join(dir, "tests")
	os.MkdirAll(testsSubDir, 0755)
	writeSpec(t, testsSubDir, "first.spec.ts", `test('a', () => {})`)
	writeSpec(t, testsSubDir, "second.spec.ts", `test('b', () => {})`)

	scenarios := domain.ListScenarios(dir)
	if len(scenarios) != 2 {
		t.Errorf("ListScenarios() = %d scenarios, want 2", len(scenarios))
	}
	names := map[string]bool{}
	for _, s := range scenarios {
		names[s.Name] = true
	}
	if !names["first.spec.ts"] || !names["second.spec.ts"] {
		t.Errorf("unexpected scenario names: %v", scenarios)
	}
}

// ── HTTP handlers ─────────────────────────────────────────────────────────────

func TestHandleTags(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "tests"), 0755)
	writeSpec(t, filepath.Join(dir, "tests"), "s.spec.ts", `test('@api smoke', () => {})`)

	h := newTestHandler(t, dir)
	req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	w := httptest.NewRecorder()
	h.Tags(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("Tags status = %d, want 200", w.Result().StatusCode)
	}
	var body struct {
		Tags []struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		} `json:"tags"`
	}
	if err := json.NewDecoder(w.Result().Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(body.Tags) == 0 {
		t.Error("expected at least one tag in response")
	}
	// 既存 spec の @tag がブートストラップで初期色付きタグになる。
	if body.Tags[0].Color == "" {
		t.Error("expected bootstrapped tag to have a color")
	}
}

func TestHandleScenarios_GET(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "tests"), 0755)
	writeSpec(t, filepath.Join(dir, "tests"), "demo.spec.ts", `test('x', () => {})`)

	h := newTestHandler(t, dir)
	req := httptest.NewRequest(http.MethodGet, "/api/scenarios", nil)
	w := httptest.NewRecorder()
	h.Scenarios(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("Scenarios GET status = %d, want 200", w.Result().StatusCode)
	}
	var body struct {
		Scenarios []domain.Scenario `json:"scenarios"`
	}
	if err := json.NewDecoder(w.Result().Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(body.Scenarios) != 1 || body.Scenarios[0].Name != "demo.spec.ts" {
		t.Errorf("unexpected scenarios: %+v", body.Scenarios)
	}
}

func TestHandleScenarios_DELETE(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "tests"), 0755)
	writeSpec(t, filepath.Join(dir, "tests"), "todelete.spec.ts", `test('x', () => {})`)

	h := newTestHandler(t, dir)
	req := httptest.NewRequest(http.MethodDelete, "/api/scenarios?name=todelete.spec.ts", nil)
	w := httptest.NewRecorder()
	h.Scenarios(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("DELETE status = %d, want 200", w.Result().StatusCode)
	}
	_, err := os.Stat(filepath.Join(dir, "tests", "todelete.spec.ts"))
	if !os.IsNotExist(err) {
		t.Error("file should have been deleted")
	}
}

func TestHandleRun_MissingBody(t *testing.T) {
	h := newTestHandler(t, t.TempDir())
	req := httptest.NewRequest(http.MethodPost, "/api/run", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Run(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("Run with empty body status = %d, want 400", w.Result().StatusCode)
	}
}

// ── domain.ListScenarios ──────────────────────────────────────────────────────

// 新しいファイルが先頭に来る順番で返ることを確認する。
func TestListScenarios_SortedByModifiedDateDescending(t *testing.T) {
	dir := t.TempDir()
	testsSubDir := filepath.Join(dir, "tests")
	os.MkdirAll(testsSubDir, 0755)

	writeSpec(t, testsSubDir, "old.spec.ts", `test('a', () => {})`)
	writeSpec(t, testsSubDir, "new.spec.ts", `test('b', () => {})`)

	now := time.Now()
	older := now.Add(-1 * time.Hour)
	os.Chtimes(filepath.Join(testsSubDir, "old.spec.ts"), older, older)
	os.Chtimes(filepath.Join(testsSubDir, "new.spec.ts"), now, now)

	scenarios := domain.ListScenarios(dir)
	if len(scenarios) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(scenarios))
	}
	if scenarios[0].Name != "new.spec.ts" {
		t.Errorf("expected new.spec.ts first, got %s", scenarios[0].Name)
	}
	if scenarios[1].Name != "old.spec.ts" {
		t.Errorf("expected old.spec.ts second, got %s", scenarios[1].Name)
	}
}

// tests/ 以下の .spec.ts 以外のファイルはタグスキャン対象外であることを確認する。
func TestScanTags_IgnoresNonSpecFiles(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "tests"), 0755)
	writeSpec(t, filepath.Join(dir, "tests"), "helper.ts", `// @smoke @regression`)

	if tags := domain.ScanTags(dir); len(tags) != 0 {
		t.Errorf("ScanTags() should ignore non-spec files, got %v", tags)
	}
}

// ── HTTP ハンドラー（scenarios） ──────────────────────────────────────────────

// 存在しないファイルを DELETE したとき 404 が返ることを確認する。
func TestHandleScenarios_DELETE_NotFound(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "tests"), 0755)

	h := newTestHandler(t, dir)
	req := httptest.NewRequest(http.MethodDelete, "/api/scenarios?name=notexist.spec.ts", nil)
	w := httptest.NewRecorder()
	h.Scenarios(w, req)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("DELETE nonexistent status = %d, want 404", w.Result().StatusCode)
	}
}

// .spec.ts でない名前を DELETE したとき 400 が返ることを確認する。
func TestHandleScenarios_DELETE_InvalidName(t *testing.T) {
	dir := t.TempDir()
	h := newTestHandler(t, dir)
	req := httptest.NewRequest(http.MethodDelete, "/api/scenarios?name=foo.ts", nil)
	w := httptest.NewRecorder()
	h.Scenarios(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("DELETE invalid name status = %d, want 400", w.Result().StatusCode)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeSpec(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
