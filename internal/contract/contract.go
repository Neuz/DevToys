// Package contract 定义外壳与功能之间的唯一契约。
//
// 功能模块只依赖这里的接口，外壳也只通过它们驱动功能，因此两侧可以各自
// 演进：新增功能不必修改外壳，新增外壳行为也不必让功能知道。
package contract

import (
	"log/slog"

	"github.com/Neuz/DevToys/internal/config"
	"github.com/Neuz/DevToys/internal/task"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Host 是外壳交给功能的运行时设施。
type Host interface {
	// Tasks 返回属于该功能的 runner，后台结果经它回到主线程。
	Tasks() *task.Runner
	// Log 返回带应用标识的日志器。
	Log() *slog.Logger
	// Window 返回功能所在的主窗口，用于把弹出层挂成原生 flyout。
	Window() *mygo.Window
	// Feature 返回本功能的配置节点，功能只应改写自己的节点。
	Feature() *config.Feature
	// SaveConfig 持久化当前配置。
	SaveConfig() error
	// SetEnabled 保存启用状态并让外壳立即生效。
	SetEnabled(enabled bool) error
	// HotkeysChanged 通知外壳重新绑定全局快捷键。
	HotkeysChanged()
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

// Toggleable 是模块可选实现的能力：外壳按它决定是否注册全局快捷键。
// 没有实现它的模块始终启用。
type Toggleable interface {
	Module
	Enabled() bool
}

// GlobalHotkeyer 是模块可选实现的能力：声明自己需要的全局快捷键。
type GlobalHotkeyer interface {
	Hotkeys() []Hotkey
}

// Hotkey 是一个全局快捷键。ID 用于配置与冲突提示，Accel 为空时使用 Default。
type Hotkey struct {
	ID          string
	Default     string
	Description string
	Run         func()
}

// Closer 是模块可选实现的能力：外壳在退出时调用它释放资源。
type Closer interface {
	Close()
}
