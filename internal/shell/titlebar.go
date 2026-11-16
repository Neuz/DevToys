package shell

import (
	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Cyenoch/mygoui"
	"github.com/Cyenoch/mygoui/form"
	"github.com/Cyenoch/mygoui/overlay"
	"github.com/Cyenoch/mygoui/theme"
	"github.com/Neuz/DevToys/internal/config"
	"github.com/Neuz/DevToys/internal/prefs"
	"github.com/Neuz/DevToys/locales"
	"github.com/egoist/mygo/ui"
)

// titleBarHeight 是自绘标题栏的高度，与窗口创建时的原生带一致。
const titleBarHeight float32 = 44

// titleBarTrailing 在标题栏右侧放置主题、语言与设置。
func (s *Shell) titleBarTrailing(c *ui.Context) {
	s.themeButton(c)
	s.languageButton(c)
	if s.settings != nil {
		mygoui.Button(c, mygoui.ButtonOptions{
			Label:    locales.SettingsTitle(),
			Icon:     lucide.Settings(),
			IconOnly: true,
			Variant:  mygoui.ButtonGhost,
			Size:     theme.ButtonSmall,
			Tooltip:  locales.OpenSettings(),
			OnClick:  s.openSettings,
		})
	}
}

// themeButton 是主题 Popover 的锚点与面板。
func (s *Shell) themeButton(c *ui.Context) {
	anchor := mygoui.Button(c, mygoui.ButtonOptions{
		Label:    locales.Theme(),
		Icon:     lucide.SunMoon(),
		IconOnly: true,
		Variant:  mygoui.ButtonGhost,
		Size:     theme.ButtonSmall,
		Tooltip:  locales.Theme(),
		OnClick:  func() { s.themeOpen = !s.themeOpen },
	})
	overlay.Popover(c, anchor, &s.themeOpen, overlay.PopoverOptions{
		Title:     locales.Theme(),
		Placement: mygoui.Placement{Side: mygoui.SideBottom, Align: mygoui.AlignEnd},
		Width:     220,
		Body: func(pc *ui.Context) {
			form.RadioGroup(pc, form.RadioGroupOptions{Orientation: form.GroupVertical}, func() {
				for _, choice := range themeChoices() {
					form.Radio(pc, form.RadioOptions[config.ThemeMode]{
						State: &s.themeState,
						Value: choice.mode,
						Label: choice.label,
					})
				}
			})
		},
	})
}

// languageButton 是语言 Popover 的锚点与面板。
func (s *Shell) languageButton(c *ui.Context) {
	anchor := mygoui.Button(c, mygoui.ButtonOptions{
		Label:    locales.Language(),
		Icon:     lucide.Languages(),
		IconOnly: true,
		Variant:  mygoui.ButtonGhost,
		Size:     theme.ButtonSmall,
		Tooltip:  locales.Language(),
		OnClick:  func() { s.langOpen = !s.langOpen },
	})
	overlay.Popover(c, anchor, &s.langOpen, overlay.PopoverOptions{
		Title:     locales.Language(),
		Placement: mygoui.Placement{Side: mygoui.SideBottom, Align: mygoui.AlignEnd},
		Width:     220,
		Body: func(pc *ui.Context) {
			form.RadioGroup(pc, form.RadioGroupOptions{Orientation: form.GroupVertical}, func() {
				for _, choice := range languageChoices() {
					form.Radio(pc, form.RadioOptions[string]{
						State: &s.langState,
						Value: choice.tag,
						Label: choice.label,
					})
				}
			})
		},
	})
}

// changeTheme 应用标题栏选择的外观并交给 app 保存。
func (s *Shell) changeTheme(mode config.ThemeMode) {
	s.themeMode = mode
	prefs.ApplyTheme(mode)
	if s.opts.OnThemeChange != nil {
		s.opts.OnThemeChange(mode)
	}
}

// changeLocale 应用标题栏选择的语言；空 tag 跟随系统。
func (s *Shell) changeLocale(tag string) {
	s.localeTag = tag
	prefs.ApplyLocale(tag)
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
