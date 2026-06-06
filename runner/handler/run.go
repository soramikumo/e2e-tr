package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"e2e-runner/domain"
)

func (h *Handler) Run(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Tag     string `json:"tag"`
		File    string `json:"file"`
		Trace   bool   `json:"trace"`
		BaseURL string `json:"baseURL"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if body.Tag == "" && body.File == "" {
		http.Error(w, "tag or file required", http.StatusBadRequest)
		return
	}
	// baseURL は任意。指定時のみ http/https を検証する(不正値をそのまま env に
	// 流すと実行が黙って意図しない先を叩くため、入口で弾く)。
	if body.BaseURL != "" {
		u, err := url.ParseRequestURI(body.BaseURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			http.Error(w, "invalid baseURL", http.StatusBadRequest)
			return
		}
	}
	run := domain.NewRun(body.Tag, body.File)
	run.BaseURL = body.BaseURL
	// タグ実行はメタデータ主導: そのタグが貼られたシナリオ群を解決して複数ファイルを走らせる。
	if body.Tag != "" {
		files := h.TagStore.ScenariosForTag(body.Tag)
		if len(files) == 0 {
			http.Error(w, "no scenarios for tag", http.StatusBadRequest)
			return
		}
		run.Files = files
	}
	run.Trace = body.Trace
	h.RunStore.Save(r.Context(), run)

	// 実行はリクエストを超えて生きる goroutine で行う。r.Context() はレスポンス
	// 返却時にキャンセルされるため、そのまま渡すと実行途中で打ち切られる。
	// WithoutCancel で「キャンセルは切り離すが、載った値(owner_id 等)は残す」
	// context を作って渡す。
	bgCtx := context.WithoutCancel(r.Context())
	select {
	case h.sem <- struct{}{}:
		go func() {
			defer func() { <-h.sem }()
			h.Executor.ExecuteTest(bgCtx, run, h.RunStore, h.cfg.RunTimeout)
		}()
	default:
		h.RunStore.Delete(r.Context(), run.ID)
		http.Error(w, "too many concurrent runs", http.StatusTooManyRequests)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": run.ID})
}

func (h *Handler) Stream(w http.ResponseWriter, r *http.Request) {
	run, ok := h.RunStore.Get(r.Context(), r.URL.Query().Get("id"))
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
