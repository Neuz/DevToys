package network

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strings"
)

// parseProxyURL 补全 scheme 并校验手动代理地址。
func parseProxyURL(raw string) (string, *url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil, errors.New("代理地址不能为空")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", nil, fmt.Errorf("代理地址无效: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		u.Scheme = strings.ToLower(u.Scheme)
	case "socks5", "socks5h":
		u.Scheme = "socks5"
	default:
		return "", nil, fmt.Errorf("不支持的代理协议 %q", u.Scheme)
	}
	if u.Hostname() == "" {
		return "", nil, errors.New("代理地址缺少主机名")
	}
	if u.Port() == "" {
		return "", nil, errors.New("代理地址需要包含端口")
	}
	if u.Opaque != "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", nil, errors.New("代理地址只应包含协议、主机和端口")
	}
	return u.String(), u, nil
}

// exception 是手动模式的一条“直连”规则。
type exception struct {
	any    bool
	domain string
	ip     netip.Addr
	prefix netip.Prefix
}

// exceptionSet 是若干例外规则；命中任意一条即直连。
type exceptionSet []exception

func (s exceptionSet) match(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, e := range s {
		if e.match(host) {
			return true
		}
	}
	return false
}

func (e exception) match(host string) bool {
	switch {
	case e.any:
		return true
	case e.prefix.IsValid():
		ip, err := parseIP(host)
		return err == nil && e.prefix.Contains(ip)
	case e.ip.IsValid():
		ip, err := parseIP(host)
		return err == nil && ip == e.ip
	default:
		return host == e.domain || strings.HasSuffix(host, "."+e.domain)
	}
}

// normalizeExceptions 去掉空白项并忽略大小写去重，保持输入顺序。
func normalizeExceptions(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func validateExceptions(items []string) error {
	for _, item := range items {
		if _, err := compileException(item); err != nil {
			return err
		}
	}
	return nil
}

func compileExceptions(items []string) (exceptionSet, error) {
	set := make(exceptionSet, 0, len(items))
	for _, item := range items {
		e, err := compileException(item)
		if err != nil {
			return nil, err
		}
		set = append(set, e)
	}
	return set, nil
}

func compileException(item string) (exception, error) {
	item = strings.ToLower(strings.TrimSpace(item))
	if item == "*" {
		return exception{any: true}, nil
	}
	if strings.Contains(item, "/") {
		prefix, err := netip.ParsePrefix(item)
		if err != nil {
			return exception{}, fmt.Errorf("代理例外 %q 不是有效的网段", item)
		}
		return exception{prefix: prefix}, nil
	}
	host := strings.TrimPrefix(item, "*.")
	host = strings.TrimPrefix(host, ".")
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return exception{}, fmt.Errorf("代理例外 %q 缺少主机名", item)
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return exception{ip: ip.Unmap()}, nil
	}
	if strings.ContainsAny(host, " \t\r\n/:*?#@") {
		return exception{}, fmt.Errorf("代理例外 %q 不是有效的主机名", item)
	}
	return exception{domain: host}, nil
}

// parseIP 解析主机名中的 IP，并去掉 IPv6 zone、归一化 IPv4 映射地址。
func parseIP(host string) (netip.Addr, error) {
	if i := strings.LastIndexByte(host, '%'); i >= 0 {
		host = host[:i]
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, err
	}
	return addr.Unmap(), nil
}
