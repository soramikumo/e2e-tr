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
	ID        string    `json:"id"`
	Tag       string    `json:"tag,omitempty"`
	File      string    `json:"file,omitempty"`
	Files     []string  `json:"files,omitempty"`    // タグ実行で解決した複数シナリオ
	Trace     bool      `json:"trace,omitempty"`    // true なら成功時も trace を保存(--trace on)
	BaseURL   string    `json:"base_url,omitempty"`     // 非空なら PLAYWRIGHT_BASE_URL として実行時に注入(dev/prod 切替)
	AuthUser  string    `json:"-"`                      // Basic Auth ユーザー名(Environment 経由のみ設定、JSON 露出しない)
	AuthPass  string    `json:"-"`                      // Basic Auth パスワード(同上)
	Status    RunStatus `json:"status"`
	StartedAt time.Time `json:"started_at"`

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

// LoadRun reconstructs a Run from persisted data without triggering pub/sub.
func LoadRun(id, tag, file string, status RunStatus, startedAt time.Time, logs []string) *Run {
	return &Run{
		ID:        id,
		Tag:       tag,
		File:      file,
		Status:    status,
		StartedAt: startedAt,
		logs:      logs,
		done:      status != StatusRunning,
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
