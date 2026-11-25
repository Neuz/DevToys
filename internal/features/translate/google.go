package translate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Neuz/DevToys/internal/network"
)

// googleEndpoint 是无需密钥的 Google 网页翻译接口。
const googleEndpoint = "https://translate.googleapis.com/translate_a/single"

// googleMaxRunes 是 Google 网页接口的单次长度上限。
const googleMaxRunes = 5000

// googleResponseLimit 限制单个响应体大小，避免异常响应耗尽内存。
const googleResponseLimit = 4 << 20

type googleProvider struct {
	endpoint string
	client   func() *http.Client
}

func newGoogleProvider() *googleProvider {
	return &googleProvider{endpoint: googleEndpoint, client: network.Client}
}

func (p *googleProvider) ID() string { return providerGoogle }

func (p *googleProvider) Name() string { return "Google" }

// send 用本服务自带的客户端发送请求，读取受限响应体并映射传输层错误。
func (p *googleProvider) send(ctx context.Context, req *http.Request) ([]byte, error) {
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json, text/plain, */*")
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) DevToys/Translate")
	}
	resp, err := p.client().Do(req)
	if err != nil {
		if ctxErr := req.Context().Err(); ctxErr != nil {
			if errors.Is(ctxErr, context.DeadlineExceeded) {
				return nil, errors.New("请求超时，请检查网络后重试")
			}
			return nil, ctxErr
		}
		var timeout interface{ Timeout() bool }
		if errors.As(err, &timeout) && timeout.Timeout() {
			return nil, errors.New("请求超时，请检查网络后重试")
		}
		return nil, errors.New("无法连接翻译服务，请检查网络")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, googleResponseLimit+1))
	if err != nil {
		if ctxErr := req.Context().Err(); ctxErr != nil {
			if errors.Is(ctxErr, context.DeadlineExceeded) {
				return nil, errors.New("请求超时，请检查网络后重试")
			}
			return nil, ctxErr
		}
		return nil, errors.New("读取翻译响应失败")
	}
	if len(body) > googleResponseLimit {
		return nil, errors.New("翻译服务响应过大")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, p.httpError(resp.StatusCode, body)
	}
	return body, nil
}

// httpError 把状态码转成稳定、可展示且不泄露请求内容的错误。
func (p *googleProvider) httpError(status int, body []byte) error {
	switch status {
	case http.StatusUnauthorized:
		return errors.New("认证失败")
	case http.StatusForbidden:
		return errors.New("访问被拒绝")
	case http.StatusTooManyRequests:
		return errors.New("请求过于频繁，请稍后重试")
	}
	if message := p.decodeError(body); message != "" {
		return fmt.Errorf("Google 返回 HTTP %d：%s", status, message)
	}
	return fmt.Errorf("Google 返回 HTTP %d", status)
}

// decodeError 只读取常见 JSON 错误字段，不展示完整响应。
func (p *googleProvider) decodeError(body []byte) string {
	var value struct {
		Error   json.RawMessage `json:"error"`
		Message string          `json:"message"`
	}
	if json.Unmarshal(body, &value) != nil {
		return ""
	}
	message := value.Message
	if len(value.Error) > 0 {
		var text string
		if json.Unmarshal(value.Error, &text) == nil {
			message = text
		} else {
			var detail struct {
				Message string `json:"message"`
			}
			if json.Unmarshal(value.Error, &detail) == nil {
				message = detail.Message
			}
		}
	}
	message = strings.Join(strings.Fields(message), " ")
	runes := []rune(message)
	if len(runes) > 240 {
		return string(runes[:240]) + "…"
	}
	return message
}

// fail 给错误补上服务上下文，便于日志定位。
func (p *googleProvider) fail(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", p.ID(), err)
}

func (p *googleProvider) Translate(ctx context.Context, req Request) (Result, error) {
	if strings.TrimSpace(req.Text) == "" {
		return Result{}, errors.New("请输入要翻译的文本")
	}
	if maxRunes(req.Text) > googleMaxRunes {
		return Result{}, errors.New("Google 单次最多翻译 5000 个字符，请分段翻译")
	}
	endpoint, err := url.Parse(p.endpoint)
	if err != nil {
		return Result{}, p.fail(err)
	}
	query := endpoint.Query()
	query.Set("client", "gtx")
	query.Set("sl", googleLanguage(req.Source))
	query.Set("tl", googleLanguage(req.Target))
	query.Set("dt", "t")
	query.Set("q", req.Text)
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return Result{}, p.fail(err)
	}
	body, err := p.send(ctx, request)
	if err != nil {
		return Result{}, p.fail(err)
	}
	var response []json.RawMessage
	if err := json.Unmarshal(body, &response); err != nil || len(response) == 0 {
		return Result{}, p.fail(errors.New("Google 响应格式异常"))
	}
	var sentences [][]json.RawMessage
	if err := json.Unmarshal(response[0], &sentences); err != nil {
		return Result{}, p.fail(errors.New("Google 响应格式异常"))
	}
	var translated strings.Builder
	for _, sentence := range sentences {
		if len(sentence) == 0 {
			continue
		}
		var text string
		if json.Unmarshal(sentence[0], &text) == nil {
			translated.WriteString(text)
		}
	}
	if translated.Len() == 0 {
		return Result{}, p.fail(errors.New("Google 未返回译文"))
	}
	var detected string
	if len(response) > 2 {
		_ = json.Unmarshal(response[2], &detected)
	}
	return Result{
		Provider: p.ID(),
		Text:     html.UnescapeString(translated.String()),
		Detected: detected,
	}, nil
}

// googleLanguage 把内部语言标签映射为 Google 使用的中文代码。
func googleLanguage(tag string) string {
	switch tag {
	case "zh-Hans":
		return "zh-CN"
	case "zh-Hant":
		return "zh-TW"
	default:
		return tag
	}
}
