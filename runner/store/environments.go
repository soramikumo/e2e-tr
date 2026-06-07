package store

import (
	"context"
	"errors"
	"sync"

	"e2e-runner/domain"
)

// FileEnvironmentStore は .environments.json を単一の真実の源とし、
// read-modify-write をロックで直列化する。TagStore と同じ流儀。
// OSS=ローカル単一ユーザー前提なので ctx は無視する(cloud 実装で使う継ぎ目)。
type FileEnvironmentStore struct {
	mu       sync.Mutex
	testsDir string
}

func NewFileEnvironmentStore(testsDir string) *FileEnvironmentStore {
	return &FileEnvironmentStore{testsDir: testsDir}
}

// ErrEnvNotFound は対象 id の environment が無いことを示す。
var ErrEnvNotFound = errors.New("environment not found")

// ErrEnvNameTaken は同名の environment が既にあることを示す。Name は人間が選ぶ
// 一意キーとして扱う(dev/staging/prod のような短い区別子)。
var ErrEnvNameTaken = errors.New("environment name already exists")

func (s *FileEnvironmentStore) List(_ context.Context) ([]domain.Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta := domain.LoadEnvMeta(s.testsDir)
	// コピーを返す(呼び出し側の変更が内部状態に波及しないように)。
	out := make([]domain.Environment, len(meta.Environments))
	copy(out, meta.Environments)
	return out, nil
}

func (s *FileEnvironmentStore) Get(_ context.Context, id string) (*domain.Environment, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta := domain.LoadEnvMeta(s.testsDir)
	for i := range meta.Environments {
		if meta.Environments[i].ID == id {
			e := meta.Environments[i]
			return &e, true
		}
	}
	return nil, false
}

func (s *FileEnvironmentStore) Create(_ context.Context, env *domain.Environment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta := domain.LoadEnvMeta(s.testsDir)
	for i := range meta.Environments {
		if meta.Environments[i].Name == env.Name {
			return ErrEnvNameTaken
		}
	}
	meta.Environments = append(meta.Environments, *env)
	return domain.SaveEnvMeta(s.testsDir, meta)
}

func (s *FileEnvironmentStore) Update(_ context.Context, env *domain.Environment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta := domain.LoadEnvMeta(s.testsDir)
	idx := -1
	for i := range meta.Environments {
		if meta.Environments[i].ID == env.ID {
			idx = i
			continue
		}
		// 名前重複チェック(自分以外で同名は不可)。
		if meta.Environments[i].Name == env.Name {
			return ErrEnvNameTaken
		}
	}
	if idx < 0 {
		return ErrEnvNotFound
	}
	meta.Environments[idx] = *env
	return domain.SaveEnvMeta(s.testsDir, meta)
}

func (s *FileEnvironmentStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta := domain.LoadEnvMeta(s.testsDir)
	for i := range meta.Environments {
		if meta.Environments[i].ID == id {
			meta.Environments = append(meta.Environments[:i], meta.Environments[i+1:]...)
			return domain.SaveEnvMeta(s.testsDir, meta)
		}
	}
	return ErrEnvNotFound
}
