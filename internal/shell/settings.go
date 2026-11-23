package shell

import (
	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Cyenoch/mygoui"
	"github.com/Cyenoch/mygoui/form"
	"github.com/Cyenoch/mygoui/layout"
	"github.com/Cyenoch/mygoui/list"
	"github.com/Cyenoch/mygoui/navigation"
	"github.com/Cyenoch/mygoui/theme"
	"github.com/Neuz/DevToys/internal/config"
	"github.com/Neuz/DevToys/internal/hotkey"
	"github.com/Neuz/DevToys/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"
)

// 设置窗口的尺寸。
const (
	settingsWidth  = 580
	settingsHeight = 480
)

// settings 是设置窗口的选择状态，并直接读写配置；窗口关闭后随窗口一起丢弃。
type settings struct {
	shell *Shell
	cfg   *config.Config

	tab int

	// mode 是当前外观选择，theme 单选组把它作为选中值。
	mode  string
	theme form.RadioGroupState[string]

	locale       form.SelectState
	globalHotkey form.Hotkey
	startAtLogin bool
}

// openSettings 打开设置窗口；已打开时只把它提到前面。
func (s *Shell) openSettings() {
	mygo.RunOnMain(func() {
		if s.win == nil {
			return
		}
		if s.settingsWin != nil {
			s.settingsWin.Focus()
			return
		}
		s.settings = newSettings(s)
		s.settingsWin = mygo.NewWindow(mygo.WindowOptions{
			Title:           locales.SettingsTitle(),
			Parent:          s.win,
			Modal:           true,
			Width:           settingsWidth,
			Height:          settingsHeight,
			DisableResize:   true,
			DisableMinimize: true,
			DisableMaximize: true,
			TitleBarStyle:   mygo.TitleBarHidden,
			TitleBarHeight:  int(titleBarHeight),
			Content:         ui.View(s.View),
		})
		s.settingsWin.OnClose(func(*mygo.CloseEvent) {
			s.settingsWin = nil
			s.settings = nil
		})
	})
}

// newSettings 用配置里的当前值作为初始选择。
func newSettings(s *Shell) *settings {
	st := &settings{
		shell:        s,
		cfg:          s.opts.Config,
		mode:         string(s.opts.Config.Theme),
		locale:       form.SelectState{List: list.ListBoxState{SelectedKey: s.opts.Config.Locale}},
		globalHotkey: hotkey.Parse(s.opts.Config.Hotkey.Global.WindowToggle),
		startAtLogin: mygo.App.OpenAtLogin(),
	}
	st.theme = form.RadioGroupState[string]{Selected: &st.mode, OnChange: st.changeTheme}
	return st
}

// View 绘制设置窗口的内容：标题栏、选项卡与关闭按钮。
func (s *Shell) View(c *ui.Context) {
	theme.Apply(c)
	if s.settings == nil {
		return
	}
	st := s.settings

	layout.TitleBar(c, layout.TitleBarOptions{
		Span:       layout.SpanWindow,
		Borderless: false,
		Center: func() {
			bar := c.TitleBar()
			ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
				if inset := bar.Right - bar.Left; inset > 0 {
					ui.Box(c).Width(inset).Shrink(0)
				}
				mygoui.Typography(c, locales.SettingsTitle(), mygoui.TypographyOptions{
					Type:     theme.TypeBodySM,
					Weight:   theme.WeightMedium,
					Truncate: true,
				})
			})
		},
	})

	ui.Column(c).Fill().Padding(theme.Space(6)).Children(func() {
		navigation.Tabs(c, navigation.TabsOptions{
			Selected: &st.tab,
			Items: []navigation.TabOptions{
				{Label: locales.SettingsGeneral(), Content: func() { s.generalSection(c) }},
				{Label: locales.SettingsHotkeys(), Content: func() { s.hotkeysSection(c) }},
			},
		}).Grow(1)
	})
}

// generalSection 是“通用”标签页：外观与语言。
func (s *Shell) generalSection(c *ui.Context) {
	st := s.settings
	ui.Column(c).Gap(theme.Space(4)).Children(func() {
		mygoui.SettingsGroup(c, mygoui.SettingsGroupOptions{
			Rows: []mygoui.SettingsRow{
				{
					Icon:  lucide.SunMoon(),
					Title: locales.SettingsTheme(),
					Trailing: func() {
						form.RadioGroup(c, form.RadioGroupOptions{Orientation: form.GroupHorizontal}, func() {
							for _, choice := range themeChoices() {
								form.Radio(c, form.RadioOptions[string]{
									State: &st.theme,
									Value: choice.value,
									Label: choice.label,
								})
							}
						})
					},
				},
				{
					Icon:  lucide.Languages(),
					Title: locales.SettingsLanguage(),
					Trailing: func() {
						form.Select(c, form.SelectOptions{
							Items:    languageItems(),
							State:    &st.locale,
							OnChange: st.changeLocale,
						})
					},
					Control: &st.locale.Trigger,
				},
				{
					Icon:     lucide.Power(),
					Title:    locales.SettingsStartAtLogin(),
					Subtitle: startAtLoginHint(),
					Trailing: func() {
						form.Switch(c, form.SwitchOptions{
							On:       &st.startAtLogin,
							Disabled: mygo.IsDev(),
							OnChange: st.changeStartAtLogin,
						})
					},
				},
			},
		}).Grow(1)

		mygoui.SettingsGroup(c, mygoui.SettingsGroupOptions{
			Rows: []mygoui.SettingsRow{
				{
					Title:    mygo.App.Name(),
					Subtitle: mygo.App.Version(),
					Trailing: func() {
						mygoui.Button(c, mygoui.ButtonOptions{
							Label:   locales.SettingsCheckForUpdates(),
							Icon:    lucide.Download(),
							Variant: mygoui.ButtonOutline,
							OnClick: func() {
								updater.CheckForUpdates()
							},
						})
					},
				},
			},
		})
	})
}

