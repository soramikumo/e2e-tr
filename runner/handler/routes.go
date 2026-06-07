package handler

import "net/http"

// Middleware は 1 つのハンドラを別のハンドラで包む、合成可能な関数。
// OSS はこの型に auth など固有の意味を持たせない ── 上位レイヤ(Web 版)が
// 認証や credentialed CORS などの自前ミドルウェアを差し込むための、
// 中立な継ぎ目として定義しているだけ。
type Middleware func(http.HandlerFunc) http.HandlerFunc

// Register は runner の API ルートを mux に登録する。
//
// 各ルートは mw で包まれる。mw を省略すると OSS 既定の CORS のみが適用され、
// 従来挙動と同一(後方互換 ── 引数ゼロの h.Register(mux) はそのまま動く)。
// Web 版は独自の CORS/認証を mw として渡すことで、この API 面を再利用しつつ
// 振る舞いだけを差し替えられる(OSS は auth を一切知らないままでいられる)。
//
// 重要: 既定の CORS は「mw を渡さなかった時だけ」適用される(常時適用ではない)。
// mw を1つでも渡すと既定は無効になるため、CORS が必要な呼び出し側は CORS 相当の
// mw を**自分で先頭に渡す**こと。例: Register(mux, myCORS, RequireAuth)。これは
// 意図的な設計 ── OSS 既定の `Access-Control-Allow-Origin: *` は cookie 認証
// (credentialed)と併用できないため、Web 版は特定オリジン+Allow-Credentials の
// CORS を自前で渡す必要がある。auth だけ渡して CORS を忘れると、別オリジンの
// portal からのリクエストが CORS 無しで壊れる。
//
// 合成順の契約: 先に渡した mw ほど外側、後の mw ほど handler に近い。
//
//	Register(mux, CORS, RequireAuth) => CORS(RequireAuth(handler))
//
// これは必須の契約 ── ブラウザの OPTIONS preflight は cookie を乗せずに飛ぶため、
// CORS を最外側に置いて 204 を返してから認証する必要がある(逆順だと preflight
// が 401 になり、本リクエストがブラウザから送られない)。
//
// SSE の契約: /api/stream・/api/codegen/stream は ResponseWriter を
// http.Flusher にアサートする(handler.go の sseStart)。渡す mw は
// ResponseWriter をラップしてはならない ── ラップすると Flusher が剥がれ、
// SSE が無言で停止する。
func (h *Handler) Register(mux *http.ServeMux, mw ...Middleware) {
	if len(mw) == 0 {
		mw = []Middleware{CORS}
	}
	wrap := func(fn http.HandlerFunc) http.HandlerFunc { return chain(fn, mw...) }

	mux.HandleFunc("/api/tags", wrap(h.Tags))
	mux.HandleFunc("/api/run", wrap(h.Run))
	mux.HandleFunc("/api/stream", wrap(h.Stream))
	mux.HandleFunc("/api/codegen/start", wrap(h.CodegenStart))
	mux.HandleFunc("/api/codegen/stream", wrap(h.CodegenStream))
	mux.HandleFunc("/api/codegen/code", wrap(h.CodegenCode))
	mux.HandleFunc("/api/scenarios", wrap(h.Scenarios))
	mux.HandleFunc("/api/scenarios/code", wrap(h.ScenarioCode))
	mux.HandleFunc("/api/scenarios/tags", wrap(h.ScenarioTags))
	mux.HandleFunc("/api/environments", wrap(h.Environments))
}

// chain は fn を mw... で包んだ http.HandlerFunc を返す。
// 合成順は Register のドキュメントの契約に従う(先頭の mw が最外側)。
func chain(fn http.HandlerFunc, mw ...Middleware) http.HandlerFunc {
	// 末尾から畳み込むことで、先頭の mw が最後に巻かれ最外側になる。
	// 例: chain(fn, CORS, RequireAuth) => CORS(RequireAuth(fn))
	for i := len(mw) - 1; i >= 0; i-- {
		fn = mw[i](fn)
	}
	return fn
}
