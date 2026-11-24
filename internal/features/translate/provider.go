package translate

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Neuz/DevToys/internal/network"
)

// 翻译服务 ID，与配置里的 providers 键一致。
const (
	providerGoogle    = "google"
	providerMicrosoft = "microsoft"
	providerBilibili  = "bilibili"
)

// maxResponseBytes 限制单个响应体大小，避免异常服务耗尽内存。
const maxResponseBytes = 4 << 20

// Request 是已经决策过语言的翻译请求。
type Request struct {
	Text   string
	Source string
	Target string
}

// Result 是单个服务的翻译结果。
type Result struct {
	Provider string
	Text     string
	Detected string
	Latency  time.Duration
}

// Provider 是一个翻译渠道。
type Provider interface {
	ID() string
	Name() string
	Translate(ctx context.Context, req Request) (Result, error)
}

// maxRunes 返回文本的字符数；各服务的长度限制都按字符计算。
func maxRunes(text string) int {
	return len([]rune(text))
}

// httpClient 返回遵循应用代理设置的客户端。
func httpClient() *http.Client {
	return network.Client()
}

// doRequest 用应用客户端发送请求，并把网络、超时与 HTTP 错误转成可展示的错误。
func doRequest(ctx context.Context, req *http.Request) ([]byte, error) {
	return doWithClient(ctx, httpClient(), req)
}

