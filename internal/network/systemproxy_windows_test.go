//go:build windows

package network

import (
	"reflect"
	"testing"
)

func TestDisplayProxyServer(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"", ""},
		{"127.0.0.1:7890", "http://127.0.0.1:7890"},
		{"http=127.0.0.1:8080;https=127.0.0.1:8443", "http://127.0.0.1:8080, https://127.0.0.1:8443"},
		{"socks=127.0.0.1:1080;", "socks://127.0.0.1:1080"},
	}
	for _, tt := range tests {
		if got := displayProxyServer(tt.raw); got != tt.want {
			t.Errorf("displayProxyServer(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestSplitBypass(t *testing.T) {
	got := splitBypass("localhost; 127.* ;<local>;;")
	want := []string{"localhost", "127.*", "<local>"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitBypass = %v, want %v", got, want)
	}
	if got := splitBypass(""); got != nil {
		t.Fatalf("空列表 = %v, want nil", got)
	}
}
