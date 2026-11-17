package shell

import (
	"runtime"

	"github.com/egoist/mygo"
)

// Tray 配置托盘图标。
type Tray struct {
	Icon    []byte
	ToolTip string
}

// setupTray 创建托盘图标；点击或右键都回到主窗口。
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

	show := func() { s.showMain(s.current) }
	tray.OnClick(show)
	tray.OnRightClick(show)
}
