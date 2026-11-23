package launcher

import (
	"os"
	"path/filepath"
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
		{Name: "远程桌面连接", ID: "mstsc.exe", Path: `shell:AppsFolder\mstsc.exe`},
		{Name: "Duplicate", ID: `C:\dup.exe`, Path: "p1"},
		{Name: "Duplicate again", ID: `C:\dup.exe`, Path: "p2"},
		{Name: "No path", ID: "nopath"},
	}
	entries := entriesFromApps(apps)
	if len(entries) != 4 {
		t.Fatalf("entries = %+v, want 4", entries)
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

	remote := byTitle["远程桌面连接"]
	if !strings.Contains(remote.Keywords, " mstsc") {
		t.Errorf("remote keywords = %q, want bare exe name", remote.Keywords)
	}
	if strings.Contains(remote.Keywords, ".exe") {
		t.Errorf("remote keywords = %q, want the extension removed", remote.Keywords)
	}
}

func TestDedupeByTitle(t *testing.T) {
	entries := []Entry{
		{Title: "远程桌面连接", ID: "mstsc"},
		{Title: "远程桌面连接", ID: "store"},
		{Title: "Calculator", ID: "calc"},
		{Title: "CALCULATOR", ID: "calc2"},
		{Title: "Notepad", ID: "notepad"},
	}
	got := dedupeByTitle(entries)
	if len(got) != 3 {
		t.Fatalf("dedupeByTitle() = %+v, want 3", got)
	}
	for i, want := range []string{"mstsc", "calc", "notepad"} {
		if got[i].ID != want {
			t.Errorf("entry %d id = %q, want %q", i, got[i].ID, want)
		}
	}
}

func TestBuiltinProgramEntries(t *testing.T) {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	if _, err := os.Stat(filepath.Join(root, "System32", "mstsc.exe")); err != nil {
		t.Skip("mstsc.exe not available")
	}
	entries := builtinEntries()
	var remote Entry
	for _, e := range entries {
		if e.Category == categoryApp && strings.EqualFold(filepath.Base(e.Path), "mstsc.exe") {
			remote = e
		}
	}
	if remote.Path == "" {
		t.Fatalf("builtinEntries() = %+v, want mstsc", entries)
	}
	if remote.Description != "mstsc.exe" {
		t.Errorf("description = %q, want mstsc.exe", remote.Description)
	}
	// 输入 mstsc 必须能命中，这是补充索引的目的。
	if got := filter(entries, "mstsc"); len(got) == 0 {
		t.Errorf("filter(%+v, mstsc) = empty, want the remote desktop tool", entries)
	}
}

func TestExecutableStem(t *testing.T) {
	cases := []struct {
		name string
		app  mygo.InstalledApp
		want string
	}{
		{"id path", mygo.InstalledApp{ID: `C:\Windows\System32\mstsc.exe`}, "mstsc"},
		{"bare id", mygo.InstalledApp{ID: "calc.exe"}, "calc"},
		{"path only", mygo.InstalledApp{ID: "pkg!App", Path: `shell:AppsFolder\C:\Tools\calc.exe`}, "calc"},
		{"name only", mygo.InstalledApp{Name: "notepad.exe"}, "notepad"},
		{"packaged", mygo.InstalledApp{ID: "Microsoft.WindowsCalculator_8wekyb3d8bbwe!App", Name: "计算器"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := executableStem(tc.app); got != tc.want {
				t.Errorf("executableStem(%+v) = %q, want %q", tc.app, got, tc.want)
			}
		})
	}
}

func TestEntriesFromAppsSetsCategory(t *testing.T) {
	entries := entriesFromApps([]mygo.InstalledApp{{Name: "Code", ID: "code", Path: "p"}})
	if len(entries) != 1 || entries[0].Category != categoryApp {
		t.Fatalf("entries = %+v, want category %q", entries, categoryApp)
	}
}

func TestBuiltinSettingsEntries(t *testing.T) {
	want := 0
	keys := make(map[string]struct{}, len(builtins))
	for _, b := range builtins {
		if _, ok := keys[b.key]; ok {
			t.Errorf("duplicate key %q", b.key)
		}
		keys[b.key] = struct{}{}
		if b.settings {
			want++
		}
	}
	seen := make(map[string]struct{}, want)
	titles := make(map[string]struct{}, want)
	got := 0
	for _, e := range builtinEntries() {
		if e.Category != categorySystem {
			continue
		}
		got++
		if e.Title == "" {
			t.Errorf("empty title for %q", e.Path)
		}
		if e.Keywords == "" {
			t.Errorf("%q has empty keywords", e.Title)
		}
		if !strings.HasPrefix(e.Path, "ms-settings:") {
			t.Errorf("%q path = %q, want MS-Settings URI", e.Title, e.Path)
		}
		if _, ok := seen[e.ID]; ok {
			t.Errorf("duplicate id %q", e.ID)
		}
		seen[e.ID] = struct{}{}
		if _, ok := titles[e.Title]; ok {
			t.Errorf("duplicate title %q", e.Title)
		}
		titles[e.Title] = struct{}{}
	}
	if got != want {
		t.Errorf("settings entries = %d, want %d", got, want)
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
	if got := (indexOptions{MaxResults: 3}).normalized().MaxResults; got != 3 {
		t.Errorf("max_results = %d, want 3", got)
	}
	if got := (indexOptions{MaxResults: 100}).normalized().MaxResults; got != 100 {
		t.Errorf("large max_results = %d, want 100", got)
	}
	if got := (indexOptions{}).normalized().MaxResults; got != defaultResultCount {
		t.Errorf("default max_results = %d, want %d", got, defaultResultCount)
	}
	if got := (indexOptions{MaxResults: -1}).normalized().MaxResults; got != defaultResultCount {
		t.Errorf("negative max_results = %d, want %d", got, defaultResultCount)
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
