package handler

import (
	"net/http"
	"net/url"
	"os"
	"strings"
)

func CORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

// SameOriginGuard はブラウザ経由のドライブバイ攻撃(悪意あるサイトが被害者の
// ブラウザから localhost の runner へ任意 spec 実行を仕掛ける)を防ぐ軽量ガード。
//
// 方針:
//   - Origin ヘッダが付いているリクエストは、その Origin が許可リストに無ければ 403。
//     ブラウザのクロスサイト fetch には必ず Origin が付くため、別オリジンからの
//     攻撃をここで弾ける。
//   - Origin が無いリクエスト(curl 等の非ブラウザ CLI、同一オリジンの通常
//     ナビゲーション)は通す。無認証ローカルツールという主用途を壊さないための
//     意図的な緩和 ── ブラウザのクロスサイト fetch は必ず Origin を付けるので、
//     Origin 無し = ブラウザ越しのクロスサイト攻撃ではない、と判断できる。
//   - 許可リストの既定は localhost / 127.0.0.1 の任意ポート([::1] も含む)。
//     ALLOWED_ORIGINS env(カンマ区切り)で追加できる。
//
// 重要(SSE 契約): このガードは ResponseWriter を一切ラップしない。判定して
// next を呼ぶか 403 を書くだけ ── http.Flusher を剥がさないため /api/stream・
// /api/codegen/stream を壊さない(routes.go の SSE 契約を参照)。
//
// OPTIONS preflight: preflight にも Origin は付くため、不許可 Origin の OPTIONS は
// ここで 403 になる。ただし合成順契約上 CORS を最外側に置くため、許可 Origin
// (および Origin 無し)の OPTIONS は CORS が 204 を返してから本ガードに到達する
// 前に短絡される。許可リストにある正規の利用元はこれで問題なく動く。
func SameOriginGuard(next http.HandlerFunc) http.HandlerFunc {
	allowed := loadAllowedOrigins()
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && !originAllowed(origin, allowed) {
			http.Error(w, "forbidden: cross-origin request rejected", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// loadAllowedOrigins は ALLOWED_ORIGINS env(カンマ区切り)を読み、追加の許可
// オリジンを返す。未設定でも localhost/127.0.0.1 は originAllowed 側で常に許可する。
func loadAllowedOrigins() []string {
	raw := os.Getenv("ALLOWED_ORIGINS")
	if raw == "" {
		return nil
	}
	var out []string
	for _, s := range strings.Split(raw, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// originAllowed は origin が許可されるかを判定する。
// localhost / 127.0.0.1 / [::1] は(ポートに関わらず)常に許可し、加えて
// extra の各エントリと完全一致するものを許可する。
func originAllowed(origin string, extra []string) bool {
	if isLoopbackOrigin(origin) {
		return true
	}
	for _, e := range extra {
		if origin == e {
			return true
		}
	}
	return false
}

// isLoopbackOrigin は origin のホストがループバック(localhost/127.0.0.1/[::1])
// かどうかを判定する。ポートは問わない。
func isLoopbackOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}
