package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Environment は「実行先」を表す名前付き設定。dev/staging/prod 等を保存して
// 実行時に id で選ばせるための一級概念。PR #81 の baseURL 直入力を resource へ格上げ。
//
// パスワードはファイルに平文で書く(OSS=ローカル前提、暗号化なし)。
// API レスポンスでは EnvironmentView を介して伏字化する(平文を JSON 経由で
// portal/proxy に漏らさないため)。
type Environment struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	BaseURL       string    `json:"baseURL"`
	BasicAuthUser string    `json:"basicAuthUser,omitempty"`
	BasicAuthPass string    `json:"basicAuthPass,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// EnvironmentView は API レスポンス用。パスワードを「設定済みかどうか」だけに丸める。
// これにより一覧 GET でパスワード平文が leak しない。
type EnvironmentView struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	BaseURL       string    `json:"baseURL"`
	BasicAuthUser string    `json:"basicAuthUser,omitempty"`
	HasAuthPass   bool      `json:"hasAuthPass"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (e *Environment) View() EnvironmentView {
	return EnvironmentView{
		ID:            e.ID,
		Name:          e.Name,
		BaseURL:       e.BaseURL,
		BasicAuthUser: e.BasicAuthUser,
		HasAuthPass:   e.BasicAuthPass != "",
		CreatedAt:     e.CreatedAt,
		UpdatedAt:     e.UpdatedAt,
	}
}

// EnvironmentMeta はファイル永続化の最上位構造。TagMeta と同じ流儀で、
// 全件を 1 ファイル(.environments.json)に丸ごと書く read-modify-write。
type EnvironmentMeta struct {
	Environments []Environment `json:"environments"`
}

func envMetaPath(testsDir string) string {
	return filepath.Join(testsDir, ".environments.json")
}

// LoadEnvMeta は .environments.json を読み込む。未生成なら空メタを返す。
func LoadEnvMeta(testsDir string) *EnvironmentMeta {
	data, err := os.ReadFile(envMetaPath(testsDir))
	if err == nil {
		var meta EnvironmentMeta
		if json.Unmarshal(data, &meta) == nil {
			if meta.Environments == nil {
				meta.Environments = []Environment{}
			}
			return &meta
		}
	}
	return &EnvironmentMeta{Environments: []Environment{}}
}

// SaveEnvMeta は .environments.json へ整形して書き出す。
// パーミッションは 0600 ── basic auth パスワードを平文で含むため、他ユーザーから
// 読まれないようにする(OSS=ローカル前提、暗号化はしない代わりにこれだけは守る)。
func SaveEnvMeta(testsDir string, meta *EnvironmentMeta) error {
	sort.Slice(meta.Environments, func(i, j int) bool {
		return meta.Environments[i].Name < meta.Environments[j].Name
	})
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(envMetaPath(testsDir), data, 0o600)
}
