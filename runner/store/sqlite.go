package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"e2e-runner/domain"

	_ "modernc.org/sqlite"
)

// SQLiteRunStore はアクティブなRunをメモリに、完了したRunをSQLiteに保持する。
type SQLiteRunStore struct {
	db     *sql.DB
	mu     sync.RWMutex
	active map[string]*domain.Run
}

func NewSQLiteRunStore(dbPath string) (*SQLiteRunStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS runs (
		id         TEXT PRIMARY KEY,
		tag        TEXT,
		file       TEXT,
		status     TEXT NOT NULL,
		logs       TEXT NOT NULL DEFAULT '[]',
		started_at DATETIME NOT NULL
	)`)
	if err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}
	return &SQLiteRunStore{db: db, active: map[string]*domain.Run{}}, nil
}

// Save はRunをメモリに登録し、完了済みの場合はSQLiteにも保存する。
func (s *SQLiteRunStore) Save(run *domain.Run) error {
	s.mu.Lock()
	s.active[run.ID] = run
	s.mu.Unlock()

	if run.GetStatus() == domain.StatusRunning {
		return nil
	}

	logsJSON, _ := json.Marshal(run.Logs())

	// TODO(human): INSERT OR REPLACE INTO runs (id, tag, file, status, logs, started_at)
	// VALUES (?, ?, ?, ?, ?, ?) を使って run を SQLite に保存する。
	// s.db.Exec(sql, args...) の形で書く。エラーがあれば return err する。
	_ = logsJSON
	return nil
}

func (s *SQLiteRunStore) Get(id string) (*domain.Run, bool) {
	s.mu.RLock()
	r, ok := s.active[id]
	s.mu.RUnlock()
	if ok {
		return r, true
	}
	return s.getFromDB(id)
}

func (s *SQLiteRunStore) getFromDB(id string) (*domain.Run, bool) {
	var (
		tag, file, status string
		logsJSON          string
		startedAt         time.Time
	)
	err := s.db.QueryRow(
		`SELECT tag, file, status, logs, started_at FROM runs WHERE id = ?`, id,
	).Scan(&tag, &file, &status, &logsJSON, &startedAt)
	if err != nil {
		return nil, false
	}
	var logs []string
	json.Unmarshal([]byte(logsJSON), &logs)
	return domain.LoadRun(id, tag, file, domain.RunStatus(status), startedAt, logs), true
}
