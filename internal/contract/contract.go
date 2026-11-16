// Package contract 定义外壳与功能之间的唯一契约。
//
// 功能模块只依赖这里的接口，外壳也只通过它们驱动功能，因此两侧可以各自
// 演进：新增功能不必修改外壳，新增外壳行为也不必让功能知道。
package contract

import (
	"log/slog"

	"github.com/Neuz/DevToys/internal/task"
	"github.com/egoist/mygo/ui"
)

// Host 是外壳交给功能的运行时设施。
type Host interface {
	// Tasks 返回属于该功能的 runner，后台结果经它回到主线程。
	Tasks() *task.Runner
	// Log 返回带应用标识的日志器。
	Log() *slog.Logger
}

// Module 是主窗口中的一个功能页面。
type Module interface {
	ID() string
	Title() string
	Init(h Host)
	View(c *ui.Context)
}

// Live 是模块可选实现的能力：外壳在切换页面时通知它。
type Live interface {
	OnActivate()
	OnDeactivate()
}

// Iconed 是模块可选实现的能力：返回导航栏使用的图标。
// 未实现时外壳使用通用工具图标。
type Iconed interface {
	Icon() *ui.SVG
}

// Panel 是托盘启动器面板，由 mygo.Flyout 承载。
type Panel interface {
	Init(h Host)
	Show()
	Hide()
	View(c *ui.Context)
	Height() int
}

// Settings 是设置面板，绘制在主窗口的遮罩层之上。
type Settings interface {
	Open()
	View(c *ui.Context, open *bool)
}

// Controller 让功能反向控制外壳。
type Controller interface {
	// ShowMain 显示主窗口并切换到 id 对应的模块。
	ShowMain(id string)
	OpenSettings()
	Quit()
}
