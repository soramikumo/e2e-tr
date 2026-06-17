package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"e2e-runner/domain"
)

// Runs は実行履歴の一覧を JSON で返す(GET /api/runs)。新しい順。
// ログ全文は重いので一覧には含めない ── 個別ログは /api/stream?id= が完了 run でも
// 蓄積ログをリプレイして返すため、フロントはそちらで取得する。
func (h *Handler) Runs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	runs, err := h.RunStore.List(r.Context())
	if err != nil {
		http.Error(w, "list failed", http.StatusInternalServerError)
		return
	}
	type summary struct {
		ID         string           `json:"id"`
		Tag        string           `json:"tag,omitempty"`
		File       string           `json:"file,omitempty"`
		Files      []string         `json:"files,omitempty"`
		Status     domain.RunStatus `json:"status"`
		StartedAt  time.Time        `json:"started_at"`
		FinishedAt time.Time        `json:"finished_at,omitzero"`
	}
	out := make([]summary, 0, len(runs))
	for _, run := range runs {
		out = append(out, summary{
			ID:         run.ID,
			Tag:        run.Tag,
			File:       run.File,
			Files:      run.Files,
			Status:     run.GetStatus(),
			StartedAt:  run.StartedAt,
			FinishedAt: run.FinishedAt,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"runs": out})
}

func (h *Handler) Run(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Tag           string `json:"tag"`
		File          string `json:"file"`
		Trace         bool   `json:"trace"`
		BaseURL       string `json:"baseURL"`       // 後方互換: 直入力(PR #81 の経路)
		EnvironmentID string `json:"environmentId"` // 推奨: Environment を id で参照
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
	// environmentId 指定があれば優先(直入力 baseURL より強い)。Environment は
	// 名前付き resource として認証情報も束ねる ── 解決して run に埋めることで、
	// 以降のレイヤは「Environment という概念」を知らずに済む(executor は env と
	// 認証情報をそのまま注入するだけ)。
	if body.EnvironmentID != "" {
		env, ok := h.EnvStore.Get(r.Context(), body.EnvironmentID)
		if !ok {
			http.Error(w, "environment not found", http.StatusBadRequest)
			return
		}
		run.BaseURL = env.BaseURL
		run.AuthUser = env.BasicAuthUser
		run.AuthPass = env.BasicAuthPass
	} else if body.BaseURL != "" {
		// 後方互換経路: 指定時のみ http/https を検証する(不正値をそのまま env に
		// 流すと実行が黙って意図しない先を叩くため、入口で弾く)。
		u, err := url.ParseRequestURI(body.BaseURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			http.Error(w, "invalid baseURL", http.StatusBadRequest)
			return
		}
		run.BaseURL = body.BaseURL
	}
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
