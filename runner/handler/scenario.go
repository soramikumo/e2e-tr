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
		scenarios := domain.ListScenarios(h.cfg.TestsDir)
		for i := range scenarios {
			scenarios[i].Tags = h.TagStore.TagsForScenario(scenarios[i].Name)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"scenarios": scenarios})

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
		// シナリオ削除に伴い、そのファイルへのタグ割当も取り除く。
		// ここで失敗すると spec は消えたのに割当だけ残り、タグ実行時に
		// 存在しないファイルを解決してしまうため、整合性のため 500 を返す。
		if err := h.TagStore.DropScenario(name); err != nil {
			http.Error(w, "tag cleanup failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
