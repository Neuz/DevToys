package launcher

import (
	"strings"
	"testing"

	"github.com/egoist/mygo"
)

func TestEntriesFromApps(t *testing.T) {
	apps := []mygo.InstalledApp{
		{
			Name: "Visual Studio Code",
			ID:   `C:\Program Files\Microsoft VS Code\Code.exe`,
			Path: `shell:AppsFolder\C:\Program Files\Microsoft VS Code\Code.exe`,
		},
		{
			Name: "计算器",
			ID:   "Microsoft.WindowsCalculator_8wekyb3d8bbwe!App",
			Path: `shell:AppsFolder\Microsoft.WindowsCalculator_8wekyb3d8bbwe!App`,
		},
		{Name: "Duplicate", ID: `C:\dup.exe`, Path: "p1"},
		{Name: "Duplicate again", ID: `C:\dup.exe`, Path: "p2"},
		{Name: "No path", ID: "nopath"},
	}
	entries := entriesFromApps(apps)
	if len(entries) != 3 {
		t.Fatalf("entries = %+v, want 3", entries)
	}
	byTitle := make(map[string]Entry, len(entries))
	for _, e := range entries {
		byTitle[e.Title] = e
	}

	code := byTitle["Visual Studio Code"]
	if !strings.Contains(code.Keywords, "Code") {
		t.Errorf("code keywords = %q, want the exe alias", code.Keywords)
	}
	if code.Description != "Code.exe" {
		t.Errorf("code description = %q, want Code.exe", code.Description)
	}
	if !strings.Contains(code.Pinyin, "visualstudiocode") {
		t.Errorf("code pinyin = %q", code.Pinyin)
	}

	calc := byTitle["计算器"]
	if !strings.Contains(calc.Keywords, "WindowsCalculator") {
		t.Errorf("calc keywords = %q, want the package alias", calc.Keywords)
	}
	if calc.Description != "" {
		t.Errorf("packaged description = %q, want empty", calc.Description)
	}
	if !strings.Contains(calc.Pinyin, "jisuanqi") {
		t.Errorf("calc pinyin = %q", calc.Pinyin)
	}
}

func TestEntriesFromAppsSetsCategory(t *testing.T) {
	entries := entriesFromApps([]mygo.InstalledApp{{Name: "Code", ID: "code", Path: "p"}})
	if len(entries) != 1 || entries[0].Category != categoryApp {
		t.Fatalf("entries = %+v, want category %q", entries, categoryApp)
	}
}

func TestSystemEntries(t *testing.T) {
	entries := systemEntries()
	if len(entries) != len(windowsSettings) {
		t.Fatalf("len = %d, want %d", len(entries), len(windowsSettings))
	}
	seen := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		if e.Title == "" {
			t.Errorf("empty title for %q", e.Path)
		}
		if e.Category != categorySystem {
			t.Errorf("%q category = %q, want %q", e.Title, e.Category, categorySystem)
		}
		if !strings.HasPrefix(e.Path, "ms-settings:") {
			t.Errorf("%q path = %q, want MS-Settings URI", e.Title, e.Path)
		}
		if _, ok := seen[e.ID]; ok {
			t.Errorf("duplicate id %q", e.ID)
		}
		seen[e.ID] = struct{}{}
	}
}

func TestEntriesFromAppsFallsBackToFileName(t *testing.T) {
	entries := entriesFromApps([]mygo.InstalledApp{{
		ID:   `C:\Tools\Notepad.exe`,
		Path: `shell:AppsFolder\C:\Tools\Notepad.exe`,
	}})
	if len(entries) != 1 || entries[0].Title != "Notepad" {
		t.Fatalf("entries = %+v, want title Notepad", entries)
	}
}

func TestAppAlias(t *testing.T) {
	cases := []struct {
		name string
		id   string
		want string
	}{
		{"desktop", `C:\Program Files\Microsoft VS Code\Code.exe`, "Code"},
		{"packaged", "Microsoft.WindowsCalculator_8wekyb3d8bbwe!App", "Microsoft.WindowsCalculator_8wekyb3d8bbwe"},
		{"empty", "", ""},
	}
	for _, c := range cases {
		if got := appAlias(mygo.InstalledApp{ID: c.id}); got != c.want {
			t.Errorf("%s alias = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestApplyExclude(t *testing.T) {
	entries := []Entry{
		{Title: "Uninstall Foo"},
		{Title: "README"},
		{Title: "帮助中心"},
		{Title: "文档查看器"},
		{Title: "Visual Studio Code"},
	}
	got := applyExclude(entries, defaultIndexOptions().Exclude)
	if len(got) != 1 || got[0].Title != "Visual Studio Code" {
		t.Fatalf("got %+v, want only Visual Studio Code", got)
	}
}

func TestApplyExcludeLeavesInputIntact(t *testing.T) {
	entries := []Entry{{Title: "Uninstall Foo"}, {Title: "Keep"}}
	_ = applyExclude(entries, defaultIndexOptions().Exclude)
	if len(entries) != 2 {
		t.Fatalf("input length changed to %d", len(entries))
	}
}

func TestPinyinTextKeepsFullAndInitials(t *testing.T) {
	got := pinyinText("计算器")
	if !strings.Contains(got, "jisuanqi") {
		t.Errorf("pinyin = %q, want full spelling", got)
	}
	if !strings.Contains(got, "jsq") {
		t.Errorf("pinyin = %q, want initials", got)
	}
	if got := pinyinText(""); got != "" {
		t.Errorf("empty title pinyin = %q, want empty", got)
	}
	if got := pinyinText("Visual Studio Code"); !strings.Contains(got, "visualstudiocode") {
		t.Errorf("latin pinyin = %q, want the lowercased title", got)
	}
}

func TestIndexOptionsNormalized(t *testing.T) {
	if got := (indexOptions{MaxResults: 1}).normalized().MaxResults; got != minResultCount {
		t.Errorf("low max_results = %d, want %d", got, minResultCount)
	}
	if got := (indexOptions{MaxResults: 100}).normalized().MaxResults; got != maxResultCount {
		t.Errorf("high max_results = %d, want %d", got, maxResultCount)
	}
	if got := (indexOptions{}).normalized().MaxResults; got != defaultResultCount {
		t.Errorf("default max_results = %d, want %d", got, defaultResultCount)
	}
	if got := (indexOptions{Exclude: []string{}}).normalized().Exclude; len(got) != 0 {
		t.Errorf("explicit empty exclude = %v, want empty", got)
	}
}

func TestExcludedCaseInsensitive(t *testing.T) {
	if !excluded("UNINSTALL Tool", []string{"uninstall"}) {
		t.Fatal("expected case-insensitive match")
	}
	if excluded(strings.ToUpper("Visual Studio"), []string{"uninstall"}) {
		t.Fatal("unexpected match")
	}
}
