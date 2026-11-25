package translate

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Neuz/DevToys/internal/network"
)

func TestDecodeError(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"object message", `{"error":{"message":"bad  request"}}`, "bad request"},
		{"plain error", `{"error":"denied"}`, "denied"},
		{"top message", `{"message":"limited"}`, "limited"},
		{"invalid", `<html>`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &googleProvider{}
			if got := p.decodeError([]byte(tt.body)); got != tt.want {
				t.Fatalf("decodeError(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func TestDecodeErrorTruncates(t *testing.T) {
	body := `{"message":"` + strings.Repeat("x", 300) + `"}`
	p := &googleProvider{}
	got := p.decodeError([]byte(body))
	if len([]rune(got)) != 241 || !strings.HasSuffix(got, "…") {
		t.Fatalf("decodeError length = %d, want 241 with ellipsis", len([]rune(got)))
	}
}

func TestHTTPError(t *testing.T) {
	p := &googleProvider{}
	if err := p.httpError(http.StatusUnauthorized, nil); err == nil || err.Error() != "认证失败" {
		t.Fatalf("401 error = %v", err)
	}
	err := p.httpError(http.StatusBadRequest, []byte(`{"error":{"message":"bad"}}`))
	if err == nil || !strings.Contains(err.Error(), "HTTP 400") || !strings.Contains(err.Error(), "bad") {
		t.Fatalf("400 error = %v", err)
	}
}

func TestGoogleSendContextTimeout(t *testing.T) {
	p := &googleProvider{client: network.Client}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1:1", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	_, err = p.send(ctx, request.WithContext(ctx))
	if err == nil || !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "超时") {
		t.Fatalf("err = %v, want timeout", err)
	}
}

func TestMaxRunes(t *testing.T) {
	if got := maxRunes("你好a"); got != 3 {
		t.Fatalf("maxRunes = %d, want 3", got)
	}
}

func TestGoogleTranslate(t *testing.T) {
	var gotQuery map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		gotQuery = map[string]string{
			"client": query.Get("client"),
			"sl":     query.Get("sl"),
			"tl":     query.Get("tl"),
			"dt":     query.Get("dt"),
			"q":      query.Get("q"),
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[[["Hello &amp; <b>","你好",null,null],[" world","世界",null,null]],null,"zh-CN"]`))
	}))
	defer server.Close()

	p := &googleProvider{endpoint: server.URL, client: server.Client}
	res, err := p.Translate(context.Background(), Request{Text: "你好 世界", Source: "zh-Hans", Target: "en"})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if res.Text != "Hello & <b> world" {
		t.Fatalf("Text = %q", res.Text)
	}
	if res.Detected != "zh-CN" {
		t.Fatalf("Detected = %q", res.Detected)
	}
	want := map[string]string{"client": "gtx", "sl": "zh-CN", "tl": "en", "dt": "t", "q": "你好 世界"}
	for key, value := range want {
		if gotQuery[key] != value {
			t.Fatalf("query[%s] = %q, want %q", key, gotQuery[key], value)
		}
	}
}

func TestGoogleTooLong(t *testing.T) {
	p := &googleProvider{endpoint: "http://127.0.0.1:1", client: network.Client}
	_, err := p.Translate(context.Background(), Request{Text: strings.Repeat("a", googleMaxRunes+1)})
	if err == nil || !strings.Contains(err.Error(), "5000") {
		t.Fatalf("err = %v, want length error", err)
	}
}

func TestGoogleEmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[[],null,"en"]`))
	}))
	defer server.Close()
	p := &googleProvider{endpoint: server.URL, client: server.Client}
	if _, err := p.Translate(context.Background(), Request{Text: "hello"}); err == nil || !strings.Contains(err.Error(), "未返回译文") {
		t.Fatalf("err = %v, want no result", err)
	}
}

func TestBilibiliTranslate(t *testing.T) {
	var mu sync.Mutex
	var prompt, model string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		mu.Lock()
		model = body.Model
		if len(body.Messages) > 0 {
			prompt = body.Messages[0].Content
		}
		mu.Unlock()
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"<think>reasoning</think>\nHello, world"}}]}`))
	}))
	defer server.Close()

	p := &bilibiliProvider{endpoint: server.URL, client: server.Client}
	res, err := p.Translate(context.Background(), Request{Text: "你好，世界", Source: "zh-Hans", Target: "en"})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if res.Text != "Hello, world" {
		t.Fatalf("Text = %q", res.Text)
	}
	mu.Lock()
	defer mu.Unlock()
	if model != bilibiliModel {
		t.Fatalf("model = %q, want %q", model, bilibiliModel)
	}
	if !strings.Contains(prompt, "请将以下简体中文文本翻译为English") || !strings.Contains(prompt, "你好，世界") {
		t.Fatalf("prompt = %q", prompt)
	}
}

func TestBilibiliHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer server.Close()
	p := &bilibiliProvider{endpoint: server.URL, client: server.Client}
	_, err := p.Translate(context.Background(), Request{Text: "hello"})
	if err == nil || !strings.Contains(err.Error(), "bad request") {
		t.Fatalf("err = %v, want bad request", err)
	}
}

func TestBilibiliEmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"   "}}]}`))
	}))
	defer server.Close()
	p := &bilibiliProvider{endpoint: server.URL, client: server.Client}
	if _, err := p.Translate(context.Background(), Request{Text: "hello"}); err == nil || !strings.Contains(err.Error(), "未返回译文") {
		t.Fatalf("err = %v, want no result", err)
	}
}
