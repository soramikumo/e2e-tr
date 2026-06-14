package handler_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"e2e-runner/internal/domain"
	"e2e-runner/internal/handler"
)

// TestRegister_MiddlewareOrder は Register の合成順契約を検証する:
// 先に渡した mw ほど外側、後の mw ほど handler に近い。
// ここでは各 mw がリクエスト処理の前後に自分の名前を order に追記し、
// 外側→内側→(handler)→内側→外側 の入れ子順になることを確かめる。
func TestRegister_MiddlewareOrder(t *testing.T) {
	h := newTestHandler(t)

	var order []string
	record := func(name string) handler.Middleware {
		return func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name+":before")
				next(w, r)
				order = append(order, name+":after")
			}
		}
	}

	mux := http.NewServeMux()
	// outer が先頭 = 最外側、inner が後 = handler に近い。
	h.Register(mux, record("outer"), record("inner"))

	// 副作用の無い GET エンドポイントを叩く(タグ一覧)。
	req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	mux.ServeHTTP(httptest.NewRecorder(), req)

	want := []string{"outer:before", "inner:before", "inner:after", "outer:after"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

// TestRegister_DefaultsToCORS は mw を省略した場合(OSS の従来呼び出し
// h.Register(mux))に既定の CORS が適用され、preflight が 204 を返すことを確かめる。
func TestRegister_DefaultsToCORS(t *testing.T) {
	h := newTestHandler(t)

	mux := http.NewServeMux()
	h.Register(mux) // 引数ゼロ = 後方互換の呼び出し

	req := httptest.NewRequest(http.MethodOptions, "/api/tags", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusNoContent {
		t.Errorf("OPTIONS preflight: got %d, want 204", res.StatusCode)
	}
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Allow-Origin = %q, want %q", got, "*")
	}
}

// TestRegister_DefaultGuardsCrossOrigin は既定チェーン(引数ゼロ)に
// SameOriginGuard が組み込まれ、不許可オリジンの本リクエストが 403 になることを
// 確かめる。許可オリジン(localhost)の同種リクエストは通る。
func TestRegister_DefaultGuardsCrossOrigin(t *testing.T) {
	h := newTestHandler(t)

	mux := http.NewServeMux()
	h.Register(mux) // 既定 = CORS + SameOriginGuard

	// 不許可オリジンの GET → 403(GET なら CORS の OPTIONS 短絡は効かない)。
	bad := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	bad.Header.Set("Origin", "http://evil.example.com")
	wBad := httptest.NewRecorder()
	mux.ServeHTTP(wBad, bad)
	if wBad.Result().StatusCode != http.StatusForbidden {
		t.Errorf("cross-origin GET via default chain: got %d, want 403", wBad.Result().StatusCode)
	}

	// 許可オリジン(localhost)の GET は 403 にならない。
	good := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	good.Header.Set("Origin", "http://localhost:3000")
	wGood := httptest.NewRecorder()
	mux.ServeHTTP(wGood, good)
	if wGood.Result().StatusCode == http.StatusForbidden {
		t.Errorf("same-origin GET via default chain: got 403, want pass-through")
	}
}

// TestRegister_DefaultCORSShortCircuitsPreflight は既定チェーンに
// SameOriginGuard を足しても、CORS が最外側で OPTIONS preflight を 204 で
// 短絡する契約が壊れていないことを確かめる(Origin 無し preflight)。
func TestRegister_DefaultCORSShortCircuitsPreflight(t *testing.T) {
	h := newTestHandler(t)

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodOptions, "/api/tags", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if got := w.Result().StatusCode; got != http.StatusNoContent {
		t.Errorf("preflight via default chain: got %d, want 204", got)
	}
}

// TestRegister_CORSOutermostBeatsRejectingMiddleware は順序契約の実害を検証する:
// CORS を最外側、すべてを 401 で弾く mw をその内側に置くと、OPTIONS preflight は
// 認証に到達する前に CORS が 204 を返す。逆順だと preflight が 401 になり、
// ブラウザが本リクエストを送らなくなる ── これが順序を固定する理由。
func TestRegister_CORSOutermostBeatsRejectingMiddleware(t *testing.T) {
	h := newTestHandler(t)

	reject := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
	}

	mux := http.NewServeMux()
	h.Register(mux, handler.CORS, reject) // CORS が外、reject が内

	req := httptest.NewRequest(http.MethodOptions, "/api/tags", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if got := w.Result().StatusCode; got != http.StatusNoContent {
		t.Errorf("preflight with CORS outermost: got %d, want 204 (CORS must short-circuit OPTIONS before auth)", got)
	}
}

// TestRegister_SSEFlusherSurvivesWrapping は継ぎ目が存在する理由そのものを検証する:
// chain/wrap でルートを mw に包んでも、SSE ハンドラの http.Flusher アサート
// (sseStart)が剥がれないこと。ここを破る mw は SSE を無言で停止させる。
// 注: これは chain 自体が Flusher 透過であることを固定するもので、w をラップする
// 行儀の悪い消費者 mw までは守れない(それは Register の doc コメントの責務)。
func TestRegister_SSEFlusherSurvivesWrapping(t *testing.T) {
	h := newTestHandler(t)

	passthrough := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { next(w, r) }
	}

	mux := http.NewServeMux()
	h.Register(mux, passthrough)

	run := domain.NewRun("seam-sse", "")
	run.AddLog("only line")
	run.Finish(true)
	h.RunStore.Save(context.Background(), run)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/api/stream?id=%s", srv.URL, run.ID))
	if err != nil {
		t.Fatalf("GET stream via Register: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}

	// 1 log + 1 done を受け取れれば、包んだ後も Flusher が機能している証拠。
	events := readSSEEvents(t, resp, 2, 3*time.Second)
	if len(events) < 2 {
		t.Fatalf("expected 2 SSE events through wrapped route, got %d: %v", len(events), events)
	}
	if events[len(events)-1]["type"] != "done" {
		t.Errorf("last event = %v, want done", events[len(events)-1])
	}
}
