//go:build !windows

package network

// ReadSystemProxy 只在 Windows 上读取系统代理设置；其他平台交给 netproxy。
func ReadSystemProxy() SystemProxy {
	return SystemProxy{}
}
