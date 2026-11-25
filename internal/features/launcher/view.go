package launcher

import (
	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Cyenoch/mygoui"
	"github.com/Cyenoch/mygoui/form"
	"github.com/Cyenoch/mygoui/overlay"
	"github.com/Cyenoch/mygoui/theme"
	"github.com/Neuz/DevToys/internal/features/launcher/locales"
	"github.com/egoist/mygo/ui"
)

// View 绘制启动器的设置页：启用开关、触发快捷键、手动触发与索引信息。
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
					Icon:  lucide.ListOrdered(),
					Title: locales.T("settings_max_results"),
					Trailing: func() {
						form.NumberField(c, &m.maxResults, form.NumberFieldOptions{
							AriaLabel: locales.T("settings_max_results"),
							OnChange:  m.changeOptions,
						}).Width(theme.Scale(c, 120))
					},
				},

				{
					Icon:  lucide.Play(),
					Title: locales.T("settings_trigger"),
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
					Icon:  lucide.Database(),
					Title: locales.T("settings_index"),
					Trailing: func() {
						mygoui.Button(c, mygoui.ButtonOptions{
							Label:   locales.T("settings_rebuild"),
							Icon:    lucide.RefreshCw(),
							Variant: mygoui.ButtonOutline,
							Size:    theme.ButtonSmall,
							OnClick: func() { m.rebuildIndex(true) },
						})
					},
				},
				{
					Icon:  lucide.Filter(),
					Title: locales.T("settings_exclude"),
					Trailing: func() {
						form.TagsInput(c, &m.excludeTags, form.TagsInputOptions{
							AriaLabel: locales.T("settings_exclude"),
							OnChange:  m.changeOptions,
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
	// 索引重建完成后在帧里采用结果并弹一次 Toast。
	m.applyIndexResult()
	m.notifyIndexResult(c)
	m.toasts.View(c)
}

// notifyIndexResult 在索引重建结束时弹一次 Toast：成功或失败各一句。
// 只有设置页按钮触发的重建才提示，启动时的自动重建保持静默。
func (m *Module) notifyIndexResult(c *ui.Context) {
	r := m.manualIndex
	if r == nil || r.Running() {
		return
	}
	m.manualIndex = nil
	opts := overlay.ToastOptions{
		ID:      "launcher-index",
		Title:   locales.T("toast_index_ok"),
		Variant: overlay.ToastSuccess,
	}
	if err := r.Err(); err != nil {
		opts.Title = locales.T("toast_index_failed")
		opts.Description = err.Error()
		opts.Variant = overlay.ToastDanger
	} else if _, ok := r.Value(); !ok {
		// 结果被取消或丢弃时没有可提示的内容。
		return
	}
	overlay.Toast(c, &m.toasts, opts)
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
