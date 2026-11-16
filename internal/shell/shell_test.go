package shell

import (
	"log/slog"
	"testing"
	"time"

	"github.com/Cyenoch/mygoui/locale"
	"github.com/Neuz/DevToys/internal/config"
	"github.com/Neuz/DevToys/internal/contract"
	"github.com/Neuz/DevToys/internal/features/settings"
	"github.com/Neuz/DevToys/locales"
	"github.com/egoist/mygo/ui"
)

// stubModule 提供一个不依赖 mygo.App 的页面，让窗口外壳可以独立布局。
type stubModule struct{}

func (stubModule) ID() string         { return "welcome" }
func (stubModule) Title() string      { return locales.WelcomeTitle() }
func (stubModule) Init(contract.Host) {}
func (stubModule) View(c *ui.Context) { ui.Text(c, "content") }

func TestMainWindowOpensSettingsFromTitleBar(t *testing.T) {
	st := settings.New(settings.Options{Config: config.Default(), Log: slog.Default()})
	s := newShell(Options{
		Name:      "DevToys",
		Modules:   []contract.Module{stubModule{}},
		DefaultID: "welcome",
		Log:       slog.Default(),
		Settings:  func(contract.Controller) contract.Settings { return st },
	})

	tt := ui.NewTester(s.view, 960, 640)
	tt.Frame()
	if _, ok := tt.Access(locales.SettingsTitle()); !ok {
		t.Fatalf("the title bar has no %q button", locales.SettingsTitle())
	}
	if err := tt.Click(locales.SettingsTitle()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !s.settingsOpen {
		t.Fatal("clicking the settings button did not open the dialog")
	}
	if _, ok := tt.AccessByRole(ui.RoleDialog, locales.SettingsTitle()); !ok {
		t.Fatal("the settings dialog is not in the accessibility tree")
	}
	tt.Advance(time.Second)
	if tt.FramePending() {
		t.Fatal("the view still asks for frames at rest")
	}
}

// TestSidebarCollapseMovesContent 确认侧栏折叠成图标轨道后，内容区跟着左移，
// 而不是停在原来的位置留下空白。
func TestSidebarCollapseMovesContent(t *testing.T) {
	s := newShell(Options{
		Name:         "DevToys",
		Modules:      []contract.Module{stubModule{}},
		DefaultID:    "welcome",
		SidebarWidth: 220,
		Log:          slog.Default(),
	})

	tt := ui.NewTester(s.view, 960, 640)
	tt.Frame()
	before, ok := tt.Access("content")
	if !ok {
		t.Fatal("the content is not in the accessibility tree")
	}
	if err := tt.Click(locale.M().ToggleSidebar); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	tt.Frame()
	after, ok := tt.Access("content")
	if !ok {
		t.Fatal("the content left the accessibility tree after collapsing")
	}
	if !s.collapsed {
		t.Fatal("clicking the toggle did not collapse the sidebar")
	}
	if after.Bounds.X >= before.Bounds.X {
		t.Errorf("content starts at %v after collapsing, was %v", after.Bounds.X, before.Bounds.X)
	}
	if after.Bounds.W <= before.Bounds.W {
		t.Errorf("content is %v wide after collapsing, was %v", after.Bounds.W, before.Bounds.W)
	}
}
