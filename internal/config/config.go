// Package config 保存用户偏好。
package config

import (
	"encoding/json"
	"log/slog"
)

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
	LogLevel LogLevel      `json:"log_level"`
	Theme    ThemeMode     `json:"theme"`
	Locale   string        `json:"locale"`
	Hotkey   HotkeyConfig  `json:"hotkey"`
	Network  NetworkConfig `json:"network"`
	Features Features      `json:"features,omitempty"`
}

// ProxyMode 是应用级网络代理的选择。
type ProxyMode string

const (
	ProxyDirect ProxyMode = "direct"
	ProxySystem ProxyMode = "system"
	ProxyManual ProxyMode = "manual"
)

// NetworkConfig 是应用自身的网络设置。
type NetworkConfig struct {
	Proxy ProxyConfig `json:"proxy"`
}

// ProxyConfig 是代理模式与手动代理参数。
type ProxyConfig struct {
	// Mode 取 direct、system 或 manual。
	Mode ProxyMode `json:"mode"`
	// URL 是手动模式的代理地址；切回其他模式时保留，便于再次切换。
	URL string `json:"url,omitempty"`
	// Exceptions 是手动模式下直连的主机、域名或网段。
	Exceptions []string `json:"exceptions,omitempty"`
}

// HotkeyConfig 是外壳自己使用的快捷键；空字符串表示未设置。
type HotkeyConfig struct {
	Global GlobalHotkey `json:"global"`
}

// GlobalHotkey 是系统级全局快捷键。
type GlobalHotkey struct {
	// WindowToggle 显示/隐藏主窗口；空表示不注册。
	WindowToggle string `json:"window_toggle"`
}

// Features 以模块 ID 为键保存各功能的开关、快捷键与私有配置。
// 没有出现的模块视为启用。
type Features map[string]*Feature

// Feature 是一个功能模块的配置。
type Feature struct {
	Enabled bool              `json:"enabled"`
	Hotkeys map[string]string `json:"hotkeys,omitempty"` // 热键 ID → accelerator
	Options json.RawMessage   `json:"options,omitempty"` // 该功能自己的配置
}

// UnmarshalJSON 让省略 enabled 的节点仍然启用，避免手写配置意外关掉功能。
func (f *Feature) UnmarshalJSON(data []byte) error {
	type plain Feature
	p := plain{Enabled: true}
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	*f = Feature(p)
	return nil
}

// FeatureNode 返回 id 的配置节点，缺失时创建并写入 map，供功能模块直接改写。
func (c *Config) FeatureNode(id string) *Feature {
	if c.Features == nil {
		c.Features = make(Features)
	}
	if f, ok := c.Features[id]; ok && f != nil {
		return f
	}
	f := &Feature{Enabled: true}
	c.Features[id] = f
	return f
}

// DecodeOptions 把节点里的 options 解成 T；缺失或解析失败时返回 def。
func DecodeOptions[T any](id string, node *Feature, def T) T {
	if node == nil || len(node.Options) == 0 {
		return def
	}
	var v T
	if err := json.Unmarshal(node.Options, &v); err != nil {
		slog.Warn("config: 功能配置解析失败", "feature", id, "error", err)
		return def
	}
	return v
}

// Default 返回首次运行使用的配置。
func Default() Config {
	return Config{
		LogLevel: LogLevelInfo,
		Theme:    ThemeSystem,
		Locale:   LocaleSystem,
		Network: NetworkConfig{
			Proxy: ProxyConfig{Mode: ProxySystem},
		},
	}
}

// Normalize 把非法值修正为默认值：未知主题、语言退回跟随系统，未知级别退回 info，
// 未知代理模式退回系统代理。
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
	if !c.Network.Proxy.Mode.valid() {
		c.Network.Proxy.Mode = ProxySystem
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

// valid 报告代理模式是否是已知取值。
func (m ProxyMode) valid() bool {
	switch m {
	case ProxyDirect, ProxySystem, ProxyManual:
		return true
	}
	return false
}
