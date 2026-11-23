package launcher

import (
	"fmt"
	"os"

	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Cyenoch/mygoui"
	"github.com/Cyenoch/mygoui/form"
	"github.com/Cyenoch/mygoui/theme"
	"github.com/Neuz/DevToys/internal/features/launcher/locales"
	"github.com/egoist/mygo/ui"
)

// View 绘制启动器的设置页：启用开关、触发快捷键、手动触发与索引信息。
func (m *Module) View(c *ui.Context) {
	theme.Apply(c)
	enabled := m.Enabled()
	ui.Column(c).Fill().Padding(theme.Space(8)).Gap(theme.Space(5)).Children(func() {
		ui.Column(c).Gap(theme.Space(1)).Children(func() {
			mygoui.Heading(c, 1, locales.T("title"))
			mygoui.Typography(c, locales.T("subtitle"), mygoui.TypographyOptions{
				Type: theme.TypeBody,
			})
		})
		mygoui.SettingsGroup(c, mygoui.SettingsGroupOptions{
			Rows: []mygoui.SettingsRow{
				{
					Icon:  lucide.Search(),
					Title: locales.T("settings_enable"),
					Trailing: func() {
						form.Switch(c, form.SwitchOptions{
							On:       &enabled,
							OnChange: m.setEnabled,
						})
					},
				},
				{
					Icon:  lucide.Command(),
					Title: locales.T("settings_hotkey"),
					Trailing: func() {
						form.HotkeyInput(c, &m.trigger, form.HotkeyInputOptions{
							AriaLabel:   locales.T("settings_hotkey"),
							Placeholder: locales.T("settings_hotkey_unset"),
							Clearable:   true,
							OnChange:    m.changeTrigger,
						})
					},
				},
				{
					Icon:     lucide.ListOrdered(),
					Title:    locales.T("settings_max_results"),
					Subtitle: locales.T("settings_max_results_hint"),
					Trailing: func() {
						form.NumberField(c, &m.maxResults, form.NumberFieldOptions{
							AriaLabel: locales.T("settings_max_results"),
							OnChange:  m.changeOptions,
						}).Width(theme.Scale(c, 120))
					},
				},
				{
					Icon:     lucide.Filter(),
					Title:    locales.T("settings_exclude"),
					Subtitle: locales.T("settings_exclude_hint"),
					Trailing: func() {
						form.TagsInput(c, &m.excludeTags, form.TagsInputOptions{
							AriaLabel: locales.T("settings_exclude"),
							OnChange:  m.changeOptions,
						})
					},
				},
				{
					Icon:     lucide.Play(),
					Title:    locales.T("settings_trigger"),
					Subtitle: locales.T("settings_trigger_hint"),
					Trailing: func() {
						mygoui.Button(c, mygoui.ButtonOptions{
							Label:   locales.T("settings_trigger"),
							Icon:    lucide.Play(),
							Variant: mygoui.ButtonOutline,
							Size:    theme.ButtonSmall,
							OnClick: m.toggle,
						})
					},
				},
				{
					Icon:     lucide.Database(),
					Title:    locales.T("settings_index"),
					Subtitle: m.indexSummary(),
					Trailing: func() {
						mygoui.Button(c, mygoui.ButtonOptions{
							Label:   locales.T("settings_rebuild"),
							Icon:    lucide.RefreshCw(),
							Variant: mygoui.ButtonOutline,
							Size:    theme.ButtonSmall,
							OnClick: m.rebuildIndex,
						})
					},
				},
			},
		})
		// 本期没有其他设置；禁用时只多给一句提示，开关本身保留以便重新启用。
		if !enabled {
			mygoui.Typography(c, locales.T("settings_disabled_hint"), mygoui.TypographyOptions{
				Type:  theme.TypeBodySM,
				Muted: true,
			})
		}
	})
}

// indexSummary 返回索引文件的位置与大小，供设置页展示。
func (m *Module) indexSummary() string {
	path, err := dbPath()
	if err != nil {
		return "—"
	}
	size := "—"
	if info, err := os.Stat(path); err == nil {
		size = formatBytes(info.Size())
	}
	return path + " · " + size
}

// formatBytes 把字节数格式化为 B / KB / MB。
func formatBytes(n int64) string {
	switch {
	case n < 1<<10:
		return fmt.Sprintf("%d B", n)
	case n < 1<<20:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	}
}

// setEnabled 把开关状态写回配置，并让外壳立即生效（重新绑定快捷键）。
func (m *Module) setEnabled(enabled bool) {
	if m.host == nil {
		return
	}
	if err := m.host.SetEnabled(enabled); err != nil {
		m.host.Log().Error("launcher: 保存启用状态", "error", err)
	}
}
