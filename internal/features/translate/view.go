package translate

import (
	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Cyenoch/mygoui"
	"github.com/Cyenoch/mygoui/form"
	"github.com/Cyenoch/mygoui/theme"
	"github.com/Neuz/DevToys/internal/features/translate/locales"
	"github.com/egoist/mygo/ui"
)

// View 绘制翻译设置页：启用、快捷键、服务、语言与隐私说明。
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
					Icon:  lucide.Languages(),
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
			},
		})
		mygoui.SettingsGroup(c, mygoui.SettingsGroupOptions{
			Title: locales.T("settings_services"),
			Rows: []mygoui.SettingsRow{
				{
					Icon:  lucide.Globe(),
					Title: locales.T("settings_google"),
					Trailing: func() {
						form.Switch(c, form.SwitchOptions{
							On: &m.opts.Providers.Google,
							OnChange: func(value bool) {
								m.opts.Providers.Google = value
								m.changeOptions()
							},
						})
					},
				},
				{
					Icon:     lucide.Languages(),
					Title:    locales.T("settings_microsoft"),
					Subtitle: locales.T("settings_microsoft_hint"),
					Trailing: func() {
						form.Switch(c, form.SwitchOptions{
							On: &m.opts.Providers.Microsoft,
							OnChange: func(value bool) {
								m.opts.Providers.Microsoft = value
								m.changeOptions()
							},
						})
					},
				},
				{
					Icon:  lucide.Video(),
					Title: locales.T("settings_bilibili"),
					Trailing: func() {
						form.Switch(c, form.SwitchOptions{
							On: &m.opts.Providers.Bilibili,
							OnChange: func(value bool) {
								m.opts.Providers.Bilibili = value
								m.changeOptions()
							},
						})
					},
				},
				{
					Icon:  lucide.KeyRound(),
					Title: locales.T("settings_microsoft_key"),
					Trailing: func() {
						form.PasswordField(c, &m.opts.Providers.MicrosoftAPIKey, form.PasswordFieldOptions{
							AriaLabel: locales.T("settings_microsoft_key"),
							OnChange:  m.changeOptions,
						}).Width(theme.Scale(c, 220))
					},
				},
				{
					Icon:  lucide.MapPin(),
					Title: locales.T("settings_microsoft_region"),
					Trailing: func() {
						form.Input(c, &m.opts.Providers.MicrosoftRegion, form.InputOptions{
							AriaLabel: locales.T("settings_microsoft_region"),
							OnChange:  m.changeOptions,
						}).Width(theme.Scale(c, 180))
					},
				},
			},
		})
		mygoui.SettingsGroup(c, mygoui.SettingsGroupOptions{
			Rows: []mygoui.SettingsRow{
				{
					Icon:  lucide.Languages(),
					Title: locales.T("settings_target"),
					Trailing: func() {
						m.renderTarget(c)
					},
				},
				{
					Icon:  lucide.MousePointer2(),
					Title: locales.T("settings_follow_cursor"),
					Trailing: func() {
						form.Switch(c, form.SwitchOptions{
							On: &m.opts.PanelFollowsCursor,
							OnChange: func(value bool) {
								m.opts.PanelFollowsCursor = value
								m.changeOptions()
							},
						})
					},
				},
			},
		})
		mygoui.Typography(c, locales.T("settings_privacy"), mygoui.TypographyOptions{
			Type:  theme.TypeBodySM,
			Muted: true,
		})
		if !m.hasProvider() {
			mygoui.Typography(c, locales.T("settings_no_provider"), mygoui.TypographyOptions{
				Type:  theme.TypeBodySM,
				Muted: true,
			})
		}
		if !enabled {
			mygoui.Typography(c, locales.T("settings_disabled_hint"), mygoui.TypographyOptions{
				Type:  theme.TypeBodySM,
				Muted: true,
			})
		}
	})
}

// renderTarget 绘制目标语言选择器。
func (m *Module) renderTarget(c *ui.Context) {
	form.Select(c, form.SelectOptions{
		Items:  targetItems(),
		State:  &m.target,
		Flyout: &m.flyouts,
		OnChange: func(keys []string) {
			if len(keys) == 0 {
				return
			}
			m.opts.Target = keys[0]
			m.changeOptions()
		},
	}).Width(theme.Scale(c, 180))
}

// hasProvider 报告是否至少启用了一个服务。
func (m *Module) hasProvider() bool {
	return m.opts.Providers.Google || m.opts.Providers.Microsoft || m.opts.Providers.Bilibili
}

// targetLabel 返回目标语言的显示名。
func targetLabel(tag string) string {
	if tag == langAuto {
		return locales.T("target_auto")
	}
	switch tag {
	case langChinese:
		return locales.T("target_zh_hans")
	case langEnglish:
		return locales.T("target_en")
	case "ja":
		return locales.T("target_ja")
	case "ko":
		return locales.T("target_ko")
	case "ru":
		return locales.T("target_ru")
	case "fr":
		return locales.T("target_fr")
	case "de":
		return locales.T("target_de")
	case "es":
		return locales.T("target_es")
	default:
		return tag
	}
}
