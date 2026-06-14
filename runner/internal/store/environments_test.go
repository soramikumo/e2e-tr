package store_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"e2e-runner/internal/domain"
	"e2e-runner/internal/store"
)

func newEnv(name, url string) *domain.Environment {
	now := time.Now().UTC()
	return &domain.Environment{
		ID: "env_" + name, Name: name, BaseURL: url, CreatedAt: now, UpdatedAt: now,
	}
}

func TestFileEnvStore_CreateListGet(t *testing.T) {
	dir := t.TempDir()
	s := store.NewFileEnvironmentStore(dir)
	ctx := context.Background()

	if err := s.Create(ctx, newEnv("dev", "https://dev.example.com")); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Create(ctx, newEnv("prod", "https://app.example.com")); err != nil {
		t.Fatalf("create2: %v", err)
	}

	envs, err := s.List(ctx)
	if err != nil || len(envs) != 2 {
		t.Fatalf("list = %d envs, err=%v, want 2", len(envs), err)
	}

	got, ok := s.Get(ctx, "env_dev")
	if !ok || got.BaseURL != "https://dev.example.com" {
		t.Errorf("Get(env_dev) = %+v, ok=%v", got, ok)
	}
}

func TestFileEnvStore_CreateDuplicateName_Conflict(t *testing.T) {
	s := store.NewFileEnvironmentStore(t.TempDir())
	ctx := context.Background()

	if err := s.Create(ctx, newEnv("dev", "https://a.example.com")); err != nil {
		t.Fatal(err)
	}
	err := s.Create(ctx, &domain.Environment{ID: "env_other", Name: "dev", BaseURL: "https://b.example.com"})
	if !errors.Is(err, store.ErrEnvNameTaken) {
		t.Errorf("want ErrEnvNameTaken, got %v", err)
	}
}

func TestFileEnvStore_UpdateAndDelete(t *testing.T) {
	s := store.NewFileEnvironmentStore(t.TempDir())
	ctx := context.Background()

	if err := s.Create(ctx, newEnv("dev", "https://a.example.com")); err != nil {
		t.Fatal(err)
	}
	env, _ := s.Get(ctx, "env_dev")
	env.BaseURL = "https://b.example.com"
	if err := s.Update(ctx, env); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := s.Get(ctx, "env_dev")
	if got.BaseURL != "https://b.example.com" {
		t.Errorf("update did not persist: %q", got.BaseURL)
	}

	if err := s.Delete(ctx, "env_dev"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := s.Get(ctx, "env_dev"); ok {
		t.Error("delete did not remove entry")
	}
	if err := s.Delete(ctx, "env_ghost"); !errors.Is(err, store.ErrEnvNotFound) {
		t.Errorf("delete nonexistent: want ErrEnvNotFound, got %v", err)
	}
}

// パスワードを含むファイルは 0600 で書かれる(他ユーザーから読まれないように)。
func TestFileEnvStore_FilePermissions0600(t *testing.T) {
	dir := t.TempDir()
	s := store.NewFileEnvironmentStore(dir)
	env := newEnv("dev", "https://a.example.com")
	env.BasicAuthPass = "secret"
	if err := s.Create(context.Background(), env); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, ".environments.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %v, want 0600 (パスワードを平文で含むため)", info.Mode().Perm())
	}
}
