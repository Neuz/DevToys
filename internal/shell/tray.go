package shell

import (
	"runtime"

	"github.com/Neuz/DevToys/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
)

// Tray 配置托盘图标。
type Tray struct {
	Icon    []byte
	ToolTip string
}

// setupTray 创建托盘图标；只有右键菜单，单击与双击不做任何事。
func (s *Shell) setupTray() {
	t := s.opts.Tray
	tray, err := mygo.NewTray(mygo.TrayOptions{
		Icon:           t.Icon,
		IconIsTemplate: runtime.GOOS == "darwin",
		ToolTip:        t.ToolTip,
		Menu:           s.trayMenu(),
	})
	if err != nil {
		s.log.Warn("tray: creating the icon", "error", err)
		return
	}
	s.tray = tray
}

// trayMenu 按当前的快捷键与界面语言构建托盘菜单。mygo 刷新菜单时不会
// 重装托盘菜单，所以快捷键或语言变化后要用 Tray.SetMenu 整份换掉。
func (s *Shell) trayMenu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{
			Label:       locales.TrayShowHide(),
			Accelerator: s.hotkeyGlobal,
			Click:       func(*mygo.MenuItem, *mygo.Window) { s.toggleMain() },
		},
		updater.MenuItem(),
		{Type: mygo.MenuItemSeparator},
		{Role: mygo.RoleQuit},
	})
}

// refreshTrayMenu 让托盘菜单跟上快捷键与界面语言。
func (s *Shell) refreshTrayMenu() {
	if s.tray == nil {
		return
	}
	s.tray.SetMenu(s.trayMenu())
}
