package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"e2e-runner/internal/domain"
	"e2e-runner/internal/store"
)

// Environments は environment の CRUD を 1 ハンドラで多重化する。
// /api/scenarios と同じ規約(query string で id、メソッドで動作分岐)に揃える。
//
//	GET    /api/environments              一覧(View=パスワード伏字)
//	POST   /api/environments              作成 body: {name, baseURL, basicAuthUser?, basicAuthPass?}
//	PATCH  /api/environments?id=...       更新(全置換)
//	DELETE /api/environments?id=...       削除
func (h *Handler) Environments(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		envs, err := h.EnvStore.List(r.Context())
		if err != nil {
			http.Error(w, "list failed", http.StatusInternalServerError)
			return
		}
		views := make([]domain.EnvironmentView, len(envs))
		for i := range envs {
			views[i] = envs[i].View()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"environments": views})

	case http.MethodPost:
		var body struct {
			Name          string `json:"name"`
			BaseURL       string `json:"baseURL"`
			BasicAuthUser string `json:"basicAuthUser"`
			BasicAuthPass string `json:"basicAuthPass"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if err := validateEnvInput(body.Name, body.BaseURL); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		now := time.Now().UTC()
		env := &domain.Environment{
			ID:            "env_" + domain.RandomID(),
			Name:          strings.TrimSpace(body.Name),
			BaseURL:       strings.TrimSpace(body.BaseURL),
			BasicAuthUser: body.BasicAuthUser,
			BasicAuthPass: body.BasicAuthPass,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := h.EnvStore.Create(r.Context(), env); err != nil {
			if errors.Is(err, store.ErrEnvNameTaken) {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			http.Error(w, "create failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		view := env.View()
		json.NewEncoder(w).Encode(view)

	case http.MethodPatch:
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "id required", http.StatusBadRequest)
			return
		}
		existing, ok := h.EnvStore.Get(r.Context(), id)
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var body struct {
			Name          string `json:"name"`
			BaseURL       string `json:"baseURL"`
			BasicAuthUser string `json:"basicAuthUser"`
			// パスワードは「未指定 = 触らない / 空文字 = クリア / 値あり = 上書き」を
			// 区別したいので *string で受ける。omitempty では null を作れず常に上書きに
			// なるため、明示的に nullable で扱う。
			BasicAuthPass *string `json:"basicAuthPass,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if err := validateEnvInput(body.Name, body.BaseURL); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		existing.Name = strings.TrimSpace(body.Name)
		existing.BaseURL = strings.TrimSpace(body.BaseURL)
		existing.BasicAuthUser = body.BasicAuthUser
		if body.BasicAuthPass != nil {
			existing.BasicAuthPass = *body.BasicAuthPass
		}
		existing.UpdatedAt = time.Now().UTC()
		if err := h.EnvStore.Update(r.Context(), existing); err != nil {
			if errors.Is(err, store.ErrEnvNameTaken) {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			http.Error(w, "update failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(existing.View())

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "id required", http.StatusBadRequest)
			return
		}
		if err := h.EnvStore.Delete(r.Context(), id); err != nil {
			if errors.Is(err, store.ErrEnvNotFound) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			http.Error(w, "delete failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// validateEnvInput は名前と URL の最低限の検証。空・不正 scheme をここで弾き、
// store/exec 層に変な値が流れないようにする。
func validateEnvInput(name, baseURL string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("name required")
	}
	if strings.TrimSpace(baseURL) == "" {
		return errors.New("baseURL required")
	}
	u, err := url.ParseRequestURI(strings.TrimSpace(baseURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("invalid baseURL (must be http/https)")
	}
	return nil
}
