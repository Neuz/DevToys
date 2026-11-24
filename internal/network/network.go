// Package network 把配置里的代理模式应用到进程内的 HTTP 请求。
package network

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Neuz/DevToys/internal/config"
	"github.com/egoist/mygo/netproxy"
)

// dialTimeout 是“测试连接”检查代理地址可达性的超时。
const dialTimeout = 3 * time.Second

// compiled 是一个已经校验过的代理配置；存入 atomic 后不再修改。
type compiled struct {
	mode       config.ProxyMode
	manual     *url.URL
	exceptions exceptionSet
}

var current atomic.Pointer[compiled]

// Normalize 校验并规范化代理配置：补全 scheme、去重例外列表，并校验手动地址。
func Normalize(cfg config.ProxyConfig) (config.ProxyConfig, error) {
	n := config.ProxyConfig{
		Mode:       cfg.Mode,
		URL:        strings.TrimSpace(cfg.URL),
		Exceptions: normalizeExceptions(cfg.Exceptions),
	}
	switch n.Mode {
	case config.ProxyDirect, config.ProxySystem:
		return n, nil
	case config.ProxyManual:
		canonical, _, err := parseProxyURL(n.URL)
		if err != nil {
			return n, err
		}
		n.URL = canonical
		if err := validateExceptions(n.Exceptions); err != nil {
			return n, err
		}
		return n, nil
	default:
		n.Mode = config.ProxySystem
		return n, nil
	}
}

// Apply 校验并切换当前代理；失败时保留上一次有效配置。
func Apply(cfg config.ProxyConfig) error {
	st, err := compile(cfg)
	if err != nil {
		return err
	}
	current.Store(st)
	return nil
}

// Init 安装全局代理决策函数。必须在任何网络请求之前调用。
func Init(cfg config.ProxyConfig) error {
	if err := Apply(cfg); err != nil {
		return err
	}
	netproxy.Transport().Proxy = Proxy
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		t.Proxy = Proxy
	}
	return nil
}

// Client 返回遵循当前代理设置的 HTTP 客户端。
func Client() *http.Client {
	return &http.Client{Transport: netproxy.Transport(), Timeout: time.Minute}
}

// Current 返回当前生效的模式与手动代理地址。
func Current() (config.ProxyMode, string) {
	st := current.Load()
	if st == nil {
		return config.ProxySystem, ""
	}
	if st.manual == nil {
		return st.mode, ""
	}
	return st.mode, st.manual.String()
}

// Proxy 决定一个请求使用哪个代理；它实现了 http.Transport.Proxy。
func Proxy(req *http.Request) (*url.URL, error) {
	st := current.Load()
	if st == nil {
		return netproxy.Proxy(req)
	}
	if req == nil || req.URL == nil {
		return nil, nil
	}
	host := req.URL.Hostname()
	if isLoopback(host) {
		return nil, nil
	}
	switch st.mode {
	case config.ProxyDirect:
		return nil, nil
	case config.ProxySystem:
		return netproxy.Proxy(req)
	case config.ProxyManual:
		if st.exceptions.match(host) {
			return nil, nil
		}
		u := *st.manual
		return &u, nil
	}
	return nil, nil
}

// CheckReachable 检测代理地址的 host:port 是否可以建立 TCP 连接。
// 它只验证地址可达，不校验协议、认证或代理出口。
func CheckReachable(ctx context.Context, rawURL string) (time.Duration, error) {
	_, u, err := parseProxyURL(rawURL)
	if err != nil {
		return 0, err
	}
	addr := net.JoinHostPort(u.Hostname(), u.Port())
	dialer := net.Dialer{Timeout: dialTimeout}
	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return time.Since(start), err
	}
	_ = conn.Close()
	return time.Since(start), nil
}

// Redact 隐藏代理 URL 里的密码，便于写日志。
func Redact(rawURL string) string {
	_, u, err := parseProxyURL(rawURL)
	if err != nil {
		return "<invalid>"
	}
	if u.User == nil {
		return u.String()
	}
	u.User = url.User(u.User.Username())
	return u.String()
}

func compile(cfg config.ProxyConfig) (*compiled, error) {
	n, err := Normalize(cfg)
	if err != nil {
		return nil, err
	}
	st := &compiled{mode: n.Mode}
	if n.Mode != config.ProxyManual {
		return st, nil
	}
	_, u, err := parseProxyURL(n.URL)
	if err != nil {
		return nil, err
	}
	exceptions, err := compileExceptions(n.Exceptions)
	if err != nil {
		return nil, err
	}
	st.manual = u
	st.exceptions = exceptions
	return st, nil
}

func isLoopback(host string) bool {
	if host == "" {
		return false
	}
	host = strings.ToLower(host)
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip, err := parseIP(host)
	return err == nil && ip.IsLoopback()
}
