package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── sanitizeName ─────────────────────────────────────────────────────────────

func TestSanitizeName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"my-scenario", "my-scenario"},
		// パストラバーサル防止
		{"../etc/passwd", "etcpasswd"},
		{"foo/bar", "foobar"},
		{"foo\\bar", "foobar"},
		// 前後スペース除去
		{"  spaced  ", "spaced"},
	}

	for _, tc := range cases {
		got := sanitizeName(tc.input)
		if got != tc.want {
			t.Errorf("sanitizeName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSanitizeNameEmpty(t *testing.T) {
	got := sanitizeName("")
	if got == "" {
		t.Error("sanitizeName(\"\") should return a non-empty fallback name")
	}
	if strings.Contains(got, "/") || strings.Contains(got, "\\") {
		t.Errorf("sanitizeName(\"\") fallback contains path separator: %q", got)
	}
}

// ── randomID ─────────────────────────────────────────────────────────────────

func TestRandomID(t *testing.T) {
	id := randomID()
	if len(id) != 12 {
		t.Errorf("randomID() length = %d, want 12", len(id))
	}
	// hex 文字のみ
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("randomID() contains non-hex char: %q", c)
		}
	}
	// 衝突しないこと（確率的）
	if id2 := randomID(); id == id2 {
		t.Error("randomID() returned identical values twice (improbable, re-run to confirm)")
	}
}

// ── scanTags ─────────────────────────────────────────────────────────────────

func TestScanTags(t *testing.T) {
	dir := t.TempDir()
	origDir := testsDir
	testsDir = dir
	defer func() { testsDir = origDir }()

	testsSubDir := filepath.Join(dir, "tests")
	if err := os.MkdirAll(testsSubDir, 0755); err != nil {
		t.Fatal(err)
	}

	// タグ付きスペックを2ファイル作成
	writeSpec(t, testsSubDir, "a.spec.ts", `test('foo @smoke', ...);\ntest('bar @regression', ...);`)
	writeSpec(t, testsSubDir, "b.spec.ts", `test('baz @smoke', ...);`) // @smoke は重複

	tags := scanTags()

	if len(tags) != 2 {
		t.Errorf("scanTags() = %v, want 2 unique tags", tags)
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
	origDir := testsDir
	testsDir = dir
	defer func() { testsDir = origDir }()

	os.MkdirAll(filepath.Join(dir, "tests"), 0755)

	tags := scanTags()
	if len(tags) != 0 {
		t.Errorf("scanTags() on empty dir = %v, want []", tags)
	}
}

// ── listScenarios ─────────────────────────────────────────────────────────────

func TestListScenarios(t *testing.T) {
	dir := t.TempDir()
	origDir := testsDir
	testsDir = dir
	defer func() { testsDir = origDir }()

	testsSubDir := filepath.Join(dir, "tests")
	os.MkdirAll(testsSubDir, 0755)

	writeSpec(t, testsSubDir, "first.spec.ts", `test('a', () => {})`)
	writeSpec(t, testsSubDir, "second.spec.ts", `test('b', () => {})`)

	scenarios := listScenarios()
	if len(scenarios) != 2 {
		t.Errorf("listScenarios() = %d scenarios, want 2", len(scenarios))
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
	origDir := testsDir
	testsDir = dir
	defer func() { testsDir = origDir }()

	os.MkdirAll(filepath.Join(dir, "tests"), 0755)
	writeSpec(t, filepath.Join(dir, "tests"), "s.spec.ts", `test('@api smoke', () => {})`)

	req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	w := httptest.NewRecorder()
	handleTags(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("handleTags status = %d, want 200", res.StatusCode)
	}

	var body struct {
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(body.Tags) == 0 {
		t.Error("expected at least one tag in response")
	}
}

func TestHandleScenarios_GET(t *testing.T) {
	dir := t.TempDir()
	origDir := testsDir
	testsDir = dir
	defer func() { testsDir = origDir }()

	os.MkdirAll(filepath.Join(dir, "tests"), 0755)
	writeSpec(t, filepath.Join(dir, "tests"), "demo.spec.ts", `test('x', () => {})`)

	req := httptest.NewRequest(http.MethodGet, "/api/scenarios", nil)
	w := httptest.NewRecorder()
	handleScenarios(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("handleScenarios GET status = %d, want 200", res.StatusCode)
	}

	var body struct {
		Scenarios []Scenario `json:"scenarios"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(body.Scenarios) != 1 || body.Scenarios[0].Name != "demo.spec.ts" {
		t.Errorf("unexpected scenarios: %+v", body.Scenarios)
	}
}

func TestHandleScenarios_DELETE(t *testing.T) {
	dir := t.TempDir()
	origDir := testsDir
	testsDir = dir
	defer func() { testsDir = origDir }()

	os.MkdirAll(filepath.Join(dir, "tests"), 0755)
	writeSpec(t, filepath.Join(dir, "tests"), "todelete.spec.ts", `test('x', () => {})`)

	req := httptest.NewRequest(http.MethodDelete, "/api/scenarios?name=todelete.spec.ts", nil)
	w := httptest.NewRecorder()
	handleScenarios(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("DELETE status = %d, want 200", w.Result().StatusCode)
	}

	// ファイルが削除されていること
	_, err := os.Stat(filepath.Join(dir, "tests", "todelete.spec.ts"))
	if !os.IsNotExist(err) {
		t.Error("file should have been deleted")
	}
}

func TestHandleRun_MissingBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/run", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handleRun(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("handleRun with empty body status = %d, want 400", w.Result().StatusCode)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func writeSpec(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
