// Package prefs 把用户选择的外观与语言应用到 mygo 运行时。
package prefs

import (
	"github.com/Neuz/DevToys/internal/config"
	"github.com/egoist/mygo"
)

// ApplyTheme 让 mygo 按 mode 选择浅色、深色或跟随系统；mygoui 会跟随外观。
func ApplyTheme(mode config.ThemeMode) {
	mygo.Theme.SetSource(themeSource(mode))
}

// ApplyLocale 切换界面语言；tag 为空时跟随系统。
func ApplyLocale(tag string) {
	mygo.App.SetLocale(tag)
}

func themeSource(mode config.ThemeMode) mygo.ThemeSource {
	switch mode {
	case config.ThemeLight:
		return mygo.ThemeLight
	case config.ThemeDark:
		return mygo.ThemeDark
	default:
		return mygo.ThemeSystem
	}
}
