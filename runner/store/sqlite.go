package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
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
	// WAL + busy_timeout: レポート閲覧(読み)と run 完了(書き)の同時アクセスを
	// 滑らかにする。ローカル単一プロセス前提でも、SSE 配信中の読みと完了書き込みが
	// 重なるためロック競合を緩和しておく。
	for _, pragma := range []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA busy_timeout=5000`,
	} {
		if _, err := db.Exec(pragma); err != nil {
			return nil, fmt.Errorf("pragma %q: %w", pragma, err)
		}
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS runs (
		id          TEXT PRIMARY KEY,
		tag         TEXT,
		file        TEXT,
		files       TEXT NOT NULL DEFAULT '[]',
		base_url    TEXT NOT NULL DEFAULT '',
		trace       INTEGER NOT NULL DEFAULT 0,
		status      TEXT NOT NULL,
		logs        TEXT NOT NULL DEFAULT '[]',
		started_at  DATETIME NOT NULL,
		finished_at DATETIME
	)`)
	if err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}
	// 起動時の孤児クリーンアップ: プロセスが落ちて中断された run が status='running'
	// のまま残っていれば failed に倒す。再起動後はメモリの subscriber も無く再開
	// 不能なため、履歴の整合を保つための防御(通常は完了 run のみ永続化される)。
	if _, err := db.Exec(`UPDATE runs SET status = 'failed' WHERE status = 'running'`); err != nil {
		return nil, fmt.Errorf("orphan cleanup: %w", err)
	}
	return &SQLiteRunStore{db: db, active: map[string]*domain.Run{}}, nil
}

// Save はRunをメモリに登録し、完了済みの場合はSQLiteにも保存する。
func (s *SQLiteRunStore) Save(ctx context.Context, run *domain.Run) error {
	s.mu.Lock()
	s.active[run.ID] = run
	s.mu.Unlock()

	if run.GetStatus() == domain.StatusRunning {
		return nil
	}

	logsJSON, _ := json.Marshal(run.Logs())
	filesJSON, _ := json.Marshal(run.Files)
	trace := 0
	if run.Trace {
		trace = 1
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO runs (id, tag, file, files, base_url, trace, status, logs, started_at, finished_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.Tag, run.File, string(filesJSON), run.BaseURL, trace,
		string(run.GetStatus()), string(logsJSON), run.StartedAt, run.FinishedAt,
	)
	return err
}

func (s *SQLiteRunStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	delete(s.active, id)
	s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `DELETE FROM runs WHERE id = ?`, id)
	return err
}

func (s *SQLiteRunStore) Get(ctx context.Context, id string) (*domain.Run, bool) {
	s.mu.RLock()
	r, ok := s.active[id]
	s.mu.RUnlock()
	if ok {
		return r, true
	}
	return s.getFromDB(ctx, id)
}

// List は実行履歴を新しい順で返す。アクティブ(メモリ)を優先しつつ、DB の完了 run と
// マージする(同一 id はメモリ側がライブで新しいため優先)。
func (s *SQLiteRunStore) List(ctx context.Context) ([]*domain.Run, error) {
	s.mu.RLock()
	out := make([]*domain.Run, 0, len(s.active))
	seen := make(map[string]bool, len(s.active))
	for _, r := range s.active {
		out = append(out, r)
		seen[r.ID] = true
	}
	s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tag, file, files, base_url, trace, status, logs, started_at, finished_at
		 FROM runs ORDER BY started_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		if seen[r.ID] {
			continue
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	if len(out) > 200 {
		out = out[:200]
	}
	return out, nil
}

func (s *SQLiteRunStore) getFromDB(ctx context.Context, id string) (*domain.Run, bool) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, tag, file, files, base_url, trace, status, logs, started_at, finished_at
		 FROM runs WHERE id = ?`, id)
	r, err := scanRun(row)
	if err != nil {
		return nil, false
	}
	return r, true
}

// scanner は *sql.Row と *sql.Rows の共通 Scan を抽象化する(getFromDB と List で共用)。
type scanner interface {
	Scan(dest ...any) error
}

// scanRun は runs テーブルの1行を domain.Run へ復元する。
func scanRun(sc scanner) (*domain.Run, error) {
	var (
		id, tag, file, status string
		filesJSON, baseURL    string
		logsJSON              string
		trace                 int
		startedAt             time.Time
		finishedAt            sql.NullTime
	)
	if err := sc.Scan(&id, &tag, &file, &filesJSON, &baseURL, &trace,
		&status, &logsJSON, &startedAt, &finishedAt); err != nil {
		return nil, err
	}
	var files []string
	if filesJSON != "" {
		json.Unmarshal([]byte(filesJSON), &files)
	}
	var logs []string
	json.Unmarshal([]byte(logsJSON), &logs)
	var finished time.Time
	if finishedAt.Valid {
		finished = finishedAt.Time
	}
	return domain.LoadRun(id, tag, file, files, baseURL, trace != 0,
		domain.RunStatus(status), startedAt, finished, logs), nil
}
