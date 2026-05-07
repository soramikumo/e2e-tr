package store

import (
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

func (s *MemoryRunStore) Save(run *domain.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.ID] = run
	return nil
}

func (s *MemoryRunStore) Get(id string) (*domain.Run, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.runs[id]
	return r, ok
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
