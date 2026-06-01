package handler

import "net/http"

// Register は runner の API ルートを mux に登録する。
//
// ルーティングを package main から切り出すことで、別バイナリ(例: 認証や
// マルチテナントを足す上位レイヤ)がこの API 面を import して再利用し、独自の
// ミドルウェアやルートを重ねられるようにする。main は import できないため、
// 再利用可能なルート定義はこの handler パッケージ側に置く。
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/tags", CORS(h.Tags))
	mux.HandleFunc("/api/run", CORS(h.Run))
	mux.HandleFunc("/api/stream", CORS(h.Stream))
	mux.HandleFunc("/api/codegen/start", CORS(h.CodegenStart))
	mux.HandleFunc("/api/codegen/stream", CORS(h.CodegenStream))
	mux.HandleFunc("/api/codegen/code", CORS(h.CodegenCode))
	mux.HandleFunc("/api/scenarios", CORS(h.Scenarios))
	mux.HandleFunc("/api/scenarios/tags", CORS(h.ScenarioTags))
}
