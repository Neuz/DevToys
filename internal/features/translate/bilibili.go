package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Neuz/DevToys/internal/network"
)

const (
	bilibiliEndpoint = "https://index-translate.bilibili.com/v1/chat/completions"
	bilibiliModel    = "Index-Translate-35B-A3B"
	bilibiliMaxRunes = 4000
	bilibiliMaxToken = 2048
)

// bilibiliResponseLimit 限制单个响应体大小，避免异常响应耗尽内存。
const bilibiliResponseLimit = 4 << 20

type bilibiliProvider struct {
	endpoint string
	client   func() *http.Client
}

func newBilibiliProvider() *bilibiliProvider {
	return &bilibiliProvider{endpoint: bilibiliEndpoint, client: network.Client}
}

func (p *bilibiliProvider) ID() string { return providerBilibili }

func (p *bilibiliProvider) Name() string { return "Bilibili" }

// send 用本服务自带的客户端发送请求，读取受限响应体并映射传输层错误。
func (p *bilibiliProvider) send(ctx context.Context, req *http.Request) ([]byte, error) {
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
	body, err := io.ReadAll(io.LimitReader(resp.Body, bilibiliResponseLimit+1))
	if err != nil {
		if ctxErr := req.Context().Err(); ctxErr != nil {
			if errors.Is(ctxErr, context.DeadlineExceeded) {
				return nil, errors.New("请求超时，请检查网络后重试")
			}
			return nil, ctxErr
		}
		return nil, errors.New("读取翻译响应失败")
	}
	if len(body) > bilibiliResponseLimit {
		return nil, errors.New("翻译服务响应过大")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, p.httpError(resp.StatusCode, body)
	}
	return body, nil
}

// httpError 把状态码转成稳定、可展示且不泄露请求内容的错误。
func (p *bilibiliProvider) httpError(status int, body []byte) error {
	switch status {
	case http.StatusUnauthorized:
		return errors.New("认证失败")
	case http.StatusForbidden:
		return errors.New("访问被拒绝")
	case http.StatusTooManyRequests:
		return errors.New("请求过于频繁，请稍后重试")
	}
	if message := p.decodeError(body); message != "" {
		return fmt.Errorf("哔哩哔哩返回 HTTP %d：%s", status, message)
	}
	return fmt.Errorf("哔哩哔哩返回 HTTP %d", status)
}

// decodeError 只读取常见 JSON 错误字段，不展示完整响应。
func (p *bilibiliProvider) decodeError(body []byte) string {
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
func (p *bilibiliProvider) fail(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", p.ID(), err)
}

func (p *bilibiliProvider) Translate(ctx context.Context, req Request) (Result, error) {
	if strings.TrimSpace(req.Text) == "" {
		return Result{}, errors.New("请输入要翻译的文本")
	}
	if maxRunes(req.Text) > bilibiliMaxRunes {
		return Result{}, errors.New("哔哩哔哩单次最多翻译 4000 个字符，请分段翻译")
	}
	payload := map[string]any{
		"model": bilibiliModel,
		"messages": []map[string]string{{
			"role":    "user",
			"content": bilibiliPrompt(req),
		}},
		"temperature":          0,
		"max_tokens":           bilibiliMaxToken,
		"stream":               false,
		"chat_template_kwargs": map[string]bool{"enable_thinking": false},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return Result{}, p.fail(err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(data))
	if err != nil {
		return Result{}, p.fail(err)
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	body, err := p.send(ctx, request)
	if err != nil {
		return Result{}, p.fail(err)
	}
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return Result{}, p.fail(errors.New("哔哩哔哩响应格式异常"))
	}
	if len(response.Choices) == 0 {
		return Result{}, p.fail(errors.New("哔哩哔哩未返回译文"))
	}
	text := response.Choices[0].Message.Content
	for _, marker := range []string{"</think>", "<｜end▁of▁thinking｜>"} {
		if index := strings.LastIndex(text, marker); index >= 0 {
			text = text[index+len(marker):]
		}
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return Result{}, p.fail(errors.New("哔哩哔哩未返回译文"))
	}
	return Result{Provider: p.ID(), Text: text, Detected: req.Source}, nil
}

// bilibiliPrompt 按源语言是否已知生成单轮提示词。
func bilibiliPrompt(req Request) string {
	target := bilibiliLanguageName(req.Target)
	subject := "以下文本"
	if req.Source != "" && req.Source != langAuto {
		subject = "以下" + bilibiliLanguageName(req.Source) + "文本"
	}
	return "请将" + subject + "翻译为" + target + "，直接输出翻译结果，不要进行任何解释。\n\n" + req.Text
}

// bilibiliLanguageName 返回提示词里使用的语言名。
func bilibiliLanguageName(tag string) string {
	switch tag {
	case langChinese:
		return "简体中文"
	case langEnglish:
		return "English"
	case "ja":
		return "日本語"
	case "ko":
		return "한국어"
	case "ru":
		return "русский"
	case "fr":
		return "français"
	case "de":
		return "Deutsch"
	case "es":
		return "español"
	default:
		return tag
	}
}
