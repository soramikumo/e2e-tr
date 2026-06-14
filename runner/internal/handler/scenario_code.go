package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"e2e-runner/internal/domain"
)

// 1MB。手書き/録画の spec として現実的な上限。これを超える body は弾き、
// 巨大ペイロードでメモリを食わせる事故を防ぐ。
const maxSpecBytes = 1 << 20

// ScenarioCode は既存シナリオ(.spec.ts)のソースを読み書きする。
//
//	GET  /api/scenarios/code?name=foo.spec.ts -> {"name","code"}
//	PUT  /api/scenarios/code?name=foo.spec.ts   body {"code": "..."}
//
// dev/prod の blue/green では「その場でソースを直せる」ことが要件なので、
// codegen 経由(記録セッション ID 起点の CodegenCode)とは別に、保存済みファイルを
// 名前で読み書きする経路を用意する。対象は既存ファイルに限定し、新規作成や
// 非 spec ファイルへの書き込みは許可しない(rename/delete と同じ拡張子ガード)。
func (h *Handler) ScenarioCode(w http.ResponseWriter, r *http.Request) {
	name := domain.SanitizeName(r.URL.Query().Get("name"))
	if !strings.HasSuffix(name, ".spec.ts") {
		http.Error(w, "invalid name", http.StatusBadRequest)
		return
	}
	path := filepath.Join(h.cfg.TestsDir, "tests", name)

	switch r.Method {
	case http.MethodGet:
		b, err := os.ReadFile(path)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"name": name, "code": string(b)})

	case http.MethodPut:
		// 既存ファイルのみ編集可。存在しない名前への PUT で新規作成させない
		// (一覧に出ない孤児ファイルや、任意名でのファイル生成を防ぐ)。
		if _, err := os.Stat(path); err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var body struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, maxSpecBytes)).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if err := os.WriteFile(path, []byte(body.Code), 0o644); err != nil {
			http.Error(w, "write failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "name": name})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
