package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"e2e-runner/config"
	"e2e-runner/handler"
	"e2e-runner/store"
)

func newEnvHandler(t *testing.T) *handler.Handler {
	t.Helper()
	cfg := &config.Config{TestsDir: t.TempDir(), Port: ":8080", MaxConcurrentRuns: 4}
	return handler.New(cfg, store.NewMemoryRunStore(), store.NewMemoryCodegenStore(), nil)
}

func TestEnvironments_CreateThenList_PasswordNotLeaked(t *testing.T) {
	h := newEnvHandler(t)

	// Create
	req := httptest.NewRequest(http.MethodPost, "/api/environments",
		strings.NewReader(`{"name":"dev","baseURL":"https://dev.example.com","basicAuthUser":"qa","basicAuthPass":"secret"}`))
	w := httptest.NewRecorder()
	h.Environments(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body=%s", w.Code, w.Body.String())
	}
	// 作成レスポンスにパスワードが含まれていないこと(View 経由で伏字化)。
	if strings.Contains(w.Body.String(), "secret") {
		t.Errorf("password leaked in create response: %s", w.Body.String())
	}

	// List
	req2 := httptest.NewRequest(http.MethodGet, "/api/environments", nil)
	w2 := httptest.NewRecorder()
	h.Environments(w2, req2)
	if strings.Contains(w2.Body.String(), "secret") {
		t.Errorf("password leaked in list response: %s", w2.Body.String())
	}
	var resp struct {
		Environments []map[string]any `json:"environments"`
	}
	json.NewDecoder(w2.Body).Decode(&resp)
	if len(resp.Environments) != 1 || resp.Environments[0]["hasAuthPass"] != true {
		t.Errorf("expected 1 env with hasAuthPass=true, got %+v", resp.Environments)
	}
}

func TestEnvironments_Create_InvalidURL_Rejected(t *testing.T) {
	h := newEnvHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/environments",
		strings.NewReader(`{"name":"dev","baseURL":"javascript:alert(1)"}`))
	w := httptest.NewRecorder()
	h.Environments(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestEnvironments_Create_DuplicateName_Conflict(t *testing.T) {
	h := newEnvHandler(t)
	body := `{"name":"dev","baseURL":"https://a.example.com"}`

	h.Environments(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/environments", strings.NewReader(body)))
	w := httptest.NewRecorder()
	h.Environments(w, httptest.NewRequest(http.MethodPost, "/api/environments", strings.NewReader(body)))
	if w.Code != http.StatusConflict {
		t.Errorf("dup status = %d, want 409", w.Code)
	}
}

func TestEnvironments_Delete(t *testing.T) {
	h := newEnvHandler(t)
	w := httptest.NewRecorder()
	h.Environments(w, httptest.NewRequest(http.MethodPost, "/api/environments",
		strings.NewReader(`{"name":"dev","baseURL":"https://a.example.com"}`)))
	var created map[string]string
	json.NewDecoder(w.Body).Decode(&created)

	wd := httptest.NewRecorder()
	h.Environments(wd, httptest.NewRequest(http.MethodDelete, "/api/environments?id="+created["id"], nil))
	if wd.Code != http.StatusOK {
		t.Fatalf("delete = %d", wd.Code)
	}
	envs, _ := h.EnvStore.List(context.Background())
	if len(envs) != 0 {
		t.Errorf("after delete, %d envs remain", len(envs))
	}
}

// /api/run が environmentId を受け取ると、その baseURL と認証情報が Run に解決される。
func TestHandleRun_WithEnvironmentID_ResolvesBaseURLAndAuth(t *testing.T) {
	h := newHandlerWithFakeRunner(t)
	// 事前に env を 1 件作っておく。
	wCre := httptest.NewRecorder()
	h.Environments(wCre, httptest.NewRequest(http.MethodPost, "/api/environments",
		strings.NewReader(`{"name":"staging","baseURL":"https://staging.example.com","basicAuthUser":"qa","basicAuthPass":"pw"}`)))
	var created map[string]string
	json.NewDecoder(wCre.Body).Decode(&created)

	req := httptest.NewRequest(http.MethodPost, "/api/run",
		strings.NewReader(`{"file":"a.spec.ts","environmentId":"`+created["id"]+`"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Run(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("run = %d, body=%s", w.Code, w.Body.String())
	}
	var body struct{ ID string }
	json.NewDecoder(w.Body).Decode(&body)
	run, ok := h.RunStore.Get(context.Background(), body.ID)
	if !ok {
		t.Fatal("run not found")
	}
	if run.BaseURL != "https://staging.example.com" {
		t.Errorf("baseURL = %q", run.BaseURL)
	}
	if run.AuthUser != "qa" || run.AuthPass != "pw" {
		t.Errorf("auth = %q/%q", run.AuthUser, run.AuthPass)
	}
}

func TestHandleRun_InvalidEnvironmentID_Rejected(t *testing.T) {
	h := newHandlerWithFakeRunner(t)
	req := httptest.NewRequest(http.MethodPost, "/api/run",
		strings.NewReader(`{"file":"a.spec.ts","environmentId":"env_ghost"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Run(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
