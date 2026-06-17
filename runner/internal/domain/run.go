package domain

import (
	"crypto/rand"
	"encoding/hex"
	"slices"
	"sync"
	"time"
)

type RunStatus string

const (
	StatusRunning RunStatus = "running"
	StatusDone    RunStatus = "done"
	StatusFailed  RunStatus = "failed"
)

type Run struct {
	ID         string    `json:"id"`
	Tag        string    `json:"tag,omitempty"`
	File       string    `json:"file,omitempty"`
	Files      []string  `json:"files,omitempty"`    // タグ実行で解決した複数シナリオ
	Trace      bool      `json:"trace,omitempty"`    // true なら成功時も trace を保存(--trace on)
	BaseURL    string    `json:"base_url,omitempty"` // 非空なら PLAYWRIGHT_BASE_URL として実行時に注入(dev/prod 切替)
	AuthUser   string    `json:"-"`                  // Basic Auth ユーザー名(Environment 経由のみ設定、JSON 露出しない)
	AuthPass   string    `json:"-"`                  // Basic Auth パスワード(同上)
	Status     RunStatus `json:"status"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at,omitzero"` // 完了時刻。未完了(running)はゼロ値なので省かれる

	mu   sync.RWMutex
	logs []string
	subs []chan string
	done bool
}

func NewRun(tag, file string) *Run {
	return &Run{
		ID:        RandomID(),
		Tag:       tag,
		File:      file,
		Status:    StatusRunning,
		StartedAt: time.Now(),
	}
}

// 永続化からの復元は files/baseURL/trace/finishedAt 含めて元の Run を再構成する。履歴一覧(List)やレポート参照で「どのファイルを・どこに対して走らせ・いつ終わったか」を表示するため、全フィールドを引き回す。
// AuthUser/AuthPass は JSON 露出させない秘匿情報なので永続化・復元しない。
func LoadRun(id, tag, file string, files []string, baseURL string, trace bool, status RunStatus, startedAt, finishedAt time.Time, logs []string) *Run {
	return &Run{
		ID:         id,
		Tag:        tag,
		File:       file,
		Files:      files,
		BaseURL:    baseURL,
		Trace:      trace,
		Status:     status,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		logs:       logs,
		done:       status != StatusRunning,
	}
}

func (r *Run) AddLog(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, line)
	for _, ch := range r.subs {
		select {
		case ch <- line:
		default:
		}
	}
}

func (r *Run) Subscribe() (<-chan string, func()) {
	ch := make(chan string, 512)
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, line := range r.logs {
		ch <- line
	}
	if r.done {
		close(ch)
		return ch, func() {}
	}
	r.subs = append(r.subs, ch)
	cancel := func() { r.unsubscribe(ch) }
	return ch, cancel
}

func (r *Run) unsubscribe(ch chan string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, sub := range r.subs {
		if sub == ch {
			r.subs = slices.Delete(r.subs, i, i+1)
			return
		}
	}
}

func (r *Run) Finish(success bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if success {
		r.Status = StatusDone
	} else {
		r.Status = StatusFailed
	}
	r.FinishedAt = time.Now()
	r.done = true
	for _, ch := range r.subs {
		close(ch)
	}
	r.subs = nil
}

func (r *Run) GetStatus() RunStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Status
}

func (r *Run) Logs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cp := make([]string, len(r.logs))
	copy(cp, r.logs)
	return cp
}

func RandomID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}
