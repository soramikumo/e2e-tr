package store

import (
	"slices"
	"sync"

	"e2e-runner/internal/domain"
)

// TagStore は .tags.json を単一の真実の源とし、read-modify-write をロックで直列化する。
// 毎回ロード/保存することで、外部編集とも矛盾しにくくする。
type TagStore struct {
	mu       sync.Mutex
	testsDir string
}

func NewTagStore(testsDir string) *TagStore {
	return &TagStore{testsDir: testsDir}
}

// List は定義済みタグ一覧を返す。
func (s *TagStore) List() []domain.TagDef {
	s.mu.Lock()
	defer s.mu.Unlock()
	return domain.LoadTagMeta(s.testsDir).Tags
}

// UpsertTag は同名タグがあれば色を更新し、なければ追加する。
func (s *TagStore) UpsertTag(name, color string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta := domain.LoadTagMeta(s.testsDir)
	for i := range meta.Tags {
		if meta.Tags[i].Name == name {
			meta.Tags[i].Color = color
			return domain.SaveTagMeta(s.testsDir, meta)
		}
	}
	meta.Tags = append(meta.Tags, domain.TagDef{Name: name, Color: color})
	return domain.SaveTagMeta(s.testsDir, meta)
}

// DeleteTag はタグ定義を削除し、全シナリオの割当からも取り除く(カスケード)。
func (s *TagStore) DeleteTag(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta := domain.LoadTagMeta(s.testsDir)

	// 定義一覧から該当タグを除去する。
	meta.Tags = slices.DeleteFunc(meta.Tags, func(t domain.TagDef) bool { return t.Name == name })

	// カスケード削除 — 各シナリオの割当から name を取り除く。割当が空になった
	// シナリオは、SetScenarioTags の方針に合わせてエントリごと削除する。
	for scenario, names := range meta.Assignments {
		names = slices.DeleteFunc(names, func(n string) bool { return n == name })
		if len(names) == 0 {
			delete(meta.Assignments, scenario)
		} else {
			meta.Assignments[scenario] = names
		}
	}

	return domain.SaveTagMeta(s.testsDir, meta)
}

// SetScenarioTags は指定シナリオのタグ割当を丸ごと置き換える。
func (s *TagStore) SetScenarioTags(scenario string, tags []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta := domain.LoadTagMeta(s.testsDir)
	if len(tags) == 0 {
		delete(meta.Assignments, scenario)
	} else {
		meta.Assignments[scenario] = tags
	}
	return domain.SaveTagMeta(s.testsDir, meta)
}

// RenameScenario はシナリオ rename 時に、タグ割当を旧名から新名へ移す。
// 旧名に割当が無ければ何もしない（新名のエントリは作らない）。
func (s *TagStore) RenameScenario(oldName, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta := domain.LoadTagMeta(s.testsDir)
	names, ok := meta.Assignments[oldName]
	if !ok {
		// 旧名に割当が無ければ書き込み不要（無駄なディスク I/O を避ける）。
		return nil
	}
	meta.Assignments[newName] = names
	delete(meta.Assignments, oldName)
	return domain.SaveTagMeta(s.testsDir, meta)
}

// DropScenario はシナリオ削除時に、そのシナリオの割当エントリを取り除く。
func (s *TagStore) DropScenario(scenario string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta := domain.LoadTagMeta(s.testsDir)
	delete(meta.Assignments, scenario)
	return domain.SaveTagMeta(s.testsDir, meta)
}

// TagsForScenario は指定シナリオに割り当てられたタグ名を返す。
func (s *TagStore) TagsForScenario(scenario string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return domain.LoadTagMeta(s.testsDir).TagsForScenario(scenario)
}

// ScenariosForTag は指定タグが貼られたシナリオのファイル名を返す。
func (s *TagStore) ScenariosForTag(tag string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return domain.LoadTagMeta(s.testsDir).ScenariosForTag(tag)
}
