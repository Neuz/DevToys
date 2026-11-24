package qrcode

import (
	"log/slog"
	"time"

	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Cyenoch/mygoui"
	"github.com/Cyenoch/mygoui/color"
	"github.com/Cyenoch/mygoui/form"
	"github.com/Cyenoch/mygoui/list"
	"github.com/Neuz/DevToys/internal/contract"
	"github.com/Neuz/DevToys/internal/features/qrcode/locales"
	"github.com/egoist/mygo/ui"
)

// Module 实现 contract.Module。
type Module struct {
	log *slog.Logger

	qrValue string

	// 生成参数随页面切换保留，回到工具页时仍是上次的选择。
	size       form.NumberFieldState
	level      form.SelectState
	foreground *color.ColorState
	background *color.ColorState
	fgOpen     bool
	bgOpen     bool
	// flyouts 让颜色面板以主窗口的原生 flyout 打开，避免被内容区裁剪。
	flyouts mygoui.FlyoutHost
	// copied 表示复制按钮正在显示“已复制”；copySeq 负责让旧定时器失效。
	copied    bool
	copySeq   uint64
	copyTimer *time.Timer
}

// New 返回QRCode模块。
func New() *Module {
	return &Module{
		size:       form.NumberFieldState{Value: qrDefaultSize},
		level:      form.SelectState{List: list.ListBoxState{SelectedKey: levelMedium}},
		foreground: color.NewColorState(ui.Hex("#000000")),
		background: color.NewColorState(ui.Hex("#ffffff")),
	}
}

func (m *Module) ID() string { return "qrcode" }

// Group 把QRCode模块放在通用工具分组里。
func (m *Module) Group() string { return "" }

func (m *Module) Title() string { return locales.T("title") }

// Icon 让折叠后的侧栏显示为图标轨道。
func (m *Module) Icon() *ui.SVG { return lucide.QRCode() }

func (m *Module) Init(h contract.Host) {
	m.log = h.Log()
	m.flyouts.Window = h.Window()
}
