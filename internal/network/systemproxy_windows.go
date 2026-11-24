//go:build windows

package network

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

// internetSettingsKey 是 Windows「Internet 选项」保存代理设置的位置。
const internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

// ReadSystemProxy 读取当前用户的 Windows 代理设置，用于设置页展示。
func ReadSystemProxy() SystemProxy {
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		return SystemProxy{Supported: true}
	}
	defer k.Close() //nolint:errcheck // 只读句柄关闭失败无需处理

	out := SystemProxy{Supported: true}
	if pac, _, err := k.GetStringValue("AutoConfigURL"); err == nil {
		out.AutoConfig = strings.TrimSpace(pac)
	}
	if detect, _, err := k.GetIntegerValue("AutoDetect"); err == nil {
		out.AutoDetect = detect != 0
	}
	if enabled, _, err := k.GetIntegerValue("ProxyEnable"); err == nil && enabled != 0 {
		if server, _, err := k.GetStringValue("ProxyServer"); err == nil {
			out.Address = displayProxyServer(server)
		}
	}
	if bypass, _, err := k.GetStringValue("ProxyOverride"); err == nil {
		out.Exceptions = splitBypass(bypass)
	}
	return out
}

// displayProxyServer 把 Windows 的代理配置转成便于阅读的地址：
// 单独的 host:port 补成 http://，按协议分开的列表逐项补上 scheme。
func displayProxyServer(raw string) string {
	parts := strings.Split(raw, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		scheme, addr := "http", part
		if i := strings.IndexByte(part, '='); i > 0 {
			scheme = strings.ToLower(strings.TrimSpace(part[:i]))
			addr = strings.TrimSpace(part[i+1:])
		}
		if addr == "" {
			continue
		}
		if !strings.Contains(addr, "://") {
			addr = scheme + "://" + addr
		}
		out = append(out, strings.TrimSuffix(addr, "/"))
	}
	return strings.Join(out, ", ")
}

// splitBypass 把 Windows 用分号分隔的绕过列表拆成条目。
func splitBypass(raw string) []string {
	var out []string
	for _, item := range strings.Split(raw, ";") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
