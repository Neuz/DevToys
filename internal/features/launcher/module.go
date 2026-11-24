// Package launcher 是全局快速启动器：用快捷键唤起一个浮层面板，在面板里
// 输入关键字查找并启动开始菜单或桌面上的快捷方式。
//
// 文件分工：module.go 是契约与状态，index.go 扫描并缓存条目，search.go
// 负责过滤、排序与使用记录，store.go 用 bbolt 持久化索引与使用记录，
// panel.go 管理面板窗口的生命周期，view.go 渲染设置页，panel_view.go 渲染
// 面板内容并处理键盘，launch.go 启动条目。
package launcher

import (
	"encoding/json"
	"log/slog"
	"slices"

	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Cyenoch/mygoui/form"
	"github.com/Cyenoch/mygoui/list"
	"github.com/Cyenoch/mygoui/overlay"
	"github.com/Neuz/DevToys/internal/contract"
	"github.com/Neuz/DevToys/internal/features/launcher/locales"
	"github.com/Neuz/DevToys/internal/hotkey"
	"github.com/Neuz/DevToys/internal/task"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// defaultHotkeyAccel 是唤起启动器的默认 accelerator，用户可在配置里覆盖。
const defaultHotkeyAccel = "Alt+F"

// 面板尺寸：宽度固定，高度按结果条数自适应（见 panel_view.go 的 panelHeightForScale）。
const (
	panelWidth = 680

	// panelInputHeight 是搜索框的固定高度（DIP），用于估算面板高度。
	panelInputHeight = 36
	// panelRowHeight 是每条结果行的固定高度：SM 20 + XS 16 + 上下内边距 8（DIP）。
	// 行内两行文字由 ListBox 自己排版，高度不能小于它，否则末行会被裁掉。
	panelRowHeight = 44
	// panelMaxRows 是列表不滚动时最多展示的行数。
	panelMaxRows = 8
	// panelTabsHeight 是分类分段控件的高度：30 的滑片 + 上下各 2 的内边距（DIP）。
	panelTabsHeight = 34
)

// Module 实现 contract.Module、contract.Toggleable、contract.GlobalHotkeyer
// 与 contract.Closer。
type Module struct {
	host contract.Host

	// panel 是首次唤起时才创建的面板窗口；隐藏不销毁，之后复用。
	panel *mygo.Window
	// panelHeight 是面板当前高度（DIP），用于跳过重复 SetSize。
	panelHeight int
	// fit 是上一帧用于估算面板高度的输入，输入不变时不再重算。
	fit panelFit
	// listMaxHeight 是与 fit 对应的列表最大高度。
	listMaxHeight float32
	// textScale 是面板最近一帧的文字缩放，供 showPanel 估算高度。
	textScale float32
	// query 是面板搜索框的当前文本。
	query string
	// category 是面板当前分类：空表示全部，否则是 categoryApp / categorySystem。
	category string
	// input 是面板搜索框的焦点句柄。
	input ui.Handle
	// list 是面板结果列表的高亮与滚动状态。
	list list.ListBoxState
	// icons 缓存已解码的应用图标，key 是条目的 Path；nil 表示没有可用图标。
	icons map[string]*ui.Bitmap
	// trigger 是设置页里录制的触发快捷键，初值取当前生效值。
	trigger form.Hotkey
	// maxResults 是设置页【显示结果数量】的输入状态。
	maxResults form.NumberFieldState
	// excludeTags 是设置页【排除关键词】的编辑缓冲，保存时写回配置。
	excludeTags []string

	// entries 是扫描到的可启动条目，主线程读写。
	entries []Entry
	// usage 是各条目的启动次数，主线程读写。
	usage map[string]Usage
	// tasks 是外壳分配给本模块的后台任务 runner。
	tasks *task.Runner
	// index 是最近一次后台重建索引的任务；结果在面板帧里采用。
	index *task.Result[[]Entry]
	// appliedIndex 记录已采用过结果的任务，避免重复应用与重复记日志。
	appliedIndex *task.Result[[]Entry]
	// manualIndex 是需要弹 Toast 的那次重建；nil 表示无需提示。
	manualIndex *task.Result[[]Entry]
	// toasts 是设置页的 Toast 状态。
	toasts overlay.ToastState
	// log 是带模块标识的日志器。
	log *slog.Logger
	// store 持久化索引与使用记录；打开失败时为 nil，降级为不持久化。
	store *store
}

// New 返回启动器。
func New() *Module { return &Module{} }

func (m *Module) ID() string { return "launcher" }

// Group 把启动器放在通用工具分组里，与欢迎页一致。
func (m *Module) Group() string { return "" }

func (m *Module) Title() string { return locales.T("title") }

// Icon 让折叠后的侧栏显示为搜索图标。
func (m *Module) Icon() *ui.SVG { return lucide.Search() }

// Init 绑定外壳、打开索引数据库，并加载缓存后触发一次后台重建。
func (m *Module) Init(h contract.Host) {
	m.host = h
	m.log = h.Log()
	m.tasks = h.Tasks()
	if st, err := openStore(); err != nil {
		m.log.Warn("launcher: 打开索引数据库", "error", err)
	} else {
		m.store = st
	}
	m.trigger = hotkey.Parse(m.accelerator())
	m.icons = make(map[string]*ui.Bitmap)
	opts := m.options()
	m.maxResults.Value = float64(opts.MaxResults)
	m.excludeTags = append([]string(nil), opts.Exclude...)
	m.loadUsage()
	// 启动时先同步读缓存，随即在后台核对目录修改时间并按需重建。
	m.entries = m.loadCachedEntries()
	m.rebuildIndex(false)
}

// Close 释放面板窗口与索引数据库；由外壳在退出时调用。
func (m *Module) Close() {
	if m.panel != nil {
		m.panel.Destroy()
		m.panel = nil
	}
	if m.store != nil {
		if err := m.store.Close(); err != nil && m.log != nil {
			m.log.Debug("launcher: 关闭索引数据库", "error", err)
		}
	}
}

// Enabled 报告启动器是否启用；禁用时外壳不会注册它的全局快捷键。
func (m *Module) Enabled() bool {
	if m.host == nil {
		return true
	}
	return m.host.Feature().Enabled
}

// Hotkeys 声明唤起启动器的全局快捷键，由外壳统一注册。
func (m *Module) Hotkeys() []contract.Hotkey {
	return []contract.Hotkey{{
		ID:          "toggle",
		Default:     defaultHotkeyAccel,
		Description: locales.T("title"),
		Run:         m.toggle,
	}}
}

// accelerator 返回当前生效的快捷键。配置里没有这个键时用默认值；
// 键存在但值为空表示用户已删除，返回空串，不再回退默认。
func (m *Module) accelerator() string {
	if m.host == nil {
		return defaultHotkeyAccel
	}
	acc, ok := m.host.Feature().Hotkeys["toggle"]
	if !ok {
		return defaultHotkeyAccel
	}
	return acc
}

// changeTrigger 保存新录制的快捷键并让外壳立即重绑；清空则显式删除，
// 面板以后只能通过设置页的【触发】按钮或侧栏打开。
func (m *Module) changeTrigger() {
	if m.host == nil {
		return
	}
	node := m.host.Feature()
	if node.Hotkeys == nil {
		node.Hotkeys = make(map[string]string)
	}
	prev, had := node.Hotkeys["toggle"]
	node.Hotkeys["toggle"] = hotkey.Format(m.trigger)
	if err := m.host.SaveConfig(); err != nil {
		if had {
			node.Hotkeys["toggle"] = prev
		} else {
			delete(node.Hotkeys, "toggle")
		}
		m.host.Log().Error("launcher: 保存快捷键", "error", err)
		return
	}
	m.host.HotkeysChanged()
}

// changeOptions 把设置页的索引选项写回配置。排除词在建索引时生效，
// 所以它变化后需要重建；结果条数只影响查询，无需重建。
func (m *Module) changeOptions() {
	if m.host == nil {
		return
	}
	current := m.options()
	opts := indexOptions{
		MaxResults: int(m.maxResults.Value),
		Exclude:    m.excludeTags,
	}.normalized()
	data, err := json.Marshal(opts)
	if err != nil {
		m.host.Log().Error("launcher: 编码索引配置", "error", err)
		return
	}
	node := m.host.Feature()
	prev := node.Options
	node.Options = data
	if err := m.host.SaveConfig(); err != nil {
		node.Options = prev
		m.host.Log().Error("launcher: 保存索引配置", "error", err)
		return
	}
	if !slices.Equal(opts.Exclude, current.Exclude) {
		m.rebuildIndex(false)
	}
}
