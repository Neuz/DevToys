// Package shell 是应用外壳：主窗口、侧栏导航、主题与托盘。
package shell

import (
	"errors"
	"log/slog"
	"sync"

	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Cyenoch/mygoui"
	"github.com/Cyenoch/mygoui/form"
	"github.com/Cyenoch/mygoui/layout"
	"github.com/Cyenoch/mygoui/theme"
	"github.com/Neuz/DevToys/internal/config"
	"github.com/Neuz/DevToys/internal/contract"
	"github.com/Neuz/DevToys/internal/task"
	"github.com/Neuz/DevToys/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// collapsedSidebarWidth 是侧栏折叠成图标轨道时的宽度。
const collapsedSidebarWidth float32 = 48

// Options 配置主窗口。
type Options struct {
	Name, Version string
	Modules       []contract.Module
	DefaultID     string
	SidebarWidth  int
	ThemeMode     func() config.ThemeMode // 读取当前主题选择
	Tray          *Tray                   // 非空：常驻托盘，关主窗口只隐藏
	Settings      func(ctl contract.Controller) contract.Settings
	OnSwitch      func(id string)        // 当前页面变化时回调，用于持久化
	OnThemeChange func(config.ThemeMode) // 标题栏切换主题时回调，由 app 保存
	Log           *slog.Logger           // nil 时使用 slog.Default()
}

// Tray 配置托盘图标与启动器面板。
type Tray struct {
	Icon    []byte
	ToolTip string
	Width   int
	Panel   func(ctl contract.Controller) contract.Panel
}

// Shell 持有窗口与各功能的运行时状态。
type Shell struct {
	opts    Options
	modules []contract.Module
	byID    map[string]contract.Module

	current      string
	active       string
	win          *mygo.Window
	runners      []*task.Runner
	settings     contract.Settings
	settingsOpen bool

	// 侧栏与标题栏由外壳持有的界面状态。
	sidebarWidth   float32 // 展开宽度，分隔条拖动会改写它
	collapsedWidth float32 // 折叠后的图标轨道宽度
	paneSize       float32 // 交给 Scaffold 的当前宽度
	collapsed      bool
	themeOpen      bool
	langOpen       bool
	themeMode      config.ThemeMode
	localeTag      string
	themeState     form.RadioGroupState[config.ThemeMode]
	langState      form.RadioGroupState[string]

	log       *slog.Logger
	quitting  bool
	closeOnce sync.Once

	tray        *mygo.Tray
	fly         *mygo.Window
	panel       contract.Panel
	panelHeight float32
}

// Run 注册外壳的回调；mygo.App.Run 仍由调用方执行。
func Run(opts Options) error {
	if len(opts.Modules) == 0 {
		return errors.New("shell: no modules")
	}
	s := newShell(opts)

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

	s.sidebarWidth = float32(opts.SidebarWidth)
	if s.sidebarWidth <= 0 {
		s.sidebarWidth = 256
	}
	s.collapsedWidth = collapsedSidebarWidth
	s.paneSize = s.sidebarWidth
	s.themeMode = config.ThemeSystem
	if opts.ThemeMode != nil {
		s.themeMode = normalizeThemeMode(opts.ThemeMode())
	}
	s.themeState = form.RadioGroupState[config.ThemeMode]{Selected: &s.themeMode, OnChange: s.changeTheme}
	s.langState = form.RadioGroupState[string]{Selected: &s.localeTag, OnChange: s.changeLocale}
	if opts.Settings != nil {
		s.settings = opts.Settings(controller{s})
	}
	return s
}

func (s *Shell) ready() {
	s.localeTag = mygo.App.ChosenLocale()
	s.win = mygo.NewWindow(mygo.WindowOptions{
		Title:          s.opts.Name,
		Width:          1200,
		Height:         780,
		MinWidth:       900,
		MinHeight:      600,
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
		s.localeTag = mygo.App.ChosenLocale()
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
	s.activate(s.current)
}

func (s *Shell) view(c *ui.Context) {
	theme.Apply(c)
	if s.opts.ThemeMode != nil {
		s.themeMode = normalizeThemeMode(s.opts.ThemeMode())
	}
	s.syncSidebarPane()

	layout.Scaffold(c, layout.ScaffoldOptions{
		StartBar: layout.TitleBarOptions{Title: s.opts.Name},
		TitleBar: layout.TitleBarOptions{
			MinHeight: titleBarHeight,
			Title:     s.currentTitle(),
			Trailing:  func() { s.titleBarTrailing(c) },
		},
		Start: &layout.PaneOptions{
			Label: locales.Tools(),
			Size:  &s.paneSize,
			Min:   s.collapsedWidth,
			Body:  func() { s.sidebar(c) },
		},
		Content: func() {
			if m := s.byID[s.current]; m != nil {
				m.View(c)
			}
		},
	})

	if s.settings != nil {
		s.settings.View(c, &s.settingsOpen)
	}
}

// sidebar 绘制模块导航；条目随模块注册表增长。
func (s *Shell) sidebar(c *ui.Context) {
	items := make([]mygoui.SidebarItemOptions, 0, len(s.modules))
	for _, m := range s.modules {
		icon := lucide.Wrench()
		if im, ok := m.(contract.Iconed); ok && im.Icon() != nil {
			icon = im.Icon()
		}
		items = append(items, mygoui.SidebarItemOptions{
			ID:    m.ID(),
			Label: m.Title(),
			Icon:  icon,
		})
	}
	mygoui.Sidebar(c, mygoui.SidebarOptions{
		Selected:       &s.current,
		Collapsed:      &s.collapsed,
		Width:          s.sidebarWidth,
		CollapsedWidth: s.collapsedWidth,
		Groups: []mygoui.SidebarGroupOptions{{
			Key:   "modules",
			Label: locales.Tools(),
			Items: items,
		}},
		OnSelect:         s.activate,
		OnCollapseChange: s.collapseSidebar,
	})
}

// syncSidebarPane 让侧栏面板的宽度跟随折叠状态：展开时使用（可能被分隔条
// 拖动改写的）展开宽度，折叠时收成图标轨道宽度，这样内容区会跟着一起动。
func (s *Shell) syncSidebarPane() {
	if s.collapsed {
		s.paneSize = s.collapsedWidth
		return
	}
	s.sidebarWidth = s.paneSize
}

// collapseSidebar 记录折叠切换，并保留展开时的宽度供再次展开使用。
func (s *Shell) collapseSidebar(collapsed bool) {
	if collapsed {
		s.sidebarWidth = s.paneSize
	} else {
		s.paneSize = s.sidebarWidth
	}
	s.collapsed = collapsed
}

// currentTitle 是标题栏显示的当前模块标题。
func (s *Shell) currentTitle() string {
	if m := s.byID[s.current]; m != nil {
		return m.Title()
	}
	return s.opts.Name
}

// normalizeThemeMode 把未知的主题选择退回跟随系统。
func normalizeThemeMode(mode config.ThemeMode) config.ThemeMode {
	switch mode {
	case config.ThemeLight, config.ThemeDark, config.ThemeSystem:
		return mode
	default:
		return config.ThemeSystem
	}
}

// invalidateAll 让主窗口和托盘面板按新的主题或语言重绘。
func (s *Shell) invalidateAll() {
	if s.win != nil {
		s.win.Invalidate()
	}
	if s.fly != nil {
		s.fly.Invalidate()
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
	if s.opts.OnSwitch != nil {
		s.opts.OnSwitch(id)
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

func (s *Shell) openSettings() {
	if s.settings != nil {
		s.settings.Open()
	}
	s.settingsOpen = true
	if s.win == nil {
		return
	}
	mygo.RunOnMain(func() {
		mygo.App.SetActivationPolicy(mygo.ActivationPolicyRegular)
		s.win.Show()
		s.win.Focus()
		s.win.Invalidate()
	})
}

// close 关闭所有 runner 并销毁托盘；由 OnBeforeQuit 调用。
func (s *Shell) close() {
	s.closeOnce.Do(func() {
		for _, r := range s.runners {
			r.Close()
		}
		if s.tray != nil {
			s.tray.Destroy()
		}
	})
}

// controller 把外壳的能力暴露给功能。
type controller struct{ s *Shell }

func (c controller) ShowMain(id string) { c.s.showMain(id) }

func (c controller) OpenSettings() { c.s.openSettings() }

func (c controller) Quit() { mygo.App.Quit() }

// host 把 runner 与日志器交给功能模块或面板。
type host struct {
	tasks *task.Runner
	log   *slog.Logger
}

func (h host) Tasks() *task.Runner { return h.tasks }

func (h host) Log() *slog.Logger { return h.log }
