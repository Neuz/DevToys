package shell

import (
	"runtime"

	"github.com/Neuz/DevToys/internal/task"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// setupTray 创建托盘图标与启动器 flyout，并把面板的生命周期接上。
func (s *Shell) setupTray() {
	t := s.opts.Tray
	tray, err := mygo.NewTray(mygo.TrayOptions{
		Icon:           t.Icon,
		IconIsTemplate: runtime.GOOS == "darwin",
		ToolTip:        t.ToolTip,
	})
	if err != nil {
		s.log.Warn("tray: creating the icon", "error", err)
		return
	}
	s.tray = tray

	panel := t.Panel(controller{s})
	s.panel = panel
	height := panel.Height()
	if height <= 0 {
		height = 420
	}
	s.panelHeight = float32(height)

	s.fly = mygo.NewFlyout(mygo.FlyoutOptions{
		Tray:         tray,
		Placement:    mygo.PlacementBottom,
		Gap:          4,
		Width:        s.trayWidth(),
		Height:       height,
		Focusable:    true,
		Shadow:       true,
		CornerRadius: 14,
		Hidden:       true,
		Content:      ui.View(s.panelView),
	})

	runner := task.NewRunner(s.fly.Update)
	s.runners = append(s.runners, runner)
	panel.Init(host{tasks: runner, log: s.log.With("module", "launcher")})

	s.fly.OnShow(panel.Show)
	s.fly.OnHide(panel.Hide)
	s.fly.OnClose(func(e *mygo.CloseEvent) {
		e.PreventDefault()
		s.fly.Hide()
	})

	toggle := func() {
		if s.fly.IsVisible() {
			s.fly.Hide()
		} else {
			s.fly.Show()
		}
	}
	tray.OnClick(toggle)
	tray.OnRightClick(toggle)
}

func (s *Shell) panelView(c *ui.Context) {
	if s.panel == nil {
		return
	}
	s.panel.View(c)
	if h := float32(s.panel.Height()); h != s.panelHeight {
		s.panelHeight = h
		s.fly.Update(func() { s.fly.SetSize(s.trayWidth(), int(h)) })
	}
}

func (s *Shell) trayWidth() int {
	if w := s.opts.Tray.Width; w > 0 {
		return w
	}
	return 560
}
