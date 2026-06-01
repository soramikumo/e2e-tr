package store_test

import (
	"slices"
	"testing"

	"e2e-runner/store"
)

func TestTagStore_UpsertAndList(t *testing.T) {
	s := store.NewTagStore(t.TempDir())
	s.UpsertTag("smoke", "#1f883d")
	s.UpsertTag("smoke", "#cf222e") // 同名は色を更新

	tags := s.List()
	if len(tags) != 1 {
		t.Fatalf("len(tags) = %d, want 1", len(tags))
	}
	if tags[0].Color != "#cf222e" {
		t.Errorf("color = %q, want updated #cf222e", tags[0].Color)
	}
}

func TestTagStore_ScenariosForTag(t *testing.T) {
	s := store.NewTagStore(t.TempDir())
	s.SetScenarioTags("a.spec.ts", []string{"smoke", "critical"})
	s.SetScenarioTags("b.spec.ts", []string{"smoke"})
	s.SetScenarioTags("c.spec.ts", []string{"critical"})

	got := s.ScenariosForTag("smoke")
	want := []string{"a.spec.ts", "b.spec.ts"}
	if !slices.Equal(got, want) {
		t.Errorf("ScenariosForTag(smoke) = %v, want %v", got, want)
	}
}

// タグ削除時に、全シナリオの割当からも取り除かれることを確認する(カスケード)。
func TestTagStore_DeleteTag_CascadesAssignments(t *testing.T) {
	s := store.NewTagStore(t.TempDir())
	s.UpsertTag("smoke", "#1f883d")
	s.SetScenarioTags("a.spec.ts", []string{"smoke", "critical"})
	s.SetScenarioTags("b.spec.ts", []string{"smoke"})

	s.DeleteTag("smoke")

	if got := s.ScenariosForTag("smoke"); len(got) != 0 {
		t.Errorf("ScenariosForTag(smoke) after delete = %v, want empty", got)
	}
	// 他タグが残るシナリオは保持、空になったシナリオはエントリごと消える。
	if got := s.TagsForScenario("a.spec.ts"); !slices.Equal(got, []string{"critical"}) {
		t.Errorf("a.spec.ts tags = %v, want [critical]", got)
	}
	if got := s.TagsForScenario("b.spec.ts"); len(got) != 0 {
		t.Errorf("b.spec.ts tags = %v, want empty", got)
	}
}

// シナリオ削除時に割当が落ちることを確認する。
func TestTagStore_DropScenario(t *testing.T) {
	s := store.NewTagStore(t.TempDir())
	s.SetScenarioTags("a.spec.ts", []string{"smoke"})
	s.DropScenario("a.spec.ts")

	if got := s.ScenariosForTag("smoke"); len(got) != 0 {
		t.Errorf("ScenariosForTag(smoke) after drop = %v, want empty", got)
	}
}
