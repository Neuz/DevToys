// Package config 保存用户偏好。
package config

// ThemeMode 是界面主题的选择。
type ThemeMode string

const (
	ThemeSystem ThemeMode = "system"
	ThemeLight  ThemeMode = "light"
	ThemeDark   ThemeMode = "dark"
)

// Locale 是界面语言标签。
const (
	LocaleSystem  = "system"
	LocaleEnglish = "en"
	LocaleChinese = "zh-Hans"
)

// HotkeyConfig 是按作用域分组的快捷键；空字符串表示未设置。
type HotkeyConfig struct {
	Global GlobalHotkey `json:"global"`
	App    AppHotkey    `json:"app"`
}

// GlobalHotkey 是系统级全局快捷键。
type GlobalHotkey struct {
	// WindowToggle 显示/隐藏主窗口；空表示不注册。
	WindowToggle string `json:"window_toggle"`
}

// AppHotkey 是仅在应用窗口内生效的快捷键。
type AppHotkey struct {
	// SidebarToggle 折叠/展开侧边栏；空表示不响应。
	SidebarToggle string `json:"sidebar_toggle"`
}

// Config 是 config.json 的内容。
type Config struct {
	LogLevel string       `json:"log_level"`
	Theme    ThemeMode    `json:"theme"`
	Locale   string       `json:"locale"`
	Hotkey   HotkeyConfig `json:"hotkey"`
}

// Default 返回首次运行使用的配置。
func Default() Config {
	return Config{
		LogLevel: "info",
		Theme:    ThemeSystem,
		Locale:   LocaleSystem,
	}
}

// Normalize 修正非法值：未知主题退回跟随系统，未知语言退回跟随系统。
func (c Config) Normalize() Config {
	switch c.Theme {
	case ThemeSystem, ThemeLight, ThemeDark:
	default:
		c.Theme = ThemeSystem
	}
	switch c.Locale {
	case LocaleSystem, LocaleEnglish, LocaleChinese:
	default:
		c.Locale = LocaleSystem
	}
	return c
}
