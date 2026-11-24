// Package translate 提供 Windows 划词翻译：读取前台选区后显示浮窗，
// 也可以用全局快捷键直接打开并手动输入。翻译请求并行发往已启用的服务。
package translate

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Cyenoch/mygoui"
	"github.com/Cyenoch/mygoui/form"
	"github.com/Cyenoch/mygoui/list"
	"github.com/Neuz/DevToys/internal/contract"
	"github.com/Neuz/DevToys/internal/features/translate/locales"
	"github.com/Neuz/DevToys/internal/hotkey"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// defaultHotkeyAccel 是唤起翻译面板的默认快捷键。
const defaultHotkeyAccel = "Alt+T"

// Module 实现 contract.Module、contract.Toggleable、contract.GlobalHotkeyer
// 与 contract.Closer。
type Module struct {
	host    contract.Host
	log     *slog.Logger
	flyouts mygoui.FlyoutHost

	opts         options
	trigger      form.Hotkey
	target       form.SelectState
	source       string
	sourceSelect form.SelectState
	targetSelect form.SelectState

	lifeCtx    context.Context
	lifeCancel context.CancelFunc
	service    *Service

	panel        *mygo.Window
	panelHeight  int
	textScale    float32
	query        string
	input        ui.Handle
	cardsScroll  ui.ScrollState
	panelFlyouts mygoui.FlyoutHost
	collapsed    map[string]bool
	pinned       bool
	notice       string
	cards        []card
	gen          uint64
	copiedID     string
	copiedSeq    uint64
	copyTimer    *time.Timer
	debounce     *time.Timer
	debounceSeq  uint64
}

// New 返回翻译模块。
func New() *Module { return &Module{} }

func (m *Module) ID() string { return "translate" }

// Group 把翻译放在通用工具分组里，与启动器一致。
func (m *Module) Group() string { return "" }

func (m *Module) Title() string { return locales.T("title") }

// Icon 让折叠后的侧栏显示为语言图标。
func (m *Module) Icon() *ui.SVG { return lucide.Languages() }

// Init 绑定外壳、读取配置并准备翻译服务。
func (m *Module) Init(h contract.Host) {
	m.host = h
	m.log = h.Log()
	m.flyouts.Window = h.Window()
	m.lifeCtx, m.lifeCancel = context.WithCancel(context.Background())
	m.opts = decodeOptions(h.Feature())
	m.service = NewService(m.lifeCtx, m.opts)
	m.trigger = hotkey.Parse(m.accelerator())
	m.target = form.SelectState{List: list.ListBoxState{SelectedKey: m.opts.Target}}
	m.source = langAuto
	m.sourceSelect = form.SelectState{List: list.ListBoxState{SelectedKey: langAuto}}
	m.targetSelect = form.SelectState{List: list.ListBoxState{SelectedKey: m.opts.Target}}
	m.collapsed = make(map[string]bool)
}

// Close 释放面板、定时器与后台请求。
func (m *Module) Close() {
	m.hidePanel()
	if m.copyTimer != nil {
		m.copyTimer.Stop()
		m.copyTimer = nil
	}
	if m.panel != nil {
		m.panel.Destroy()
		m.panel = nil
	}
	if m.lifeCancel != nil {
		m.lifeCancel()
	}
}

// Enabled 报告翻译是否启用；禁用时外壳不会注册它的全局快捷键。
func (m *Module) Enabled() bool {
	if m.host == nil {
		return true
	}
	return m.host.Feature().Enabled
}

// Hotkeys 声明唤起翻译面板的全局快捷键。
func (m *Module) Hotkeys() []contract.Hotkey {
	return []contract.Hotkey{{
		ID:          "translate",
		Default:     defaultHotkeyAccel,
		Description: locales.T("title"),
		Run:         m.toggle,
	}}
}

// accelerator 返回当前生效的快捷键。键缺失时用默认值；存在但为空表示
// 用户已删除，不再回退默认。
func (m *Module) accelerator() string {
	if m.host == nil {
		return defaultHotkeyAccel
	}
	acc, ok := m.host.Feature().Hotkeys["translate"]
	if !ok {
		return defaultHotkeyAccel
	}
	return acc
}

// changeTrigger 保存新快捷键并让外壳立即重绑。
func (m *Module) changeTrigger() {
	if m.host == nil {
		return
	}
	node := m.host.Feature()
	if node.Hotkeys == nil {
		node.Hotkeys = make(map[string]string)
	}
	prev, had := node.Hotkeys["translate"]
	node.Hotkeys["translate"] = hotkey.Format(m.trigger)
	if err := m.host.SaveConfig(); err != nil {
		if had {
			node.Hotkeys["translate"] = prev
		} else {
			delete(node.Hotkeys, "translate")
		}
		m.host.Log().Error("translate: 保存快捷键", "error", err)
		return
	}
	m.host.HotkeysChanged()
}

// changeOptions 把设置页的选项写回配置；失败时恢复界面状态。
func (m *Module) changeOptions() {
	if m.host == nil {
		return
	}
	data, err := json.Marshal(m.opts)
	if err != nil {
		m.host.Log().Error("translate: 编码配置", "error", err)
		return
	}
	node := m.host.Feature()
	prev := node.Options
	node.Options = data
	if err := m.host.SaveConfig(); err != nil {
		node.Options = prev
		m.opts = decodeOptions(node)
		m.target.List.SelectedKey = m.opts.Target
		m.targetSelect.List.SelectedKey = m.opts.Target
		m.host.Window().Invalidate()
		m.host.Log().Error("translate: 保存配置", "error", err)
		return
	}
	m.applyOptions()
}

// applyOptions 用新配置重建服务，并在面板可见时重译当前文本。
func (m *Module) applyOptions() {
	if m.lifeCtx == nil {
		return
	}
	if m.service != nil {
		m.service.Cancel()
	}
	m.gen = 0
	m.service = NewService(m.lifeCtx, m.opts)
	m.target.List.SelectedKey = m.opts.Target
	m.targetSelect.List.SelectedKey = m.opts.Target
	if m.panel == nil || !m.panel.IsVisible() {
		return
	}
	if strings.TrimSpace(m.query) != "" {
		m.translateNow()
		return
	}
	m.cards = nil
	m.panel.Invalidate()
}

// setEnabled 把开关状态写回配置，并让外壳立即生效。
func (m *Module) setEnabled(enabled bool) {
	if m.host == nil {
		return
	}
	if err := m.host.SetEnabled(enabled); err != nil {
		m.host.Log().Error("translate: 保存启用状态", "error", err)
	}
}
