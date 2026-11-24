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
	"time"
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
			if got := decodeError([]byte(tt.body)); got != tt.want {
				t.Fatalf("decodeError(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func TestDecodeErrorTruncates(t *testing.T) {
	body := `{"message":"` + strings.Repeat("x", 300) + `"}`
	got := decodeError([]byte(body))
	if len([]rune(got)) != 241 || !strings.HasSuffix(got, "…") {
		t.Fatalf("decodeError length = %d, want 241 with ellipsis", len([]rune(got)))
	}
}

func TestHTTPError(t *testing.T) {
	if err := httpError(http.StatusUnauthorized, nil); err == nil || err.Error() != "认证失败，请检查密钥" {
		t.Fatalf("401 error = %v", err)
	}
	err := httpError(http.StatusBadRequest, []byte(`{"error":{"message":"bad"}}`))
	if err == nil || !strings.Contains(err.Error(), "HTTP 400") || !strings.Contains(err.Error(), "bad") {
		t.Fatalf("400 error = %v", err)
	}
}

func TestDoWithClientContextTimeout(t *testing.T) {
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1:1", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(request.Context(), 0)
	defer cancel()
	request = request.WithContext(ctx)
	_, err = doWithClient(ctx, httpClient(), request)
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
	p := &googleProvider{endpoint: "http://127.0.0.1:1", client: httpClient}
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

func TestMicrosoftBingTranslate(t *testing.T) {
	var form map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/translator":
			_, _ = w.Write([]byte(`<html><script>var params_AbusePreventionHelper = [123456,"tok"];</script>
<script>IG:"ABC123"</script><div data-iid="translator.5024"></div></html>`))
		case "/ttranslatev3":
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse form: %v", err)
			}
			form = map[string]string{
				"fromLang": r.PostFormValue("fromLang"),
				"to":       r.PostFormValue("to"),
				"text":     r.PostFormValue("text"),
				"token":    r.PostFormValue("token"),
				"key":      r.PostFormValue("key"),
				"IG":       r.URL.Query().Get("IG"),
				"IID":      r.URL.Query().Get("IID"),
			}
			_, _ = w.Write([]byte(`[{"detectedLanguage":{"language":"en"},"translations":[{"text":"你好"}]}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	p := &microsoftProvider{
		pageEndpoint:  server.URL + "/translator",
		azureEndpoint: server.URL + "/translate",
		client:        func() *http.Client { return server.Client() },
	}
	res, err := p.Translate(context.Background(), Request{Text: "hello", Source: "en", Target: "zh-Hans"})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if res.Text != "你好" || res.Detected != "en" {
		t.Fatalf("result = %+v", res)
	}
	want := map[string]string{
		"fromLang": "en", "to": "zh-Hans", "text": "hello", "token": "tok", "key": "123456",
		"IG": "ABC123", "IID": "translator.5024.1",
	}
	for key, value := range want {
		if form[key] != value {
			t.Fatalf("form[%s] = %q, want %q", key, form[key], value)
		}
	}
}

func TestMicrosoftBingMissingSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html>no session</html>`))
	}))
	defer server.Close()
	p := &microsoftProvider{
		pageEndpoint: server.URL,
		client:       func() *http.Client { return server.Client() },
	}
	_, err := p.Translate(context.Background(), Request{Text: "hello", Source: "en", Target: "zh-Hans"})
	if err == nil || !strings.Contains(err.Error(), "验证") {
		t.Fatalf("err = %v, want verification error", err)
	}
}

func TestMicrosoftAzureTranslate(t *testing.T) {
	var header http.Header
	var query map[string][]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Clone()
		query = r.URL.Query()
		var body []map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if len(body) != 1 || body[0]["Text"] != "hello" {
			t.Errorf("body = %+v", body)
		}
		_, _ = w.Write([]byte(`[{"detectedLanguage":{"language":"en"},"translations":[{"text":"你好"}]}]`))
	}))
	defer server.Close()

	p := &microsoftProvider{
		apiKey:        "secret",
		region:        "eastasia",
		azureEndpoint: server.URL,
		client:        func() *http.Client { return server.Client() },
	}
	res, err := p.Translate(context.Background(), Request{Text: "hello", Source: "en", Target: "zh-Hans"})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if res.Text != "你好" {
		t.Fatalf("Text = %q", res.Text)
	}
	if header.Get("Ocp-Apim-Subscription-Key") != "secret" ||
		header.Get("Ocp-Apim-Subscription-Region") != "eastasia" {
		t.Fatalf("headers = %v", header)
	}
	if query["api-version"][0] != "3.0" || query["to"][0] != "zh-Hans" || query["from"][0] != "en" {
		t.Fatalf("query = %v", query)
	}
}

func TestMicrosoftAzureOmitsSourceForAuto(t *testing.T) {
	var query map[string][]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		_, _ = w.Write([]byte(`[{"translations":[{"text":"x"}]}]`))
	}))
	defer server.Close()
	p := &microsoftProvider{apiKey: "k", azureEndpoint: server.URL, client: func() *http.Client { return server.Client() }}
	if _, err := p.Translate(context.Background(), Request{Text: "hi", Source: "auto", Target: "zh-Hans"}); err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if _, ok := query["from"]; ok {
		t.Fatalf("from should be omitted, query = %v", query)
	}
}

func TestMicrosoftTooLong(t *testing.T) {
	p := &microsoftProvider{client: httpClient}
	_, err := p.Translate(context.Background(), Request{Text: strings.Repeat("a", microsoftMaxRunes+1)})
	if err == nil || !strings.Contains(err.Error(), "1000") {
		t.Fatalf("err = %v, want length error", err)
	}
}

func TestMicrosoftHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	p := &microsoftProvider{apiKey: "bad", azureEndpoint: server.URL, client: func() *http.Client { return server.Client() }}
	start := time.Now()
	_, err := p.Translate(context.Background(), Request{Text: "hello"})
	if err == nil || !strings.Contains(err.Error(), "认证失败") {
		t.Fatalf("err = %v, want auth error", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("unexpected delay")
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
