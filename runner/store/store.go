package store

import "e2e-runner/domain"

// RunStore はテスト実行の保存・取得を抽象化するインターフェース。
type RunStore interface {
	Save(run *domain.Run) error
	Get(id string) (*domain.Run, bool)
	Delete(id string) error
}

// CodegenStore はシナリオ記録セッションの保存・取得を抽象化する。
type CodegenStore interface {
	Save(c *domain.Codegen) error
	Get(id string) (*domain.Codegen, bool)
}
