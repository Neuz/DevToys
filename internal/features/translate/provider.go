package translate

import (
	"context"
	"time"
)

// 翻译服务 ID，与配置里的 providers 键一致。
const (
	providerGoogle   = "google"
	providerBilibili = "bilibili"
)

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
// 实现方自带 HTTP 客户端拼装与错误处理，彼此独立、允许重复。
type Provider interface {
	ID() string
	Name() string
	Translate(ctx context.Context, req Request) (Result, error)
}

// maxRunes 返回文本的字符数；各服务的长度限制都按字符计算。
func maxRunes(text string) int {
	return len([]rune(text))
}
