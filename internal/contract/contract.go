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
	// Group 返回侧栏中的分组名，分组顺序取模块注册顺序中首次出现的位置。
	Group() string
	Title() string
	// Icon 返回导航栏使用的图标；返回 nil 时外壳使用通用工具图标。
	Icon() *ui.SVG
	Init(h Host)
	View(c *ui.Context)
}

// Live 是模块可选实现的能力：外壳在切换页面时通知它。
type Live interface {
	OnActivate()
	OnDeactivate()
}
