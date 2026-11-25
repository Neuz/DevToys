package translate

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/Neuz/DevToys/internal/config"
	"github.com/Neuz/DevToys/internal/features/translate/locales"
)

const (
	totalTimeout    = 15 * time.Second
	providerTimeout = 10 * time.Second
)

// options 是 features.translate.options 的内容。
type options struct {
	Target             string          `json:"target"`
	PanelFollowsCursor bool            `json:"panel_follows_cursor"`
	Providers          providerOptions `json:"providers"`
}

// providerOptions 是各服务的开关。
type providerOptions struct {
	Google   bool `json:"google"`
	Bilibili bool `json:"bilibili"`
}

// defaultOptions 返回首次运行的选择：只启用哔哩哔哩。
func defaultOptions() options {
	return options{
		Target:             langAuto,
		PanelFollowsCursor: true,
		Providers: providerOptions{
			Bilibili: true,
		},
	}
}

// decodeOptions 从功能节点读出配置；缺失字段保留默认值。
func decodeOptions(node *config.Feature) options {
	opts := defaultOptions()
	if node != nil && len(node.Options) > 0 {
		if err := json.Unmarshal(node.Options, &opts); err != nil {
			return defaultOptions()
		}
	}
	return normalizeOptions(opts)
}

// normalizeOptions 修正非法目标语言。
func normalizeOptions(opts options) options {
	opts.Target = strings.TrimSpace(opts.Target)
	if !knownTarget(opts.Target) {
		opts.Target = langAuto
	}
	return opts
}

// Emit 在单个服务结束时回调；gen 用于丢弃过期结果。
type Emit func(gen uint64, p Provider, res Result, err error)

// Service 按固定顺序并行调度已启用的服务。
type Service struct {
	mu        sync.Mutex
	providers []Provider
	gen       uint64
	cancel    context.CancelFunc
	base      context.Context
}

// NewService 按 Google → 哔哩哔哩的顺序装配已启用的服务。
func NewService(base context.Context, opts options) *Service {
	if base == nil {
		base = context.Background()
	}
	opts = normalizeOptions(opts)
	providers := make([]Provider, 0, 2)
	if opts.Providers.Google {
		providers = append(providers, newGoogleProvider())
	}
	if opts.Providers.Bilibili {
		providers = append(providers, newBilibiliProvider())
	}
	return &Service{providers: providers, base: base}
}

// IDs 返回已启用服务的 ID，顺序与卡片顺序一致。
func (s *Service) IDs() []string {
	ids := make([]string, 0, len(s.providers))
	for _, p := range s.providers {
		ids = append(ids, p.ID())
	}
	return ids
}

// Translate 启动一轮并行翻译，返回本轮 generation。
// source 为 auto 时由本地检测决定源语言；结果完成后逐个回调 emit。
func (s *Service) Translate(text, source, target string, emit Emit) uint64 {
	s.mu.Lock()
	s.gen++
	gen := s.gen
	if s.cancel != nil {
		s.cancel()
	}
	ctx, cancel := context.WithTimeout(s.base, totalTimeout)
	s.cancel = cancel
	providers := append([]Provider(nil), s.providers...)
	s.mu.Unlock()

	text = strings.TrimSpace(text)
	if text == "" || len(providers) == 0 {
		return gen
	}
	sourceLang, targetLang := resolveLanguagesFrom(text, source, target)
	req := Request{Text: text, Source: sourceLang, Target: targetLang}
	for _, p := range providers {
		go func(p Provider) {
			pctx, pcancel := context.WithTimeout(ctx, providerTimeout)
			defer pcancel()
			start := time.Now()
			res, err := p.Translate(pctx, req)
			if err == nil {
				res.Latency = time.Since(start)
			}
			s.mu.Lock()
			current := s.gen == gen && ctx.Err() == nil
			s.mu.Unlock()
			if !current || emit == nil {
				return
			}
			emit(gen, p, res, err)
		}(p)
	}
	return gen
}

// Cancel 让当前一轮及后续到达的结果全部失效。
func (s *Service) Cancel() {
	s.mu.Lock()
	s.gen++
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// 语言标签：auto 只用于配置，检测结果始终是具体标签。
const (
	langAuto    = "auto"
	langChinese = "zh-Hans"
	langEnglish = "en"
)

// targetLanguages 是设置页提供的目标语言顺序。
var targetLanguages = []string{langAuto, langChinese, langEnglish, "ja", "ko", "ru", "fr", "de", "es"}

// knownTarget 报告标签是否属于设置页的可选项。
func knownTarget(tag string) bool {
	for _, candidate := range targetLanguages {
		if tag == candidate {
			return true
		}
	}
	return false
}

// detectLanguage 用本地启发式判断源语言，不发起网络请求。
func detectLanguage(text string) string {
	var kana, hangul, han, cyrillic, latin bool
	for _, r := range text {
		switch {
		case (r >= 0x3040 && r <= 0x30FF):
			kana = true
		case (r >= 0xAC00 && r <= 0xD7AF) || (r >= 0x1100 && r <= 0x11FF):
			hangul = true
		case (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF):
			han = true
		case r >= 0x0400 && r <= 0x04FF:
			cyrillic = true
		case (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= 0x00C0 && r <= 0x024F):
			latin = true
		}
	}
	switch {
	case kana:
		return "ja"
	case hangul:
		return "ko"
	case han && !latin:
		return langChinese
	case cyrillic:
		return "ru"
	default:
		return langEnglish
	}
}

// resolveLanguagesFrom 在用户指定源语言时跳过本地检测。
func resolveLanguagesFrom(text, source, target string) (sourceLang, targetLang string) {
	sourceLang = strings.TrimSpace(source)
	if sourceLang == "" || sourceLang == langAuto {
		sourceLang = detectLanguage(text)
	}
	targetLang = strings.TrimSpace(target)
	if targetLang == "" || targetLang == langAuto {
		if sourceLang == langChinese {
			return sourceLang, langEnglish
		}
		return sourceLang, langChinese
	}
	if targetLang == sourceLang {
		if sourceLang == langEnglish {
			return sourceLang, langChinese
		}
		return sourceLang, langEnglish
	}
	return sourceLang, targetLang
}

// languageName 把语言标签转成当前界面语言下的名称。
func languageName(tag string) string {
	switch tag {
	case langChinese:
		return locales.T("lang_zh_hans")
	case langEnglish:
		return locales.T("lang_en")
	case "ja":
		return locales.T("lang_ja")
	case "ko":
		return locales.T("lang_ko")
	case "ru":
		return locales.T("lang_ru")
	case "fr":
		return locales.T("lang_fr")
	case "de":
		return locales.T("lang_de")
	case "es":
		return locales.T("lang_es")
	default:
		return tag
	}
}
