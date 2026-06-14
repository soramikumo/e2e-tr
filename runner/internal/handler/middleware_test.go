package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"e2e-runner/internal/handler"
)

// okHandler は到達したことを 200 で示すだけのテスト用ハンドラ。
func okHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// TestSameOriginGuard_AllowedOrigin は localhost/127.0.0.1 系の Origin が
// (ポートに関わらず)通過し、next に到達することを確かめる。
func TestSameOriginGuard_AllowedOrigin(t *testing.T) {
	cases := []string{
		"http://localhost:3000",
		"http://127.0.0.1:8080",
		"http://localhost",
		"http://[::1]:3000",
	}
	guarded := handler.SameOriginGuard(okHandler)
	for _, origin := range cases {
		req := httptest.NewRequest(http.MethodPost, "/api/run", nil)
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		guarded(w, req)
		if w.Result().StatusCode != http.StatusOK {
			t.Errorf("origin %q: got %d, want 200 (should pass)", origin, w.Result().StatusCode)
		}
	}
}

// TestSameOriginGuard_DisallowedOrigin は外部サイトの Origin を持つリクエスト
// (= ブラウザ経由のクロスサイト fetch)が 403 で弾かれることを確かめる。
func TestSameOriginGuard_DisallowedOrigin(t *testing.T) {
	cases := []string{
		"http://evil.example.com",
		"https://attacker.test",
		"http://localhost.evil.com", // localhost を含むが別ホスト
	}
	guarded := handler.SameOriginGuard(okHandler)
	for _, origin := range cases {
		req := httptest.NewRequest(http.MethodPost, "/api/run", nil)
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		guarded(w, req)
		if w.Result().StatusCode != http.StatusForbidden {
			t.Errorf("origin %q: got %d, want 403 (should be rejected)", origin, w.Result().StatusCode)
		}
	}
}

// TestSameOriginGuard_NoOrigin は Origin ヘッダの無いリクエスト(curl 等の
// 非ブラウザ、同一オリジンナビゲーション)が通過することを確かめる。
// これは意図的な方針 ── ブラウザのクロスサイト fetch は必ず Origin を付ける。
func TestSameOriginGuard_NoOrigin(t *testing.T) {
	guarded := handler.SameOriginGuard(okHandler)
	req := httptest.NewRequest(http.MethodPost, "/api/run", nil)
	// Origin を一切設定しない。
	w := httptest.NewRecorder()
	guarded(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("no Origin: got %d, want 200 (non-browser/same-origin should pass)", w.Result().StatusCode)
	}
}

// TestSameOriginGuard_AllowedViaEnv は ALLOWED_ORIGINS env で追加した Origin が
// 通過することを確かめる。
func TestSameOriginGuard_AllowedViaEnv(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "https://portal.example.com, https://app.internal")

	guarded := handler.SameOriginGuard(okHandler)

	allow := httptest.NewRequest(http.MethodPost, "/api/run", nil)
	allow.Header.Set("Origin", "https://portal.example.com")
	w := httptest.NewRecorder()
	guarded(w, allow)
	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("env-allowed origin: got %d, want 200", w.Result().StatusCode)
	}

	deny := httptest.NewRequest(http.MethodPost, "/api/run", nil)
	deny.Header.Set("Origin", "https://other.example.com")
	w2 := httptest.NewRecorder()
	guarded(w2, deny)
	if w2.Result().StatusCode != http.StatusForbidden {
		t.Errorf("non-listed origin: got %d, want 403", w2.Result().StatusCode)
	}
}

// TestSameOriginGuard_OptionsDisallowedOrigin は OPTIONS でも不許可 Origin が
// 403 になることを確かめる(SameOriginGuard 単体の挙動 ── 既定チェーンでは CORS が
// 最外側で許可 Origin/Origin 無しの OPTIONS を先に 204 短絡する)。
func TestSameOriginGuard_OptionsDisallowedOrigin(t *testing.T) {
	guarded := handler.SameOriginGuard(okHandler)
	req := httptest.NewRequest(http.MethodOptions, "/api/run", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	w := httptest.NewRecorder()
	guarded(w, req)
	if w.Result().StatusCode != http.StatusForbidden {
		t.Errorf("OPTIONS from disallowed origin: got %d, want 403", w.Result().StatusCode)
	}
}
