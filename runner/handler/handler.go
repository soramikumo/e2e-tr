package handler

import (
	"encoding/json"
	"net/http"

	"e2e-runner/config"
	"e2e-runner/store"
)

type Handler struct {
	cfg          *config.Config
	RunStore     store.RunStore
	CodegenStore store.CodegenStore
}

func New(cfg *config.Config, rs store.RunStore, cs store.CodegenStore) *Handler {
	return &Handler{cfg: cfg, RunStore: rs, CodegenStore: cs}
}

func sseStart(w http.ResponseWriter) (http.Flusher, bool) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, false
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	return f, true
}

func sseWrite(w http.ResponseWriter, f http.Flusher, v any) {
	data, _ := json.Marshal(v)
	w.Write([]byte("data: "))
	w.Write(data)
	w.Write([]byte("\n\n"))
	f.Flush()
}
