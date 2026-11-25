//go:build !windows

package screenshot

import "golang.org/x/image/font"

// loadFace 在非 Windows 平台无系统字体解析，文本标注不输出。
func loadFace(pxSize float64) (font.Face, bool) {
	return nil, false
}
