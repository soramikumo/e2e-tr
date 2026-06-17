package store

import (
	"context"

	"e2e-runner/domain"
)

// RunStore はテスト実行の保存・取得を抽象化するインターフェース。
//
// 各メソッドは context を受け取る。ローカル(単一ユーザー)実装は値を使わないが、
// マルチテナント実装は context に載った owner_id で「誰の Run か」を判定し、
// 保存時の所有付与・取得時の認可に使う。context はクエリのキャンセル伝播にも効く。
type RunStore interface {
	Save(ctx context.Context, run *domain.Run) error
	Get(ctx context.Context, id string) (*domain.Run, bool)
	Delete(ctx context.Context, id string) error
	// List は実行履歴を新しい順(started_at 降順)で返す。履歴一覧 UI 用。
	List(ctx context.Context) ([]*domain.Run, error)
}

// CodegenStore はシナリオ記録セッションの保存・取得を抽象化する。
type CodegenStore interface {
	Save(c *domain.Codegen) error
	Get(id string) (*domain.Codegen, bool)
}

// EnvironmentStore は「実行先」設定の永続化を抽象化する。
//
// ctx は OSS のローカル実装(FileEnvironmentStore)では使わないが、cloud 実装
// (PostgresEnvironmentStore)が owner_id を取り出して WHERE 句に積むための
// 継ぎ目として最初から引数に持たせる(saas-architecture-decisions に従う)。
type EnvironmentStore interface {
	List(ctx context.Context) ([]domain.Environment, error)
	Get(ctx context.Context, id string) (*domain.Environment, bool)
	Create(ctx context.Context, env *domain.Environment) error
	Update(ctx context.Context, env *domain.Environment) error
	Delete(ctx context.Context, id string) error
}
