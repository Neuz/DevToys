// Package screenshot 提供QQ 截图式截屏：冻结屏幕、框选或点选窗口、
// 矩形 / 箭头 / 马赛克标注，完成后写入剪贴板或另存为文件。
//
// 后台任务遵循架构约束：抓屏 / 渲染 / 剪贴板 / 另存为走 task.Runner；
// Win32 阻塞调用按特例在调用方的 Runner goroutine 里执行
// （见 capture_windows.go）。
package screenshot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Cyenoch/mygoui"
	"github.com/Cyenoch/mygoui/form"
	"github.com/Cyenoch/mygoui/theme"
	"github.com/Neuz/DevToys/internal/config"
	"github.com/Neuz/DevToys/internal/contract"
	"github.com/Neuz/DevToys/internal/features/screenshot/locales"
	"github.com/Neuz/DevToys/internal/hotkey"
	"github.com/Neuz/DevToys/internal/task"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// ErrUnsupported 表示当前平台不支持该能力。
var ErrUnsupported = fmt.Errorf("当前平台不支持")

// defaultHotkeyAccel 是触发截图的默认快捷键。
const defaultHotkeyAccel = "Ctrl+Alt+A"

// options 是截图功能的用户配置。
type options struct{}

// defaultOptions 返回首次运行的默认配置；首版没有可配置项。
func defaultOptions() options { return options{} }

// decodeOptions 从功能节点读出配置；缺失字段保留默认值。
func decodeOptions(node *config.Feature) options {
	opts := defaultOptions()
	if node != nil && len(node.Options) > 0 {
		_ = json.Unmarshal(node.Options, &opts)
	}
	return opts
}

// normalizeAccelerator 清理配置里的快捷键字符串。
func normalizeAccelerator(acc string) string {
	return strings.TrimSpace(acc)
}

// trigger 是 form.Hotkey 的别名，用于设置页绑定。
type trigger = form.Hotkey
type Module struct {
	host    contract.Host
	log     *slog.Logger
	trigger trigger

	opts       options
	lifeCtx    context.Context
	lifeCancel context.CancelFunc
	tasks      *task.Runner

	overlays  []*overlay
	capturing bool
}

// New 返回截图模块。
func New() *Module { return &Module{} }

func (m *Module) ID() string { return "screenshot" }

// Group 把截图放在通用工具分组里，与启动器、翻译一致。
func (m *Module) Group() string { return "" }

func (m *Module) Title() string { return locales.T("title") }

// Icon 让侧栏显示裁剪图标。
func (m *Module) Icon() *ui.SVG { return lucide.Crop() }

// Init 绑定外壳、读取配置并准备任务 Runner。
func (m *Module) Init(h contract.Host) {
	m.host = h
	m.log = h.Log()
	m.lifeCtx, m.lifeCancel = context.WithCancel(context.Background())
	m.tasks = task.NewRunner(func(fn func()) { mygo.RunOnMain(fn) })
	m.opts = decodeOptions(h.Feature())
	m.trigger = hotkey.Parse(m.accelerator())
}

// accelerator 返回当前生效的快捷键字符串。键缺失时用默认值；存在但为空
// 表示用户已删除，不再回退默认。
func (m *Module) accelerator() string {
	if m.host == nil {
		return defaultHotkeyAccel
	}
	acc, ok := m.host.Feature().Hotkeys["trigger"]
	if !ok {
		return defaultHotkeyAccel
	}
	return normalizeAccelerator(acc)
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
	prev, had := node.Hotkeys["trigger"]
	node.Hotkeys["trigger"] = hotkey.Format(m.trigger)
	if err := m.host.SaveConfig(); err != nil {
		if had {
			node.Hotkeys["trigger"] = prev
		} else {
			delete(node.Hotkeys, "trigger")
		}
		m.host.Log().Error("screenshot: 保存快捷键", "error", err)
		return
	}
	m.host.HotkeysChanged()
}

// setEnabled 把开关状态写回配置，并让外壳立即生效。
func (m *Module) setEnabled(enabled bool) {
	if m.host == nil {
		return
	}
	if err := m.host.SetEnabled(enabled); err != nil {
		m.host.Log().Error("screenshot: 保存启用状态", "error", err)
	}
}

// Close 关闭所有覆盖层、停掉任务 Runner 并取消生命周期。
func (m *Module) Close() {
	if m.tasks != nil {
		m.tasks.Close()
		m.tasks = nil
	}
	m.closeOverlays()
	if m.lifeCancel != nil {
		m.lifeCancel()
	}
}

// Enabled 报告截图是否启用；禁用时外壳不会注册它的全局快捷键。
// 直接读配置节点（与 launcher/translate 一致）：host.SetEnabled 在
// 重绑快捷键前已更新节点，缓存副本会让重绑读到旧值。
func (m *Module) Enabled() bool {
	if m.host == nil {
		return true
	}
	return m.host.Feature().Enabled
}

// Hotkeys 声明触发截图的全局快捷键。
func (m *Module) Hotkeys() []contract.Hotkey {
	return []contract.Hotkey{{
		ID:          "trigger",
		Default:     defaultHotkeyAccel,
		Description: locales.T("title"),
		Run:         m.toggle,
	}}
}

// View 绘制截图设置页：启用开关与触发快捷键。
func (m *Module) View(c *ui.Context) {
	theme.Apply(c)
	enabled := m.Enabled()
	ui.Column(c).Fill().Padding(theme.Space(2), theme.Space(8)).Gap(theme.Space(5)).Children(func() {
		ui.Column(c).Gap(theme.Space(1)).Children(func() {
			mygoui.Heading(c, int(theme.TypeH1), locales.T("title"))
			mygoui.Typography(c, locales.T("subtitle"), mygoui.TypographyOptions{
				Type: theme.TypeBody,
			})
		})
		mygoui.SettingsGroup(c, mygoui.SettingsGroupOptions{
			Rows: []mygoui.SettingsRow{
				{
					Icon:  lucide.Crop(),
					Title: locales.T("settings_enable"),
					Trailing: func() {
						form.Switch(c, form.SwitchOptions{
							On:       &enabled,
							OnChange: m.setEnabled,
						})
					},
				},
				{
					Icon:  lucide.Keyboard(),
					Title: locales.T("settings_hotkey"),
					Trailing: func() {
						form.HotkeyInput(c, &m.trigger, form.HotkeyInputOptions{
							AriaLabel:   locales.T("settings_hotkey"),
							Placeholder: locales.T("settings_hotkey_unset"),
							Clearable:   true,
							OnChange:    m.changeTrigger,
						}).Width(theme.Scale(c, 200))
					},
				},
			},
		})
	})
}
