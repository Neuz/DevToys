// Package config 保存用户偏好。
package config

// ThemeMode 是界面主题的选择。
type ThemeMode string

const (
	ThemeSystem ThemeMode = "system"
	ThemeLight  ThemeMode = "light"
	ThemeDark   ThemeMode = "dark"
)

// LogLevel 是日志级别，取值与 slog 接受的文本一致。
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// 界面语言：LocaleSystem 表示跟随系统，其余是 BCP 47 语言标签。
const (
	LocaleSystem  = "system"
	LocaleEnglish = "en"
	LocaleChinese = "zh-Hans"
)

// Config 是 config.json 的内容。
type Config struct {
	LogLevel LogLevel     `json:"log_level"`
	Theme    ThemeMode    `json:"theme"`
	Locale   string       `json:"locale"`
	Hotkey   HotkeyConfig `json:"hotkey"`
}

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

// Default 返回首次运行使用的配置。
func Default() Config {
	return Config{
		LogLevel: LogLevelInfo,
		Theme:    ThemeSystem,
		Locale:   LocaleSystem,
	}
}

// Normalize 把非法值修正为默认值：未知主题、语言退回跟随系统，未知级别退回 info。
func (c Config) Normalize() Config {
	if !c.Theme.valid() {
		c.Theme = ThemeSystem
	}
	if !validLocale(c.Locale) {
		c.Locale = LocaleSystem
	}
	if !c.LogLevel.valid() {
		c.LogLevel = LogLevelInfo
	}
	return c
}

// valid 报告主题是否是已知取值。
func (t ThemeMode) valid() bool {
	switch t {
	case ThemeSystem, ThemeLight, ThemeDark:
		return true
	}
	return false
}

// valid 报告级别是否是已知取值。
func (l LogLevel) valid() bool {
	switch l {
	case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError:
		return true
	}
	return false
}

// validLocale 报告语言是否是设置页提供的取值。
func validLocale(tag string) bool {
	switch tag {
	case LocaleSystem, LocaleEnglish, LocaleChinese:
		return true
	}
	return false
}
