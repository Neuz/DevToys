package launcher

import "github.com/egoist/mygo"

// launchByPath 供列表的点击/回车激活使用，按路径找条目。
func (m *Module) launchByPath(path string) {
	for _, e := range m.entries {
		if e.Path == path {
			m.launchEntry(e)
			return
		}
	}
}

// launchEntry 启动条目；先收起面板把键盘交还给上一个窗口，成功后才记使用记录。
// 系统设置条目用 ms-settings URI 交给系统打开，应用条目走 Shell.LaunchApp。
func (m *Module) launchEntry(e Entry) {
	m.hidePanel()
	var err error
	if e.Category == categorySystem {
		err = mygo.Shell.OpenExternal(e.Path)
	} else {
		err = mygo.Shell.LaunchApp(mygo.InstalledApp{Name: e.Title, ID: e.ID, Path: e.Path})
	}
	if err != nil {
		if m.log != nil {
			m.log.Error("launcher: 启动失败", "path", e.Path, "error", err)
		}
		return
	}
	m.recordUsage(e.Path)
}
