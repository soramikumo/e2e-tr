package handler

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"e2e-runner/internal/domain"
)

var hexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Tags は GET=一覧 / POST=作成・更新 / DELETE=削除(割当もカスケード) を扱う。
func (h *Handler) Tags(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{"tags": h.TagStore.List()})

	case http.MethodPost:
		var body struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(body.Name)
		if name == "" {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		if !hexColorRe.MatchString(body.Color) {
			http.Error(w, "color must be #rrggbb", http.StatusBadRequest)
			return
		}
		if err := h.TagStore.UpsertTag(name, body.Color); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"tags": h.TagStore.List()})

	case http.MethodDelete:
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		if err := h.TagStore.DeleteTag(name); err != nil {
			http.Error(w, "delete failed", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"tags": h.TagStore.List()})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ScenarioTags は PUT で指定シナリオのタグ割当を丸ごと置き換える。
func (h *Handler) ScenarioTags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Scenario string   `json:"scenario"`
		Tags     []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	scenario := domain.SanitizeName(body.Scenario)
	if !strings.HasSuffix(scenario, ".spec.ts") {
		http.Error(w, "invalid scenario", http.StatusBadRequest)
		return
	}
	if err := h.TagStore.SetScenarioTags(scenario, body.Tags); err != nil {
		http.Error(w, "save failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"tags": h.TagStore.TagsForScenario(scenario)})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
