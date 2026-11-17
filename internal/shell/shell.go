// Package shell 是应用外壳：主窗口、侧栏导航、主题与托盘。
package shell

import (
	"errors"
	"log/slog"
	"sync"

	"github.com/Cyenoch/mygoui"
	"github.com/Cyenoch/mygoui/form"
	"github.com/Cyenoch/mygoui/theme"
	"github.com/Neuz/DevToys/internal/config"
	"github.com/Neuz/DevToys/internal/contract"
	"github.com/Neuz/DevToys/internal/task"
	"github.com/Neuz/DevToys/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Options 配置主窗口。
type Options struct {
	Name      string
	Modules   []contract.Module
	DefaultID string

	// 主窗口的初始尺寸与最小尺寸（DIP），由调用方决定。
	Width     int
	Height    int
	MinWidth  int
	MinHeight int

	// Config 是持久化的用户偏好；设置窗口直接读改写它。
	Config *config.Config

	Tray *Tray        // 非空：常驻托盘，关主窗口只隐藏
	Log  *slog.Logger // nil 时使用 slog.Default()
}

// Shell 持有窗口与各功能的运行时状态。
type Shell struct {
	opts    Options
	modules []contract.Module
	byID    map[string]contract.Module

	current     string
	active      string
	win         *mygo.Window
	runners     []*task.Runner
	settings    *settings
	settingsWin *mygo.Window

	// collapsed 由侧栏的折叠按钮改写，决定侧栏面板的宽度。
	collapsed bool

	// scroll 是内容区的滚动位置，跨帧保存。
	scroll ui.ScrollState

	// hotkeyGlobal 是已注册的全局快捷键 accelerator，空表示没有。
	hotkeyGlobal string

	// sidebarHotkey 是应用内折叠/展开侧栏的组合；KeyUnknown 表示未设置。
	sidebarHotkey form.Hotkey

	log       *slog.Logger
	quitting  bool
	closeOnce sync.Once

	tray *mygo.Tray
}

// Run 注册外壳的回调；mygo.App.Run 仍由调用方执行。
func Run(opts Options) error {
	if len(opts.Modules) == 0 {
		return errors.New("shell: no modules")
	}
	if opts.Config == nil {
		return errors.New("shell: no config")
	}
	s := newShell(opts)
	// 在 App.Run 之前设置，让首帧就是保存的外观。
	applyTheme(opts.Config.Theme)

	mygo.App.WhenReady(s.ready)
	mygo.App.OnSecondInstance(func([]string, string) { s.showMain(s.current) })
	mygo.App.OnWindowAllClosed(func() {
		if opts.Tray == nil {
			mygo.App.Quit()
		}
	})
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		s.quitting = true
		s.close()
	})
	return nil
}

// newShell 整理模块表与界面状态；不触碰 mygo.App，便于测试直接构建。
func newShell(opts Options) *Shell {
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}
	s := &Shell{opts: opts, log: log, byID: make(map[string]contract.Module, len(opts.Modules))}
	for _, m := range opts.Modules {
		s.modules = append(s.modules, m)
		s.byID[m.ID()] = m
	}
	s.current = opts.DefaultID
	if _, ok := s.byID[s.current]; !ok {
		s.current = s.modules[0].ID()
	}
	s.sidebarHotkey = parseHotkey(opts.Config.Hotkey.App.SidebarToggle)
	return s
}