// hotkeysSection 是“快捷键”标签页：外壳自己的全局快捷键。
func (s *Shell) hotkeysSection(c *ui.Context) {
	st := s.settings
	ui.Column(c).FillWidth().Gap(theme.Space(4)).Children(func() {
		mygoui.SettingsGroup(c, mygoui.SettingsGroupOptions{
			Title: locales.SettingsHotkeyGlobal(),
			Rows: []mygoui.SettingsRow{
				{
					Icon:  lucide.AppWindow(),
					Title: locales.SettingsHotkeyGlobalWindowToggle(),
					Trailing: func() {
						form.HotkeyInput(c, &st.globalHotkey, form.HotkeyInputOptions{
							AriaLabel:   locales.SettingsHotkeyGlobalWindowToggle(),
							Placeholder: locales.SettingsHotkeyUnset(),
							Clearable:   true,
							OnChange:    st.changeGlobalHotkey,
						})
					},
				},
			},
		})
	})
}

// changeTheme 应用并保存外观。
func (st *settings) changeTheme(mode string) {
	st.cfg.Theme = config.ThemeMode(mode)
	applyTheme(st.cfg.Theme)
	st.save()
}

// changeLocale 应用并保存语言；config.LocaleSystem 表示跟随系统。
func (st *settings) changeLocale(keys []string) {
	if len(keys) != 1 {
		return
	}
	st.cfg.Locale = keys[0]
	applyLocale(st.cfg.Locale)
	st.save()
}

// changeGlobalHotkey 重新注册全局快捷键并保存。
func (st *settings) changeGlobalHotkey() {
	st.cfg.Hotkey.Global.WindowToggle = hotkey.Format(st.globalHotkey)
	st.shell.rebindHotkeys()
	st.save()
}

// changeStartAtLogin 让系统记住是否随登录启动；失败时把开关拨回原状态。
func (st *settings) changeStartAtLogin(on bool) {
	if err := mygo.App.SetOpenAtLogin(on); err != nil {
		st.startAtLogin = !on
		st.shell.log.Warn("settings: 设置开机启动", "error", err)
		if w := st.shell.settingsWin; w != nil {
			w.Invalidate()
		}
	}
}

// startAtLoginHint 在开发模式下说明开机启动只在打包版本生效。
func startAtLoginHint() string {
	if mygo.IsDev() {
		return locales.SettingsStartAtLoginDev()
	}
	return ""
}

func (st *settings) save() {
	if err := st.cfg.Save(); err != nil {
		st.shell.log.Warn("settings: saving the config", "error", err)
	}
}

// applyTheme 让 mygo 按 mode 选择浅色、深色或跟随系统；mygoui 会跟随外观。
func applyTheme(mode config.ThemeMode) {
	source := mygo.ThemeSystem
	switch mode {
	case config.ThemeLight:
		source = mygo.ThemeLight
	case config.ThemeDark:
		source = mygo.ThemeDark
	}
	mygo.Theme.SetSource(source)
}

// applyLocale 切换界面语言；跟随系统在 mygo 里表示为空。
func applyLocale(tag string) {
	if tag == config.LocaleSystem {
		tag = ""
	}
	mygo.App.SetLocale(tag)
}

type choice struct {
	label string
	value string
}

// themeChoices 是外观的三个选择，值就是 config 里保存的值。
func themeChoices() []choice {
	return []choice{
		{locales.ThemeSystem(), string(config.ThemeSystem)},
		{locales.ThemeLight(), string(config.ThemeLight)},
		{locales.ThemeDark(), string(config.ThemeDark)},
	}
}

// languageItems 是语言的三个选择，键就是 config 里保存的值。
func languageItems() []list.ListBoxItem {
	return []list.ListBoxItem{
		{Key: config.LocaleSystem, Label: locales.LanguageSystem()},
		{Key: config.LocaleEnglish, Label: locales.LanguageEnglish()},
		{Key: config.LocaleChinese, Label: locales.LanguageChinese()},
	}
}
