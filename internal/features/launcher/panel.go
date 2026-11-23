package launcher

import (
	"time"

	"github.com/Neuz/DevToys/internal/features/launcher/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// blurHideDelay 是面板失去焦点后延迟隐藏的时间，避免点击别的窗口时立刻消失。
const blurHideDelay = 100 * time.Millisecond

// toggle 由全局快捷键调用。回调可能不在主线程，统一回到主线程执行。
func (m *Module) toggle() {
	mygo.RunOnMain(m.toggleMain)
}

// toggleMain 在面板可见时隐藏它，否则创建（首次）并显示。
func (m *Module) toggleMain() {
	if m.panel == nil {
		m.createPanel()
	}
	if m.panel.IsVisible() {
		m.hidePanel()
		return
	}
	m.showPanel()
}

// createPanel 创建一个 Spotlight 式面板：无边框、置顶、不占任务栏，
// 创建时隐藏，之后由 toggle 显示与隐藏。
func (m *Module) createPanel() {
	m.panel = mygo.NewWindow(mygo.WindowOptions{
		Title:         locales.T("title"),
		Panel:         true,
		Frameless:     true,
		AlwaysOnTop:   true,
		SkipTaskbar:   true,
		Hidden:        true,
		Width:         panelWidth,
		Height:        panelHeightForScale(1, 0, false),
		DisableResize: true,
		Content:       ui.View(m.panelView),
	})
	// Esc、Cmd-W、Alt-F4 或点关闭按钮都只隐藏面板；窗口保留到退出。
	m.panel.OnClose(func(e *mygo.CloseEvent) {
		e.PreventDefault()
		m.hidePanel()
	})
	// 失焦后稍等再隐藏，给面板一个重新获得焦点的机会。
	m.panel.OnBlur(func() {
		time.AfterFunc(blurHideDelay, func() {
			mygo.RunOnMain(func() {
				if m.panel != nil && !m.panel.IsFocused() {
					m.hidePanel()
				}
			})
		})
	})
}

// showPanel 把面板显示在鼠标所在显示器的工作区中央，并向上偏移约 20%。
// 高度先回到默认状态（只有搜索框）；之后由视图按结果条数继续调整，顶部位置保持不变。
func (m *Module) showPanel() {
	scale := m.textScale
	if scale <= 0 {
		scale = 1
	}
	height := panelHeightForScale(scale, 0, false)
	m.setPanelHeight(height)
	m.fit = panelFit{scale: scale}
	m.listMaxHeight = panelMaxListHeight(scale)
	d := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	if area := d.WorkArea; area.Width > 0 && area.Height > 0 {
		x := area.X + (area.Width-panelWidth)/2
		y := area.Y + (area.Height-height)/2 - area.Height/5
		m.panel.SetPosition(x, y)
	} else {
		// 拿不到工作区时退回窗口自身的居中，再自行向上偏移。
		m.panel.Center()
		x, y := m.panel.Position()
		m.panel.SetPosition(x, y-height/5)
	}
	m.query = ""
	m.category = ""
	m.list.Highlight = 0
	m.panel.Show()
	m.input.Focus()
}

// setPanelHeight 把面板窗口高度调整到内容高度；高度没变时跳过，避免重复布局。
func (m *Module) setPanelHeight(height int) {
	if m.panel == nil || height <= 0 || height == m.panelHeight {
		return
	}
	m.panelHeight = height
	m.panel.SetSize(panelWidth, height)
}

// panelFit 是估算面板高度所需的输入；相同输入不必重新计算。
type panelFit struct {
	scale float32
	rows  int
	tabs  bool
}

// setPanelHeightFor 只在高度输入变化时重新估算并调整面板高度。
func (m *Module) setPanelHeightFor(fit panelFit) {
	if fit == m.fit {
		return
	}
	m.fit = fit
	m.listMaxHeight = panelMaxListHeight(fit.scale)
	m.setPanelHeight(panelHeightForScale(fit.scale, fit.rows, fit.tabs))
}

// hidePanel 隐藏面板但不销毁窗口，下次唤起时保留窗口本身与内部状态。
func (m *Module) hidePanel() {
	if m.panel != nil {
		m.panel.Hide()
	}
}
