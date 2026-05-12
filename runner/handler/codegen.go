package handler

import (
	"encoding/json"
	"net/http"
	"net/url"

	"e2e-runner/domain"
	"e2e-runner/executor"
)

func (h *Handler) CodegenStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}
	u, err := url.ParseRequestURI(body.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		http.Error(w, "valid http/https url is required", http.StatusBadRequest)
		return
	}
	if body.Name == "" {
		body.Name = "scenario-" + domain.RandomID()
	}

	c := domain.NewCodegen(body.URL, body.Name)

	if h.cfg.UseNoVNC {
		session, err := h.VNCManager.Start(c.ID)
		if err != nil {
			http.Error(w, "VNC起動失敗: "+err.Error(), http.StatusInternalServerError)
			return
		}
		c.NoVNCPort = session.NoVNCPort
	}

	h.CodegenStore.Save(c)
	go executor.ExecuteCodegen(c, h.cfg.TestsDir, h.CodegenStore, h.VNCManager)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"id": c.ID, "name": c.Name, "noVNCPort": c.NoVNCPort})
}

func (h *Handler) CodegenStream(w http.ResponseWriter, r *http.Request) {
	c, ok := h.CodegenStore.Get(r.URL.Query().Get("id"))
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	flusher, ok := sseStart(w)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch, cancel := c.Subscribe()
	defer cancel()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, open := <-ch:
			if !open {
				return
			}
			sseWrite(w, flusher, ev)
		}
	}
}
