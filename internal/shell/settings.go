package shell

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Cyenoch/mygoui"
	"github.com/Cyenoch/mygoui/form"
	"github.com/Cyenoch/mygoui/layout"
	"github.com/Cyenoch/mygoui/list"
	"github.com/Cyenoch/mygoui/navigation"
	"github.com/Cyenoch/mygoui/theme"
	"github.com/Neuz/DevToys/internal/config"
	"github.com/Neuz/DevToys/internal/hotkey"
	"github.com/Neuz/DevToys/internal/network"
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

	// 网络代理：proxyMode 是单选框的选中值，proxyURL 与 proxyExceptions 是尚未保存的编辑内容。
	proxyMode       string
	proxyRadio      form.RadioGroupState[string]
	proxyURL        string
	proxyExceptions string

	// proxyTesting 表示“测试连接”正在进行；proxyError 与 proxyStatus 显示在操作行。
	proxyTesting bool
	proxyError   string
	proxyStatus  string
	proxyCancel  context.CancelFunc
	// proxySeq 标识最近一次测试，丢弃已取消或被新测试取代的结果。
	proxySeq uint64
	// systemProxy 是打开设置页时读到的系统代理，仅在系统模式下展示。
	systemProxy network.SystemProxy
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
			if s.settings != nil {
				s.settings.cancelProxyTest()
			}
			s.settingsWin = nil
			s.settings = nil
		})
	})
}