// doWithClient 允许必应内置路径使用独立的 cookie 会话。
func doWithClient(ctx context.Context, client *http.Client, req *http.Request) ([]byte, error) {
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json, text/plain, */*")
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) DevToys/Translate")
	}
	resp, err := client.Do(req)
	if err != nil {
		if ctxErr := req.Context().Err(); ctxErr != nil {
			return nil, timeoutError(ctxErr)
		}
		var timeout interface{ Timeout() bool }
		if errors.As(err, &timeout) && timeout.Timeout() {
			return nil, errors.New("请求超时，请检查网络后重试")
		}
		return nil, errors.New("无法连接翻译服务，请检查网络")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		if ctxErr := req.Context().Err(); ctxErr != nil {
			return nil, timeoutError(ctxErr)
		}
		return nil, errors.New("读取翻译响应失败")
	}
	if len(body) > maxResponseBytes {
		return nil, errors.New("翻译服务响应过大")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpError(resp.StatusCode, body)
	}
	return body, nil
}

// timeoutError 区分调用方取消与请求超时。
func timeoutError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.New("请求超时，请检查网络后重试")
	}
	return err
}

// httpError 把状态码转成稳定、可展示且不泄露请求内容的错误。
func httpError(status int, body []byte) error {
	switch status {
	case http.StatusUnauthorized:
		return errors.New("认证失败，请检查密钥")
	case http.StatusForbidden:
		return errors.New("访问被拒绝，内置渠道可能需要网页验证")
	case http.StatusTooManyRequests:
		return errors.New("请求过于频繁或额度不足，请稍后重试")
	}
	if message := decodeError(body); message != "" {
		return fmt.Errorf("翻译服务返回 HTTP %d：%s", status, message)
	}
	return fmt.Errorf("翻译服务返回 HTTP %d", status)
}

// decodeError 只读取常见 JSON 错误字段，不展示完整响应。
func decodeError(body []byte) string {
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

// providerError 给错误补上服务上下文，便于日志定位。
func providerError(id string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", id, err)
}

// googleEndpoint 是无需密钥的 Google 网页翻译接口。
const googleEndpoint = "https://translate.googleapis.com/translate_a/single"

// googleMaxRunes 是 Google 网页接口的单次长度上限。
const googleMaxRunes = 5000

type googleProvider struct {
	endpoint string
	client   func() *http.Client
}

func newGoogleProvider() *googleProvider {
	return &googleProvider{endpoint: googleEndpoint, client: httpClient}
}

func (p *googleProvider) ID() string { return providerGoogle }

func (p *googleProvider) Name() string { return "Google" }

func (p *googleProvider) Translate(ctx context.Context, req Request) (Result, error) {
	if strings.TrimSpace(req.Text) == "" {
		return Result{}, errors.New("请输入要翻译的文本")
	}
	if maxRunes(req.Text) > googleMaxRunes {
		return Result{}, errors.New("Google 单次最多翻译 5000 个字符，请分段翻译")
	}
	endpoint, err := url.Parse(p.endpoint)
	if err != nil {
		return Result{}, providerError(p.ID(), err)
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
		return Result{}, providerError(p.ID(), err)
	}
	body, err := doWithClient(ctx, p.client(), request)
	if err != nil {
		return Result{}, providerError(p.ID(), err)
	}
	var response []json.RawMessage
	if err := json.Unmarshal(body, &response); err != nil || len(response) == 0 {
		return Result{}, providerError(p.ID(), errors.New("Google 响应格式异常"))
	}
	var sentences [][]json.RawMessage
	if err := json.Unmarshal(response[0], &sentences); err != nil {
		return Result{}, providerError(p.ID(), errors.New("Google 响应格式异常"))
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
		return Result{}, providerError(p.ID(), errors.New("Google 未返回译文"))
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

const (
	bingPageEndpoint  = "https://www.bing.com/translator"
	azureEndpoint     = "https://api.cognitive.microsofttranslator.com/translate"
	microsoftMaxRunes = 1000
	bingUserAgent     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
)

var (
	bingTokenPattern = regexp.MustCompile(`params_AbusePreventionHelper\s*=\s*(\[[^\r\n;]+?\])`)
	bingIGPattern    = regexp.MustCompile(`\bIG\s*:\s*"([A-Za-z0-9]+)"`)
	bingIIDPattern   = regexp.MustCompile(`data-iid=["']([^"']+)["']`)
)

type microsoftProvider struct {
	apiKey        string
	region        string
	pageEndpoint  string
	azureEndpoint string
	client        func() *http.Client
}

func newMicrosoftProvider(opts providerOptions) *microsoftProvider {
	return &microsoftProvider{
		apiKey:        strings.TrimSpace(opts.MicrosoftAPIKey),
		region:        strings.TrimSpace(opts.MicrosoftRegion),
		pageEndpoint:  bingPageEndpoint,
		azureEndpoint: azureEndpoint,
		client:        httpClient,
	}
}

func (p *microsoftProvider) ID() string { return providerMicrosoft }

func (p *microsoftProvider) Name() string { return "Microsoft" }

func (p *microsoftProvider) Translate(ctx context.Context, req Request) (Result, error) {
	if strings.TrimSpace(req.Text) == "" {
		return Result{}, errors.New("请输入要翻译的文本")
	}
	if maxRunes(req.Text) > microsoftMaxRunes {
		return Result{}, errors.New("微软翻译单次最多翻译 1000 个字符，请分段翻译")
	}
	var text, detected string
	var err error
	if p.apiKey != "" {
		text, detected, err = p.azure(ctx, req)
	} else {
		text, detected, err = p.bing(ctx, req)
	}
	if err != nil {
		return Result{}, providerError(p.ID(), err)
	}
	if strings.TrimSpace(text) == "" {
		return Result{}, providerError(p.ID(), errors.New("微软翻译未返回译文"))
	}
	return Result{Provider: p.ID(), Text: text, Detected: microsoftDetected(detected)}, nil
}

// azure 调用微软官方 Translator v3 接口。
func (p *microsoftProvider) azure(ctx context.Context, req Request) (string, string, error) {
	endpoint, err := url.Parse(p.azureEndpoint)
	if err != nil {
		return "", "", err
	}
	query := endpoint.Query()
	query.Set("api-version", "3.0")
	query.Set("to", microsoftLanguage(req.Target))
	if req.Source != "" && req.Source != langAuto {
		query.Set("from", microsoftLanguage(req.Source))
	} else {
		query.Del("from")
	}
	endpoint.RawQuery = query.Encode()
	payload, err := json.Marshal([]map[string]string{{"Text": req.Text}})
	if err != nil {
		return "", "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return "", "", err
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("Ocp-Apim-Subscription-Key", p.apiKey)
	if p.region != "" {
		request.Header.Set("Ocp-Apim-Subscription-Region", p.region)
	}
	body, err := doWithClient(ctx, p.client(), request)
	if err != nil {
		return "", "", err
	}
	return parseMicrosoft(body)
}

// bing 读取公开翻译页的会话参数后调用网页接口，不处理验证码。
func (p *microsoftProvider) bing(ctx context.Context, req Request) (string, string, error) {
	pageRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, p.pageEndpoint, nil)
	if err != nil {
		return "", "", err
	}
	pageRequest.Header.Set("User-Agent", bingUserAgent)
	jar, err := cookiejar.New(nil)
	if err != nil {
		return "", "", err
	}
	base := p.client()
	client := *base
	client.Jar = jar
	finalPageURL := pageRequest.URL
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("必应网页重定向过多")
		}
		finalPageURL = request.URL
		return nil
	}
	page, err := doWithClient(ctx, &client, pageRequest)
	if err != nil {
		return "", "", err
	}
	match := bingTokenPattern.FindSubmatch(page)
	if len(match) < 2 {
		return "", "", errors.New("必应网页可能需要验证或接口已变更，请尝试填写 Azure 密钥")
	}
	var credentials []json.RawMessage
	if json.Unmarshal(match[1], &credentials) != nil || len(credentials) < 2 {
		return "", "", errors.New("必应网页会话格式已变更")
	}
	key := strings.Trim(string(credentials[0]), "\"")
	var token string
	if json.Unmarshal(credentials[1], &token) != nil || key == "" || token == "" {
		return "", "", errors.New("必应网页未提供有效会话参数")
	}
	ig := ""
	if match := bingIGPattern.FindSubmatch(page); len(match) > 1 {
		ig = string(match[1])
	} else {
		data := make([]byte, 16)
		if _, err := rand.Read(data); err != nil {
			return "", "", errors.New("无法生成必应会话标识")
		}
		ig = strings.ToUpper(hex.EncodeToString(data))
	}
	iid := "translator.5024"
	if match := bingIIDPattern.FindSubmatch(page); len(match) > 1 {
		iid = string(match[1])
	}
	endpoint := &url.URL{
		Scheme: finalPageURL.Scheme,
		Host:   finalPageURL.Host,
		Path:   "/ttranslatev3",
		RawQuery: url.Values{
			"isVertical": {"1"},
			"IG":         {ig},
			"IID":        {iid + ".1"},
		}.Encode(),
	}
	form := url.Values{
		"fromLang": {microsoftLanguage(req.Source)},
		"to":       {microsoftLanguage(req.Target)},
		"text":     {req.Text},
		"token":    {token},
		"key":      {key},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", bingUserAgent)
	request.Header.Set("Referer", finalPageURL.String())
	request.Header.Set("Origin", endpoint.Scheme+"://"+endpoint.Host)
	body, err := doWithClient(ctx, &client, request)
	if err != nil {
		return "", "", err
	}
	return parseMicrosoft(body)
}

// microsoftResponse 是必应网页接口与 Azure Translator 共用的响应片段。
type microsoftResponse struct {
	Detected struct {
		Language string `json:"language"`
	} `json:"detectedLanguage"`
	Translations []struct {
		Text string `json:"text"`
	} `json:"translations"`
}

// parseMicrosoft 提取译文，并处理服务以 HTTP 200 返回的业务错误。
func parseMicrosoft(body []byte) (string, string, error) {
	if message := decodeError(body); message != "" {
		return "", "", fmt.Errorf("微软翻译返回错误：%s", message)
	}
	var response []microsoftResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", "", errors.New("微软翻译响应格式异常；内置渠道可能暂时不可用")
	}
	if len(response) == 0 || len(response[0].Translations) == 0 {
		return "", "", errors.New("微软翻译未返回译文；内置渠道可能暂时不可用")
	}
	return response[0].Translations[0].Text, microsoftDetected(response[0].Detected.Language), nil
}

// microsoftLanguage 把内部语言标签映射为微软的代码。
func microsoftLanguage(tag string) string {
	switch tag {
	case "", langAuto:
		return "auto-detect"
	case "zh-CN", "zh-Hans":
		return "zh-Hans"
	case "zh-TW", "zh-Hant":
		return "zh-Hant"
	default:
		return tag
	}
}

// microsoftDetected 把微软返回的检测语言统一成内部标签。
func microsoftDetected(tag string) string {
	switch tag {
	case "zh-CN":
		return "zh-Hans"
	case "zh-TW":
		return "zh-Hant"
	default:
		return tag
	}
}

const (
	bilibiliEndpoint = "https://index-translate.bilibili.com/v1/chat/completions"
	bilibiliModel    = "Index-Translate-35B-A3B"
	bilibiliMaxRunes = 4000
	bilibiliMaxToken = 2048
)

type bilibiliProvider struct {
	endpoint string
	client   func() *http.Client
}

func newBilibiliProvider() *bilibiliProvider {
	return &bilibiliProvider{endpoint: bilibiliEndpoint, client: httpClient}
}

func (p *bilibiliProvider) ID() string { return providerBilibili }

func (p *bilibiliProvider) Name() string { return "Bilibili" }

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
		return Result{}, providerError(p.ID(), err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(data))
	if err != nil {
		return Result{}, providerError(p.ID(), err)
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	body, err := doWithClient(ctx, p.client(), request)
	if err != nil {
		return Result{}, providerError(p.ID(), err)
	}
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return Result{}, providerError(p.ID(), errors.New("哔哩哔哩响应格式异常"))
	}
	if len(response.Choices) == 0 {
		return Result{}, providerError(p.ID(), errors.New("哔哩哔哩未返回译文"))
	}
	text := response.Choices[0].Message.Content
	for _, marker := range []string{"</think>", "<｜end▁of▁thinking｜>"} {
		if index := strings.LastIndex(text, marker); index >= 0 {
			text = text[index+len(marker):]
		}
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return Result{}, providerError(p.ID(), errors.New("哔哩哔哩未返回译文"))
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
