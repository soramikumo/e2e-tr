package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// TagDef は色付きのタグ定義。GitHub の Label に相当する。
type TagDef struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// TagMeta はタグ定義と「どのシナリオにどのタグが貼られたか」の割当を保持する。
// Assignments のキーはシナリオのファイル名(例 "login.spec.ts")。
type TagMeta struct {
	Tags        []TagDef            `json:"tags"`
	Assignments map[string][]string `json:"assignments"`
}

func tagMetaPath(testsDir string) string {
	return filepath.Join(testsDir, ".tags.json")
}

// LoadTagMeta は .tags.json を読み込む。未生成の場合は空のメタデータを返す。
func LoadTagMeta(testsDir string) *TagMeta {
	data, err := os.ReadFile(tagMetaPath(testsDir))
	if err == nil {
		var meta TagMeta
		if json.Unmarshal(data, &meta) == nil {
			if meta.Assignments == nil {
				meta.Assignments = map[string][]string{}
			}
			return &meta
		}
	}
	return emptyTagMeta()
}

// emptyTagMeta は割当もタグ定義も持たない初期メタデータを返す。
// 旧来は spec 内の @tag を初期定義に変換していたが、codegen 生成のシナリオは
// 意味ある @tag を持たず、import 文の @playwright を誤検出してゴーストタグ
// (割当なしでクリックすると 400 になるタグ)を量産していたため廃止した。
func emptyTagMeta() *TagMeta {
	return &TagMeta{Tags: make([]TagDef, 0), Assignments: map[string][]string{}}
}

// SaveTagMeta は .tags.json へ整形して書き出す。
func SaveTagMeta(testsDir string, meta *TagMeta) error {
	sort.Slice(meta.Tags, func(i, j int) bool { return meta.Tags[i].Name < meta.Tags[j].Name })
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(tagMetaPath(testsDir), data, 0o644)
}

// TagsForScenario は指定シナリオに割り当てられたタグ名を返す(常に非 nil)。
func (m *TagMeta) TagsForScenario(scenario string) []string {
	names := m.Assignments[scenario]
	if names == nil {
		return []string{}
	}
	return names
}

// ScenariosForTag は指定タグが貼られたシナリオのファイル名を返す。
func (m *TagMeta) ScenariosForTag(tag string) []string {
	files := make([]string, 0)
	for scenario, names := range m.Assignments {
		for _, n := range names {
			if n == tag {
				files = append(files, scenario)
				break
			}
		}
	}
	sort.Strings(files)
	return files
}
