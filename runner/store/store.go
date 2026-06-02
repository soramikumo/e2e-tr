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
}

// CodegenStore はシナリオ記録セッションの保存・取得を抽象化する。
type CodegenStore interface {
	Save(c *domain.Codegen) error
	Get(id string) (*domain.Codegen, bool)
}
