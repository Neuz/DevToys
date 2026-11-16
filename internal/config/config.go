// Package config 保存用户偏好。
package config

// ThemeMode 是界面主题的选择。
type ThemeMode string

const (
	ThemeSystem ThemeMode = "system"
	ThemeLight  ThemeMode = "light"
	ThemeDark   ThemeMode = "dark"
)

// Config 是 config.json 的内容。
type Config struct {
	Version      int       `json:"version"`
	Theme        ThemeMode `json:"theme"`
	SidebarWidth int       `json:"sidebarWidth"`
	LastModule   string    `json:"lastModule"`
}

// Default 返回首次运行使用的配置。
func Default() Config {
	return Config{
		Version:      1,
		Theme:        ThemeSystem,
		SidebarWidth: 220,
	}
}

// Normalize 修正非法值：未知主题退回跟随系统，侧栏宽度夹在 [180, 360]。
func (c Config) Normalize() Config {
	switch c.Theme {
	case ThemeSystem, ThemeLight, ThemeDark:
	default:
		c.Theme = ThemeSystem
	}
	if c.Version <= 0 {
		c.Version = 1
	}
	if c.SidebarWidth == 0 {
		c.SidebarWidth = 220
	}
	if c.SidebarWidth < 180 {
		c.SidebarWidth = 180
	}
	if c.SidebarWidth > 360 {
		c.SidebarWidth = 360
	}
	return c
}
