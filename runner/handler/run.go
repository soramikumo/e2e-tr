package handler

import (
	"encoding/json"
	"net/http"

	"e2e-runner/domain"
	"e2e-runner/executor"
)

func (h *Handler) Tags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"tags": domain.ScanTags(h.cfg.TestsDir)})
}

func (h *Handler) Run(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Tag  string `json:"tag"`
		File string `json:"file"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if body.Tag == "" && body.File == "" {
		http.Error(w, "tag or file required", http.StatusBadRequest)
		return
	}
	run := domain.NewRun(body.Tag, body.File)
	h.RunStore.Save(run)
	select {
	case h.sem <- struct{}{}:
		go func() {
			defer func() { <-h.sem }()
			executor.ExecuteTest(run, h.cfg.TestsDir, h.RunStore, h.cfg.RunTimeout)
		}()
	default:
		http.Error(w, "too many concurrent runs", http.StatusTooManyRequests)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": run.ID})
}

func (h *Handler) Stream(w http.ResponseWriter, r *http.Request) {
	run, ok := h.RunStore.Get(r.URL.Query().Get("id"))
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	flusher, ok := sseStart(w)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch, cancel := run.Subscribe()
	defer cancel()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case line, open := <-ch:
			if !open {
				sseWrite(w, flusher, map[string]string{"type": "done", "status": string(run.GetStatus())})
				return
			}
			sseWrite(w, flusher, map[string]string{"type": "log", "message": line})
		}
	}
}
