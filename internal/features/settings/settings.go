// Package settings 是主窗口里的设置对话框。
package settings

import (
	"log/slog"

	"github.com/Cyenoch/mygoui"
	"github.com/Cyenoch/mygoui/form"
	"github.com/Cyenoch/mygoui/overlay"
	"github.com/Cyenoch/mygoui/theme"
	"github.com/Neuz/DevToys/internal/config"
	"github.com/Neuz/DevToys/internal/prefs"
	"github.com/Neuz/DevToys/locales"
	"github.com/egoist/mygo/ui"
)

// Options 是设置面板的依赖。
type Options struct {
	Config config.Config
	Save   func(config.Config) error
	// Theme 读取外壳当前的主题选择；标题栏也能改主题，对话框据此保持一致。
	Theme func() config.ThemeMode
	// Locale 读取当前选择的语言标签，空表示跟随系统；同样与标题栏保持一致。
	Locale func() string
	Log    *slog.Logger
}

// Settings 实现 contract.Settings。
type Settings struct {
	o    Options
	open bool

	cfg        config.Config
	lang       string
	themeState form.RadioGroupState[config.ThemeMode]
	langState  form.RadioGroupState[string]
}

// New 返回设置面板。
func New(o Options) *Settings {
	s := &Settings{o: o, cfg: o.Config}
	s.themeState = form.RadioGroupState[config.ThemeMode]{Selected: &s.cfg.Theme, OnChange: s.changeTheme}
	s.langState = form.RadioGroupState[string]{Selected: &s.lang, OnChange: s.changeLocale}
	return s
}

// Open 在下次绘制时打开面板。
func (s *Settings) Open() { s.open = true }

// View 绘制设置对话框；open 指向外壳的显示状态。
func (s *Settings) View(c *ui.Context, open *bool) {
	if open == nil {
		return
	}
	if s.open {
		*open = true
		s.open = false
	}
	// 标题栏也能改主题与语言，绘制前同步一次，避免对话框显示或保存旧值。
	if s.o.Theme != nil {
		s.cfg.Theme = s.o.Theme()
	}
	if s.o.Locale != nil {
		s.lang = s.o.Locale()
	}

	overlay.Dialog(c, open, overlay.DialogOptions{
		Title: locales.SettingsTitle(),
		Width: 520,
		Body: func() {
			ui.Column(c).Gap(theme.Space(5)).Children(func() {
				s.themeSection(c)
				s.languageSection(c)
			})
		},
		Footer: func() {
			ui.Row(c).Justify(ui.End).Children(func() {
				mygoui.Button(c, mygoui.ButtonOptions{
					Label:   locales.Close(),
					Variant: mygoui.ButtonSecondary,
					OnClick: func() { *open = false },
				})
			})
		},
	})
}

func (s *Settings) themeSection(c *ui.Context) {
	form.RadioGroup(c, form.RadioGroupOptions{Label: locales.SettingsTheme()}, func() {
		for _, choice := range themeChoices() {
			form.Radio(c, form.RadioOptions[config.ThemeMode]{
				State: &s.themeState,
				Value: choice.mode,
				Label: choice.label,
			})
		}
	})
}

func (s *Settings) languageSection(c *ui.Context) {
	form.RadioGroup(c, form.RadioGroupOptions{Label: locales.SettingsLanguage()}, func() {
		for _, choice := range languageChoices() {
			form.Radio(c, form.RadioOptions[string]{
				State: &s.langState,
				Value: choice.tag,
				Label: choice.label,
			})
		}
	})
}

// changeTheme 保存外观选择并让 mygoui 跟随新的外观。
func (s *Settings) changeTheme(mode config.ThemeMode) {
	s.cfg.Theme = mode
	prefs.ApplyTheme(mode)
	s.save()
}

// changeLocale 应用语言选择；空 tag 跟随系统。
func (s *Settings) changeLocale(tag string) {
	s.lang = tag
	prefs.ApplyLocale(tag)
	s.save()
}

func (s *Settings) save() {
	if s.o.Save == nil {
		return
	}
	if err := s.o.Save(s.cfg); err != nil {
		s.log().Warn("settings: saving the config", "error", err)
	}
}

func (s *Settings) log() *slog.Logger {
	if s.o.Log != nil {
		return s.o.Log
	}
	return slog.Default()
}

type themeChoice struct {
	label string
	mode  config.ThemeMode
}

func themeChoices() []themeChoice {
	return []themeChoice{
		{locales.ThemeSystem(), config.ThemeSystem},
		{locales.ThemeLight(), config.ThemeLight},
		{locales.ThemeDark(), config.ThemeDark},
	}
}

type languageChoice struct {
	label string
	tag   string
}

func languageChoices() []languageChoice {
	return []languageChoice{
		{locales.LanguageSystem(), ""},
		{locales.LanguageEnglish(), "en"},
		{locales.LanguageChinese(), "zh-Hans"},
	}
}
