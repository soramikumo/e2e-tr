package store_test

import (
	"context"
	"path/filepath"
	"testing"

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
