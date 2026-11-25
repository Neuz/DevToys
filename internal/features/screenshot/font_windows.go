//go:build windows

package screenshot

import (
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

// fontCandidates 是按优先级尝试的系统字体（中文优先微软雅黑）。
var fontCandidates = []string{
	`msyh.ttc`,
	`msyh.ttf`,
	`simsun.ttc`,
	`simhei.ttf`,
	`arial.ttf`,
}

// newFaceFromFont 由解析出的字体按物理像素字号构建 Face。
var newFaceFromFont func(pxSize float64) (font.Face, error)

var fontOnce sync.Once

// loadFace 加载系统字体并按物理像素字号构建 Face。
// 找不到可用字体时返回 ok=false（文本标注在输出里缺席，不影响其余功能）。
func loadFace(pxSize float64) (font.Face, bool) {
	if pxSize < 4 {
		return nil, false
	}
	fontOnce.Do(loadSystemFont)
	if newFaceFromFont == nil {
		return nil, false
	}
	f, err := newFaceFromFont(pxSize)
	if err != nil {
		return nil, false
	}
	return f, true
}

// loadSystemFont 从 C:\Windows\Fonts 查找并解析第一个可用字体。
// ParseCollection 同时兼容 ttc 集合与单个 ttf。
// 特例：os.ReadFile 是纯 I/O 阻塞调用，不依赖 COM / 窗口消息，
// 在调用方 goroutine（task Runner）执行安全。
func loadSystemFont() {
	dir := os.Getenv("WINDIR")
	if dir == "" {
		dir = `C:\Windows`
	}
	for _, name := range fontCandidates {
		data, err := os.ReadFile(filepath.Join(dir, "Fonts", name))
		if err != nil {
			continue
		}
		tc, err := opentype.ParseCollection(data)
		if err != nil {
			continue
		}
		f, err := tc.Font(0)
		if err != nil {
			continue
		}
		newFaceFromFont = func(px float64) (font.Face, error) {
			return opentype.NewFace(f, &opentype.FaceOptions{Size: px, DPI: 72, Hinting: font.HintingFull})
		}
		return
	}
}
