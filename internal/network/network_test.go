package network

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Neuz/DevToys/internal/config"
)

func resetState(t *testing.T) {
	t.Helper()
	current.Store(nil)
	t.Cleanup(func() { current.Store(nil) })
}

func TestNormalize(t *testing.T) {
	got, err := Normalize(config.ProxyConfig{
		Mode:       config.ProxyManual,
		URL:        "  socks5h://User:Pass@127.0.0.1:1080  ",
		Exceptions: []string{" Example.COM ", "example.com", "", "*.corp.test"},
	})
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if got.URL != "socks5://User:Pass@127.0.0.1:1080" {
		t.Fatalf("URL = %q", got.URL)
	}
	want := []string{"example.com", "*.corp.test"}
	if len(got.Exceptions) != len(want) {
		t.Fatalf("Exceptions = %v", got.Exceptions)
	}
	for i := range want {
		if got.Exceptions[i] != want[i] {
			t.Fatalf("Exceptions = %v, want %v", got.Exceptions, want)
		}
	}

	if _, err := Normalize(config.ProxyConfig{Mode: config.ProxyManual, URL: "127.0.0.1"}); err == nil {
		t.Fatal("缺少端口应当报错")
	}
	if _, err := Normalize(config.ProxyConfig{Mode: config.ProxyManual, URL: "ftp://127.0.0.1:21"}); err == nil {
		t.Fatal("不支持的协议应当报错")
	}
	if _, err := Normalize(config.ProxyConfig{Mode: config.ProxyManual, URL: "http://127.0.0.1:8080/path"}); err == nil {
		t.Fatal("带 path 的地址应当报错")
	}
	if _, err := Normalize(config.ProxyConfig{Mode: config.ProxyManual, URL: "http://127.0.0.1:8080", Exceptions: []string{"10.0.0.0/99"}}); err == nil {
		t.Fatal("非法网段应当报错")
	}
}

func TestNormalizeKeepsManualFields(t *testing.T) {
	got, err := Normalize(config.ProxyConfig{Mode: config.ProxySystem, URL: "http://127.0.0.1:8080", Exceptions: []string{"Localhost"}})
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if got.URL != "http://127.0.0.1:8080" || len(got.Exceptions) != 1 || got.Exceptions[0] != "localhost" {
		t.Fatalf("Normalize = %+v", got)
	}
}

func TestProxyDirect(t *testing.T) {
	resetState(t)
	if err := Apply(config.ProxyConfig{Mode: config.ProxyDirect}); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	got, err := Proxy(req)
	if err != nil || got != nil {
		t.Fatalf("Proxy = %v, %v", got, err)
	}
}

func TestProxyManual(t *testing.T) {
	resetState(t)
	if err := Apply(config.ProxyConfig{
		Mode:       config.ProxyManual,
		URL:        "http://user:pass@127.0.0.1:7890",
		Exceptions: []string{"example.com", "10.0.0.0/8"},
	}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		url  string
		want bool
	}{
		{"https://other.test", true},
		{"https://example.com", false},
		{"https://api.example.com", false},
		{"https://bad-example.com", true},
		{"https://10.2.3.4", false},
		{"https://11.0.0.1", true},
		{"http://localhost:8080", false},
		{"http://127.0.0.1:8080", false},
	}
	for _, tc := range cases {
		req, _ := http.NewRequest(http.MethodGet, tc.url, nil)
		got, err := Proxy(req)
		if err != nil {
			t.Fatalf("%s: %v", tc.url, err)
		}
		if (got != nil) != tc.want {
			t.Fatalf("%s: Proxy = %v, want proxy %v", tc.url, got, tc.want)
		}
	}
}

func TestExceptionMatch(t *testing.T) {
	cases := []struct {
		rule string
		host string
		want bool
	}{
		{"example.com", "example.com", true},
		{"example.com", "api.example.com", true},
		{"example.com", "bad-example.com", false},
		{"*.corp.test", "corp.test", true},
		{"*.corp.test", "a.corp.test", true},
		{".corp.test", "a.corp.test", true},
		{"10.0.0.0/8", "10.2.3.4", true},
		{"10.0.0.0/8", "11.0.0.1", false},
		{"192.168.1.5", "192.168.1.5", true},
		{"192.168.1.5", "192.168.1.6", false},
		{"*", "anything.test", true},
	}
	for _, tc := range cases {
		e, err := compileException(tc.rule)
		if err != nil {
			t.Fatalf("%s: %v", tc.rule, err)
		}
		if got := (exceptionSet{e}).match(tc.host); got != tc.want {
			t.Fatalf("%q match %q = %v, want %v", tc.rule, tc.host, got, tc.want)
		}
	}
}

func TestCheckReachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	t.Cleanup(func() { _ = ln.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := CheckReachable(ctx, "http://"+addr); err != nil {
		t.Fatalf("CheckReachable(open): %v", err)
	}

	_ = ln.Close()
	if _, err := CheckReachable(ctx, "http://"+addr); err == nil {
		t.Fatal("关闭的端口应当不可达")
	}
	if _, err := CheckReachable(ctx, "127.0.0.1"); err == nil {
		t.Fatal("缺少端口应当报错")
	}
}

func TestRedact(t *testing.T) {
	got := Redact("http://user:secret@127.0.0.1:8080")
	if strings.Contains(got, "secret") {
		t.Fatalf("Redact = %q", got)
	}
	if want := "http://user@127.0.0.1:8080"; got != want {
		t.Fatalf("Redact = %q, want %q", got, want)
	}
	if got := Redact("not a url"); got != "<invalid>" {
		t.Fatalf("Redact(invalid) = %q", got)
	}
}