func (s *Shell) ready() {
	applyLocale(s.opts.Config.Locale)
	s.win = mygo.NewWindow(mygo.WindowOptions{
		Title:          s.opts.Name,
		Width:          s.opts.Width,
		Height:         s.opts.Height,
		MinWidth:       s.opts.MinWidth,
		MinHeight:      s.opts.MinHeight,
		StateKey:       "main",
		TitleBarStyle:  mygo.TitleBarHidden,
		TitleBarHeight: int(titleBarHeight),
		Content:        ui.View(s.view),
	})

	for _, m := range s.modules {
		runner := task.NewRunner(s.win.Update)
		s.runners = append(s.runners, runner)
		m.Init(host{tasks: runner, log: s.log.With("module", m.ID())})
	}
	mygo.Theme.OnUpdated(s.invalidateAll)
	mygo.App.OnLocaleChanged(func(string) {
		if s.settingsWin != nil {
			s.settingsWin.SetTitle(locales.SettingsTitle())
		}
		s.invalidateAll()
	})

	s.win.OnClose(func(e *mygo.CloseEvent) {
		if s.opts.Tray != nil && !s.quitting {
			e.PreventDefault()
			s.win.Hide()
			mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
		}
	})

	if s.opts.Tray != nil {
		s.setupTray()
	}
	s.bindHotkey(s.opts.Config.Hotkey.Global.WindowToggle)
	s.activate(s.current)
}

// view 只做组装：左侧整列 Sidebar，右侧是标题栏与可滚动的内容区。
func (s *Shell) view(c *ui.Context) {
	theme.Apply(c)
	if k := s.sidebarHotkey; k.Key != ui.KeyUnknown {
		c.OnWindowShortcut(k.Mods, k.Key, s.toggleSidebar)
	}
	t := theme.Colors(c)

	m := s.byID[s.current]

	ui.Row(c).Fill().MinWidth(0).MinHeight(0).Background(t.Background).TextColor(t.Foreground).Children(func() {
		s.sidebar(c)
		ui.Column(c).FillHeight().Grow(1).Basis(0).MinWidth(0).MinHeight(0).Children(func() {
			s.titleBar(c, titleBarOptions{
				Title: s.opts.Name,
			})
			mygoui.ScrollArea(c, &s.scroll, mygoui.ScrollAreaOptions{
				Axis: mygoui.ScrollVertical,
			}, func() {
				if m != nil {
					m.View(c)
				}
			}).FillWidth().Grow(1).Basis(0).MinWidth(0).MinHeight(0)
		})
	})
}

// invalidateAll 让主窗口与设置窗口按新的主题或语言重绘。
func (s *Shell) invalidateAll() {
	if s.win != nil {
		s.win.Invalidate()
	}
	if s.settingsWin != nil {
		s.settingsWin.Invalidate()
	}
}

// activate 切换当前模块，并通知实现了 contract.Live 的模块。
func (s *Shell) activate(id string) {
	if _, ok := s.byID[id]; !ok {
		return
	}
	s.current = id
	if s.active == id {
		return
	}
	if s.active != "" {
		if live, ok := s.byID[s.active].(contract.Live); ok {
			live.OnDeactivate()
		}
	}
	s.active = id
	if live, ok := s.byID[id].(contract.Live); ok {
		live.OnActivate()
	}
}

func (s *Shell) showMain(id string) {
	mygo.RunOnMain(func() {
		if _, ok := s.byID[id]; ok {
			s.activate(id)
		}
		if s.win == nil {
			return
		}
		mygo.App.SetActivationPolicy(mygo.ActivationPolicyRegular)
		s.win.Show()
		s.win.Focus()
		s.win.Invalidate()
	})
}

// close 关闭所有 runner、销毁托盘并注销全局快捷键；由 OnBeforeQuit 调用。
func (s *Shell) close() {
	s.closeOnce.Do(func() {
		for _, r := range s.runners {
			r.Close()
		}
		if s.hotkeyGlobal != "" {
			mygo.GlobalShortcut.Unregister(s.hotkeyGlobal)
			s.hotkeyGlobal = ""
		}
		if s.tray != nil {
			s.tray.Destroy()
		}
	})
}

// host 把 runner 与日志器交给功能模块。
type host struct {
	tasks *task.Runner
	log   *slog.Logger
}

func (h host) Tasks() *task.Runner { return h.tasks }

func (h host) Log() *slog.Logger { return h.log }
