package launcher

import (
	"testing"
	"time"
)

func TestFilter(t *testing.T) {
	entries := []Entry{
		{Title: "Visual Studio Code", Path: "code", Keywords: "Visual Studio Code Development"},
		{Title: "计算器", Path: "calc", Keywords: "计算器 附件"},
		{Title: "Terminal", Path: "term", Keywords: "Terminal 系统工具"},
	}

	if got := filter(entries, ""); len(got) != len(entries) {
		t.Fatalf("empty query returned %d entries, want %d", len(got), len(entries))
	}
	if got := filter(entries, "  "); len(got) != len(entries) {
		t.Fatalf("blank query returned %d entries, want %d", len(got), len(entries))
	}
	if got := filter(entries, "STUDIO"); len(got) != 1 || got[0].Path != "code" {
		t.Fatalf("case-insensitive title match = %+v", got)
	}
	if got := filter(entries, "系统"); len(got) != 1 || got[0].Path != "term" {
		t.Fatalf("keyword match = %+v", got)
	}
	if got := filter(entries, "missing"); len(got) != 0 {
		t.Fatalf("unexpected matches: %+v", got)
	}
}

func TestFilterCategory(t *testing.T) {
	entries := []Entry{
		{Title: "Code", Category: categoryApp},
		{Title: "设置", Category: categorySystem},
		{Title: "Legacy"}, // 旧缓存没有分类，按应用处理
	}
	if got := filterCategory(entries, ""); len(got) != len(entries) {
		t.Errorf("all category = %d, want %d", len(got), len(entries))
	}
	if got := filterCategory(entries, categoryApp); len(got) != 2 {
		t.Errorf("app category = %+v, want app plus legacy", got)
	}
	if got := filterCategory(entries, categorySystem); len(got) != 1 || got[0].Title != "设置" {
		t.Errorf("system category = %+v, want only 设置", got)
	}
}

func TestFilterMatchesPinyin(t *testing.T) {
	entries := []Entry{{Title: "计算器", Path: "calc", Pinyin: pinyinText("计算器")}}
	for _, query := range []string{"jisuanqi", "jsq", "JISUANQI"} {
		if got := filter(entries, query); len(got) != 1 {
			t.Fatalf("query %q matched %+v, want calc", query, got)
		}
	}
}

func TestFilterRequiresEveryToken(t *testing.T) {
	entries := []Entry{
		{Title: "Visual Studio Code", Path: "code"},
		{Title: "Visual Studio", Path: "vs"},
	}
	if got := filter(entries, "visual code"); len(got) != 1 || got[0].Path != "code" {
		t.Fatalf("multi-token match = %+v, want only code", got)
	}
	if got := filter(entries, "visual missing"); len(got) != 0 {
		t.Fatalf("unexpected matches: %+v", got)
	}
}

func TestFilterMatchesFolderInKeywords(t *testing.T) {
	entries := []Entry{{Title: "Notepad", Path: "np", Keywords: "Notepad 附件"}}
	if got := filter(entries, "附件"); len(got) != 1 {
		t.Fatalf("folder match = %+v, want np", got)
	}
}

func TestRankByUsageThenLast(t *testing.T) {
	now := time.Now()
	entries := []Entry{
		{Title: "Alpha", Path: "a"},
		{Title: "Beta", Path: "b"},
		{Title: "Gamma", Path: "c"},
	}
	usage := map[string]Usage{
		"a": {Count: 1, Last: now},
		"b": {Count: 3, Last: now.Add(-time.Hour)},
		"c": {Count: 1, Last: now.Add(time.Hour)},
	}

	got := rank(entries, usage, 3)
	want := []string{"b", "c", "a"}
	for i, path := range want {
		if got[i].Path != path {
			t.Fatalf("rank order = %+v, want %v", got, want)
		}
	}
}

func TestRankTieBreaksByTitleLength(t *testing.T) {
	now := time.Now()
	entries := []Entry{
		{Title: "Longer Name", Path: "long"},
		{Title: "Short", Path: "short"},
	}
	usage := map[string]Usage{
		"long":  {Count: 2, Last: now},
		"short": {Count: 2, Last: now},
	}

	got := rank(entries, usage, 2)
	if got[0].Path != "short" {
		t.Fatalf("rank = %+v, want shorter title first", got)
	}
}

func TestRankWithoutUsageIsLexicographic(t *testing.T) {
	entries := []Entry{
		{Title: "Zeta", Path: "z"},
		{Title: "alpha", Path: "a"},
		{Title: "Beta", Path: "b"},
	}

	got := rank(entries, nil, 2)
	if len(got) != 2 || got[0].Path != "a" || got[1].Path != "b" {
		t.Fatalf("rank = %+v, want alpha then Beta", got)
	}
}

func TestRankTruncates(t *testing.T) {
	var entries []Entry
	for _, title := range []string{"A", "B", "C", "D"} {
		entries = append(entries, Entry{Title: title, Path: title})
	}
	if got := rank(entries, nil, 3); len(got) != 3 {
		t.Fatalf("rank returned %d entries, want 3", len(got))
	}
	if got := rank(entries, nil, 0); len(got) != 0 {
		t.Fatalf("zero limit returned %d entries", len(got))
	}
	if got := rank(nil, nil, 5); len(got) != 0 {
		t.Fatalf("empty input returned %d entries", len(got))
	}
}

func TestRankDoesNotReorderInput(t *testing.T) {
	entries := []Entry{{Title: "B", Path: "b"}, {Title: "A", Path: "a"}}
	_ = rank(entries, nil, 2)
	if entries[0].Title != "B" {
		t.Fatal("rank modified the input slice order")
	}
}
