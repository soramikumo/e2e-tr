package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"e2e-runner/domain"
)

func (h *Handler) Scenarios(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"scenarios": domain.ListScenarios(h.cfg.TestsDir)})

	case http.MethodDelete:
		name := domain.SanitizeName(r.URL.Query().Get("name"))
		if !strings.HasSuffix(name, ".spec.ts") {
			http.Error(w, "invalid name", http.StatusBadRequest)
			return
		}
		path := filepath.Join(h.cfg.TestsDir, "tests", name)
		if err := os.Remove(path); err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
