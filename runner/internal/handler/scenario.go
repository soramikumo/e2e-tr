package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"e2e-runner/internal/domain"
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

	case http.MethodPatch:
		oldName := domain.SanitizeName(r.URL.Query().Get("name"))
		newName := domain.SanitizeName(r.URL.Query().Get("to"))

		// 検証ガード（コストの軽い順に並べ、ディスク I/O 前に弾けるものは弾く）。
		// 1) 拡張子強制 — DELETE と同じく .spec.ts のみ許可する。oldName も検証し、
		//    フロントを介さない直叩きで非 spec ファイルを rename されるのを防ぐ。
		if !strings.HasSuffix(oldName, ".spec.ts") || !strings.HasSuffix(newName, ".spec.ts") {
			http.Error(w, "invalid name", http.StatusBadRequest)
			return
		}
		// 2) no-op — 同名 rename は想定外。衝突チェックより先に弾かないと、
		//    後段の os.Stat が「自分自身」を見つけて誤って 409 になる。
		if oldName == newName {
			http.Error(w, "same name", http.StatusBadRequest)
			return
		}

		oldPath := filepath.Join(h.cfg.TestsDir, "tests", oldName)
		newPath := filepath.Join(h.cfg.TestsDir, "tests", newName)
		// 3) 衝突 — os.Rename は既存ファイルを黙って上書きするため、自前で検知する。
		if _, err := os.Stat(newPath); err == nil {
			http.Error(w, "name already exists", http.StatusConflict)
			return
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		// ファイル rename が成功してからタグ割当を移す（順序が逆だと割当だけ
		// 動いてファイルは旧名のまま残り、タグ実行時に解決できなくなる）。
		if err := h.TagStore.RenameScenario(oldName, newName); err != nil {
			http.Error(w, "tag migration failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "name": newName})

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
