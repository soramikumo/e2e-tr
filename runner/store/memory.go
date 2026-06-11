package store

import (
	"context"
	"sort"
	"sync"

	"e2e-runner/domain"
)

type MemoryRunStore struct {
	mu   sync.RWMutex
	runs map[string]*domain.Run
}

func NewMemoryRunStore() *MemoryRunStore {
	return &MemoryRunStore{runs: map[string]*domain.Run{}}
}

// ローカル実装は単一ユーザー前提なので ctx は使わない(_ で受ける)。
func (s *MemoryRunStore) Save(_ context.Context, run *domain.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.ID] = run
	return nil
}

func (s *MemoryRunStore) Get(_ context.Context, id string) (*domain.Run, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.runs[id]
	return r, ok
}

func (s *MemoryRunStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.runs, id)
	return nil
}

// List は保持中の Run を新しい順(started_at 降順)で返す。
func (s *MemoryRunStore) List(_ context.Context) ([]*domain.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*domain.Run, 0, len(s.runs))
	for _, r := range s.runs {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out, nil
}

type MemoryCodegenStore struct {
	mu       sync.RWMutex
	codegens map[string]*domain.Codegen
}

func NewMemoryCodegenStore() *MemoryCodegenStore {
	return &MemoryCodegenStore{codegens: map[string]*domain.Codegen{}}
}

func (s *MemoryCodegenStore) Save(c *domain.Codegen) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.codegens[c.ID] = c
	return nil
}

func (s *MemoryCodegenStore) Get(id string) (*domain.Codegen, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.codegens[id]
	return c, ok
}