// newSettings 用配置里的当前值作为初始选择。
func newSettings(s *Shell) *settings {
	st := &settings{
		shell:           s,
		cfg:             s.opts.Config,
		mode:            string(s.opts.Config.Theme),
		locale:          form.SelectState{List: list.ListBoxState{SelectedKey: s.opts.Config.Locale}},
		globalHotkey:    hotkey.Parse(s.opts.Config.Hotkey.Global.WindowToggle),
		startAtLogin:    mygo.App.OpenAtLogin(),
		proxyMode:       string(s.opts.Config.Network.Proxy.Mode),
		proxyURL:        s.opts.Config.Network.Proxy.URL,
		proxyExceptions: strings.Join(s.opts.Config.Network.Proxy.Exceptions, ", "),
		systemProxy:     network.ReadSystemProxy(),
	}
	st.theme = form.RadioGroupState[string]{Selected: &st.mode, OnChange: st.changeTheme}
	st.proxyRadio = form.RadioGroupState[string]{Selected: &st.proxyMode, OnChange: st.changeProxyMode}
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
				{Label: locales.SettingsNetwork(), Content: func() { s.networkSection(c) }},
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

// networkSection 是“网络”标签页：代理模式与手动代理参数。
func (s *Shell) networkSection(c *ui.Context) {
	st := s.settings
	rows := []mygoui.SettingsRow{
		{
			Icon:     lucide.Network(),
			Title:    locales.SettingsProxyMode(),
			Subtitle: st.modeSubtitle(),
			Trailing: func() {
				form.RadioGroup(c, form.RadioGroupOptions{Orientation: form.GroupHorizontal}, func() {
					for _, choice := range proxyChoices() {
						form.Radio(c, form.RadioOptions[string]{
							State: &st.proxyRadio,
							Value: choice.value,
							Label: choice.label,
						})
					}
				})
			},
		},
	}
	switch st.proxyMode {
	case string(config.ProxySystem):
		rows = append(rows,
			mygoui.SettingsRow{
				Icon:  lucide.Globe(),
				Title: locales.SettingsProxyUrl(),
				Trailing: func() {
					systemProxyText(c, st.systemProxyAddress(), "")
				},
			},
			mygoui.SettingsRow{
				Icon:  lucide.Shield(),
				Title: locales.SettingsProxyExceptions(),
				Trailing: func() {
					text := st.systemProxyExceptions()
					systemProxyText(c, text, st.systemProxyExceptionsTooltip())
				},
			},
		)
	case string(config.ProxyManual):
		rows = append(rows,
			mygoui.SettingsRow{
				Icon:  lucide.Globe(),
				Title: locales.SettingsProxyUrl(),
				Trailing: func() {
					form.Input(c, &st.proxyURL, form.InputOptions{
						AriaLabel:   locales.SettingsProxyUrl(),
						Placeholder: locales.SettingsProxyUrlPlaceholder(),
						OnChange:    st.changeProxyURL,
						OnSubmit:    st.saveProxy,
					}).Width(theme.Scale(c, 220))
				},
			},
			mygoui.SettingsRow{
				Icon:  lucide.Shield(),
				Title: locales.SettingsProxyExceptions(),
				Trailing: func() {
					form.Input(c, &st.proxyExceptions, form.InputOptions{
						AriaLabel:   locales.SettingsProxyExceptions(),
						Placeholder: locales.SettingsProxyExceptionsPlaceholder(),
						OnChange:    st.changeProxyExceptions,
						OnSubmit:    st.saveProxy,
					}).Width(theme.Scale(c, 220))
				},
			},
			mygoui.SettingsRow{
				Icon:     lucide.Cable(),
				Title:    locales.SettingsProxyActions(),
				Subtitle: st.proxyHint(),
				Trailing: func() {
					mygoui.Button(c, mygoui.ButtonOptions{
						Label:    locales.SettingsProxySave(),
						Icon:     lucide.Save(),
						Variant:  mygoui.ButtonPrimary,
						Loading:  st.proxyTesting,
						Disabled: st.proxyTesting,
						OnClick:  st.saveProxy,
					})
				},
			},
		)
	}
	ui.Column(c).FillWidth().Gap(theme.Space(4)).Children(func() {
		mygoui.SettingsGroup(c, mygoui.SettingsGroupOptions{
			Rows: rows,
		}).Grow(1)
	})
}

// changeProxyMode 切换代理模式；手动模式要等【测试并保存】成功后才生效。
func (st *settings) changeProxyMode(mode string) {
	st.cancelProxyTest()
	st.proxyError, st.proxyStatus = "", ""
	if mode == string(config.ProxyManual) {
		st.resetProxyDrafts()
		st.invalidate()
		return
	}
	next := st.cfg.Network.Proxy
	next.Mode = config.ProxyMode(mode)
	if err := network.Apply(next); err != nil {
		st.proxyError = fmt.Sprintf(locales.SettingsProxyInvalid(), err.Error())
		st.invalidate()
		return
	}
	st.cfg.Network.Proxy = next
	if next.Mode == config.ProxySystem {
		st.systemProxy = network.ReadSystemProxy()
	}
	st.resetProxyDrafts()
	st.shell.log.Info("network: proxy updated", "mode", next.Mode)
	st.save()
	st.invalidate()
}

// changeProxyURL 丢弃尚未完成的测试与上次的状态提示。
func (st *settings) changeProxyURL() {
	st.cancelProxyTest()
	st.proxyError, st.proxyStatus = "", ""
	st.invalidate()
}

// changeProxyExceptions 清掉上次的测试或保存结果。
func (st *settings) changeProxyExceptions() {
	st.proxyError, st.proxyStatus = "", ""
	st.invalidate()
}

// saveProxy 先检测代理地址，可达才保存并生效；不可达时回退为系统代理。
func (st *settings) saveProxy() {
	if st.proxyTesting {
		return
	}
	st.cancelProxyTest()
	next := st.cfg.Network.Proxy
	next.Mode = config.ProxyManual
	next.URL = strings.TrimSpace(st.proxyURL)
	next.Exceptions = strings.Split(st.proxyExceptions, ",")
	normalized, err := network.Normalize(next)
	if err != nil {
		st.proxyStatus = ""
		st.proxyError = fmt.Sprintf(locales.SettingsProxyInvalid(), err.Error())
		st.invalidate()
		return
	}

	st.proxySeq++
	seq := st.proxySeq
	ctx, cancel := context.WithCancel(context.Background())
	st.proxyCancel = cancel
	st.proxyTesting = true
	st.proxyError, st.proxyStatus = "", locales.SettingsProxyTesting()
	st.invalidate()

	go func() {
		elapsed, err := network.CheckReachable(ctx, normalized.URL)
		mygo.RunOnMain(func() {
			// 窗口已关闭、操作被取消或被新的操作取代时丢弃结果。
			if st.shell.settings != st || st.proxySeq != seq {
				return
			}
			st.proxyTesting = false
			st.proxyCancel = nil
			if err != nil {
				st.fallbackToSystem(normalized, err)
				return
			}
			st.applyManual(normalized, elapsed)
		})
	}()
}

// applyManual 在地址可达后应用并保存手动代理。
func (st *settings) applyManual(cfg config.ProxyConfig, elapsed time.Duration) {
	if err := network.Apply(cfg); err != nil {
		st.proxyStatus = ""
		st.proxyError = fmt.Sprintf(locales.SettingsProxyInvalid(), err.Error())
		st.invalidate()
		return
	}
	st.cfg.Network.Proxy = cfg
	st.proxyMode = string(config.ProxyManual)
	st.resetProxyDrafts()
	st.proxyError, st.proxyStatus = "", fmt.Sprintf(locales.SettingsProxySavedReachable(), elapsed.Round(time.Millisecond))
	st.shell.log.Info("network: proxy updated", "mode", cfg.Mode, "proxy", network.Redact(cfg.URL))
	st.save()
	st.invalidate()
}

// fallbackToSystem 在地址不可达时切回系统代理，并保留已填写的地址供下次修改。
func (st *settings) fallbackToSystem(cfg config.ProxyConfig, cause error) {
	cfg.Mode = config.ProxySystem
	if err := network.Apply(cfg); err != nil {
		st.proxyStatus = ""
		st.proxyError = fmt.Sprintf(locales.SettingsProxyInvalid(), err.Error())
		st.invalidate()
		return
	}
	st.cfg.Network.Proxy = cfg
	st.proxyMode = string(config.ProxySystem)
	st.systemProxy = network.ReadSystemProxy()
	st.resetProxyDrafts()
	st.proxyError, st.proxyStatus = "", locales.SettingsProxyFallback()
	st.shell.log.Warn("network: proxy unreachable, falling back to system", "proxy", network.Redact(cfg.URL), "error", cause)
	st.save()
	st.invalidate()
}

// cancelProxyTest 停止正在进行的地址检测，并让它之后的结果失效。
func (st *settings) cancelProxyTest() {
	if st.proxyCancel != nil {
		st.proxyCancel()
		st.proxyCancel = nil
	}
	st.proxySeq++
	st.proxyTesting = false
}

// resetProxyDrafts 让输入框重新显示已保存的地址与例外。
func (st *settings) resetProxyDrafts() {
	st.proxyURL = st.cfg.Network.Proxy.URL
	st.proxyExceptions = strings.Join(st.cfg.Network.Proxy.Exceptions, ", ")
}

// systemProxyExceptionsTooltip 给可能被截断的例外列表提供完整内容。
func (st *settings) systemProxyExceptionsTooltip() string {
	if len(st.systemProxy.Exceptions) == 0 {
		return ""
	}
	return strings.Join(st.systemProxy.Exceptions, ", ")
}

// proxyHint 是操作行要显示的反馈：错误优先于成功提示。
func (st *settings) proxyHint() string {
	if st.proxyError != "" {
		return st.proxyError
	}
	return st.proxyStatus
}

func (st *settings) invalidate() {
	if w := st.shell.settingsWin; w != nil {
		w.Invalidate()
	}
}

// modeSubtitle 优先说明回退原因，否则显示当前真正生效的代理。
func (st *settings) modeSubtitle() string {
	if st.proxyMode == string(config.ProxySystem) && st.proxyStatus != "" {
		return st.proxyStatus
	}
	return proxyEffectiveText()
}

// proxyEffectiveText 说明当前真正生效的代理，便于区分配置与运行状态。
func proxyEffectiveText() string {
	mode, raw := network.Current()
	switch mode {
	case config.ProxyDirect:
		return locales.SettingsProxyCurrentDirect()
	case config.ProxyManual:
		return fmt.Sprintf(locales.SettingsProxyCurrentManual(), network.Redact(raw))
	default:
		return locales.SettingsProxyCurrentSystem()
	}
}

// proxyChoices 是代理模式的三个选择，值就是 config 里保存的值。
func proxyChoices() []choice {
	return []choice{
		{locales.SettingsProxyDirect(), string(config.ProxyDirect)},
		{locales.SettingsProxySystem(), string(config.ProxySystem)},
		{locales.SettingsProxyManual(), string(config.ProxyManual)},
	}
}

// systemProxyText 在行的右侧以只读文本显示系统代理信息；tooltip 为空时不提示。
func systemProxyText(c *ui.Context, text, tooltip string) {
	e := mygoui.Typography(c, text, mygoui.TypographyOptions{
		Type:       theme.TypeBodySM,
		Muted:      true,
		Truncate:   true,
		Align:      ui.End,
		Selectable: true,
	}).Width(theme.Scale(c, 220))
	if tooltip != "" {
		e.Tooltip(tooltip)
	}
}

// systemProxyAddress 把系统代理快照转成一行可读的说明。
func (st *settings) systemProxyAddress() string {
	info := st.systemProxy
	switch {
	case !info.Supported:
		return locales.SettingsProxySystemUnavailable()
	case info.AutoConfig != "":
		return fmt.Sprintf(locales.SettingsProxySystemAuto(), info.AutoConfig)
	case info.AutoDetect:
		return locales.SettingsProxySystemAutoDetect()
	case info.Address != "":
		return info.Address
	default:
		return locales.SettingsProxySystemNone()
	}
}

// systemProxyExceptions 显示系统绕过列表；为空时给出明确提示。
func (st *settings) systemProxyExceptions() string {
	if len(st.systemProxy.Exceptions) == 0 {
		return locales.SettingsProxySystemNoExceptions()
	}
	return strings.Join(st.systemProxy.Exceptions, ", ")
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
