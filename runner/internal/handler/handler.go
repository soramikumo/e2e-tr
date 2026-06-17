package handler

import (
	"encoding/json"
	"net/http"

	"e2e-runner/internal/config"
	"e2e-runner/internal/executor"
	"e2e-runner/internal/store"
	"e2e-runner/internal/vnc"
)

type Handler struct {
	cfg          *config.Config
	RunStore     store.RunStore
	CodegenStore store.CodegenStore
	TagStore     *store.TagStore
	EnvStore     store.EnvironmentStore
	VNCManager   *vnc.Manager
	Executor     *executor.Executor
	sem          chan struct{}
}

func New(cfg *config.Config, rs store.RunStore, cs store.CodegenStore, vm *vnc.Manager) *Handler {
	return &Handler{
		cfg:          cfg,
		RunStore:     rs,
		CodegenStore: cs,
		TagStore:     store.NewTagStore(cfg.TestsDir),
		EnvStore:     store.NewFileEnvironmentStore(cfg.TestsDir),
		VNCManager:   vm,
		Executor:     executor.New(executor.OSRunner{}, cfg.TestsDir),
		sem:          make(chan struct{}, cfg.MaxConcurrentRuns),
	}
}

func sseStart(w http.ResponseWriter) (http.Flusher, bool) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, false
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	f.Flush()
	return f, true
}

func sseWrite(w http.ResponseWriter, f http.Flusher, v any) {
	data, _ := json.Marshal(v)
	w.Write([]byte("data: "))
	w.Write(data)
	w.Write([]byte("\n\n"))
	f.Flush()
}
