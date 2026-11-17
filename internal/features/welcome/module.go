// Package welcome 是应用启动后显示的功能页。
package welcome

import (
	"log/slog"

	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Neuz/DevToys/internal/contract"
	"github.com/Neuz/DevToys/internal/features/welcome/locales"
	"github.com/Neuz/DevToys/internal/task"
	"github.com/egoist/mygo/ui"
)

// Module 实现 contract.Module。
type Module struct {
	tasks *task.Runner
	log   *slog.Logger
}

// New 返回欢迎页。
func New() *Module { return &Module{} }

func (m *Module) ID() string { return "welcome" }

// Group 把欢迎页放在通用工具分组里。
func (m *Module) Group() string { return "" }

func (m *Module) Title() string { return locales.T("title") }

// Icon 让折叠后的侧栏显示为图标轨道。
func (m *Module) Icon() *ui.SVG { return lucide.House() }

func (m *Module) Init(h contract.Host) {
	m.tasks = h.Tasks()
	m.log = h.Log()
}
