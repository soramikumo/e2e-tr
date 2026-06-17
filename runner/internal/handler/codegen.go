package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"e2e-runner/internal/domain"
	"e2e-runner/internal/executor"
	"e2e-runner/internal/vnc"
)

// vncSessions は *vnc.Manager を VNCSessions インターフェースへ安全に変換する。
// nil の *vnc.Manager をそのままインターフェースへ代入すると「型あり・値 nil」の
// 非 nil インターフェースになり、ExecuteCodegen 内の nil ガードをすり抜けて nil
// レシーバ呼び出しで panic する。nil は型なし nil インターフェースとして返す。
func vncSessions(m *vnc.Manager) executor.VNCSessions {
	if m == nil {
		return nil
	}
	return m
}

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
	go h.Executor.ExecuteCodegen(c, h.CodegenStore, vncSessions(h.VNCManager))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"id": c.ID, "name": c.Name, "noVNCPort": c.NoVNCPort})
}

// CodegenCode は記録中セッションの spec ファイル内容を返す。
// Inspector をオフスクリーンに追い出した代わりに、生成コードをポータルで見せる。
// codegen は --output を逐次書くため、記録中はポーリングでライブ更新できる。
// 記録開始直後でまだファイルが無い場合は空コードで 200 を返す(ポーリング側を単純化)。
func (h *Handler) CodegenCode(w http.ResponseWriter, r *http.Request) {
	c, ok := h.CodegenStore.Get(r.URL.Query().Get("id"))
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	path := filepath.Join(h.cfg.TestsDir, "tests", domain.SanitizeName(c.Name)+".spec.ts")
	code := ""
	if b, err := os.ReadFile(path); err == nil {
		code = string(b)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"code": code, "status": c.Status})
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
