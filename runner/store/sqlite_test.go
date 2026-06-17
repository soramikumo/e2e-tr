package store_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"e2e-runner/domain"
	"e2e-runner/store"
)

// newStore は t.TempDir() 配下にDBファイルを作成してストアを返す。
func newStore(t *testing.T) *store.SQLiteRunStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "runs.db")
	s, err := store.NewSQLiteRunStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteRunStore: %v", err)
	}
	return s
}

// TestSaveGet_Active は Save → Get でアクティブなRunが取得できることを確認する。
func TestSaveGet_Active(t *testing.T) {
	s := newStore(t)

	run := domain.NewRun("tag1", "scenario.yaml")
	if err := s.Save(context.Background(), run); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, ok := s.Get(context.Background(), run.ID)
	if !ok {
		t.Fatal("Get returned false for active run")
	}
	if got.ID != run.ID {
		t.Errorf("ID mismatch: got %q, want %q", got.ID, run.ID)
	}
	if got.GetStatus() != domain.StatusRunning {
		t.Errorf("Status mismatch: got %q, want %q", got.GetStatus(), domain.StatusRunning)
	}
}

// TestSaveGet_FinishedPersisted は Run が Finish した後に Save し、
// 新しいストアインスタンスで Get するとDBから復元できることを確認する（サーバー再起動相当）。
func TestSaveGet_FinishedPersisted(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "runs.db")

	run := domain.NewRun("tag2", "scenario2.yaml")
	run.AddLog("line1")
	run.AddLog("line2")
	run.Finish(true)

	// 第1ストア: Save する。
	s1, err := store.NewSQLiteRunStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteRunStore (s1): %v", err)
	}
	if err := s1.Save(context.Background(), run); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// 第2ストア: 別インスタンスで Get する（サーバー再起動相当）。
	s2, err := store.NewSQLiteRunStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteRunStore (s2): %v", err)
	}
	got, ok := s2.Get(context.Background(), run.ID)
	if !ok {
		t.Fatal("Get returned false after restart — DB persistence failed")
	}
	if got.GetStatus() != domain.StatusDone {
		t.Errorf("Status mismatch: got %q, want %q", got.GetStatus(), domain.StatusDone)
	}
	if got.Tag != run.Tag {
		t.Errorf("Tag mismatch: got %q, want %q", got.Tag, run.Tag)
	}
	if got.File != run.File {
		t.Errorf("File mismatch: got %q, want %q", got.File, run.File)
	}
	logs := got.Logs()
	if len(logs) != 2 || logs[0] != "line1" || logs[1] != "line2" {
		t.Errorf("Logs mismatch: got %v", logs)
	}
}

func TestList_DoesNotLoadLogs(t *testing.T) {
	s := newStore(t)

	run := domain.NewRun("tag-list", "list.yaml")
	run.AddLog("heavy log line")
	run.Finish(true)
	if err := s.Save(context.Background(), run); err != nil {
		t.Fatalf("Save: %v", err)
	}

	runs, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("len(runs) = %d, want 1", len(runs))
	}
	if logs := runs[0].Logs(); len(logs) != 0 {
		t.Fatalf("List loaded logs = %v, want none", logs)
	}

	got, ok := s.Get(context.Background(), run.ID)
	if !ok {
		t.Fatal("Get returned false")
	}
	if logs := got.Logs(); len(logs) != 1 || logs[0] != "heavy log line" {
		t.Fatalf("Get logs = %v, want persisted log", logs)
	}
}

func TestSave_FinishedRemovesActiveEntry(t *testing.T) {
	s := newStore(t)

	run := domain.NewRun("tag-finished", "finished.yaml")
	if err := s.Save(context.Background(), run); err != nil {
		t.Fatalf("Save running: %v", err)
	}
	run.Finish(true)
	if err := s.Save(context.Background(), run); err != nil {
		t.Fatalf("Save finished: %v", err)
	}

	runs, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("len(runs) = %d, want 1", len(runs))
	}
	if runs[0].ID != run.ID {
		t.Errorf("ID = %q, want %q", runs[0].ID, run.ID)
	}
}

// TestGet_NotFound は存在しないIDを Get したとき false が返ることを確認する。
func TestGet_NotFound(t *testing.T) {
	s := newStore(t)

	_, ok := s.Get(context.Background(), "nonexistent-id")
	if ok {
		t.Fatal("Get returned true for nonexistent ID")
	}
}

// TestSave_RunningNotPersistedToDB は status が running の Run はDBに書き込まれず、
// 別ストアインスタンスからは取得できないことを確認する（メモリのみ）。
func TestSave_RunningNotPersistedToDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "runs.db")

	run := domain.NewRun("tag3", "scenario3.yaml")
	// Finish しない → StatusRunning のまま

	s1, err := store.NewSQLiteRunStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteRunStore (s1): %v", err)
	}
	if err := s1.Save(context.Background(), run); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// 別インスタンス（DBのみ参照）では取得できないはず。
	s2, err := store.NewSQLiteRunStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteRunStore (s2): %v", err)
	}
	_, ok := s2.Get(context.Background(), run.ID)
	if ok {
		t.Fatal("running run should NOT be persisted to DB — only in memory")
	}
}

// TestNewSQLiteRunStore_MigratesOldRunsSchema は古い runs テーブルに対して
// 新しい列を追加し、その後の List/Save が失敗しないことを確認する。
func TestNewSQLiteRunStore_MigratesOldRunsSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "runs.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE runs (
		id         TEXT PRIMARY KEY,
		tag        TEXT,
		file       TEXT,
		status     TEXT NOT NULL,
		logs       TEXT NOT NULL DEFAULT '[]',
		started_at DATETIME NOT NULL
	)`); err != nil {
		t.Fatalf("create old runs schema: %v", err)
	}
	startedAt := time.Now()
	if _, err := db.Exec(
		`INSERT INTO runs (id, tag, file, status, logs, started_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"old-run", "tag-old", "old.yaml", string(domain.StatusDone), `["old log"]`, startedAt,
	); err != nil {
		t.Fatalf("insert old run: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close old db: %v", err)
	}

	s, err := store.NewSQLiteRunStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteRunStore: %v", err)
	}
	runs, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List after migration: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("len(runs) = %d, want 1", len(runs))
	}
	if runs[0].ID != "old-run" {
		t.Errorf("ID = %q, want old-run", runs[0].ID)
	}
	if runs[0].GetStatus() != domain.StatusDone {
		t.Errorf("status = %q, want %q", runs[0].GetStatus(), domain.StatusDone)
	}

	newRun := domain.NewRun("tag-new", "new.yaml")
	newRun.Files = []string{"new.yaml"}
	newRun.BaseURL = "https://example.test"
	newRun.Trace = true
	newRun.Finish(true)
	if err := s.Save(context.Background(), newRun); err != nil {
		t.Fatalf("Save after migration: %v", err)
	}
}
