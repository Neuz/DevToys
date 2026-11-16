// Package kit 提供应用共用的小部件：托盘图标。
package kit

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"runtime"
)

// TrayIcon 程序化生成托盘图标：macOS 用纯黑加透明（模板图标由系统着色），
// 其他平台用强调色，返回 PNG 字节。
func TrayIcon() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	ink := color.NRGBA{R: 0x4f, G: 0x8c, B: 0xff, A: 0xff}
	if runtime.GOOS == "darwin" {
		ink = color.NRGBA{A: 0xff}
	}
	// 一个简单的圆环加手柄，像一只工具箱的提手。
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			dx, dy := float64(x)-13, float64(y)-13
			if r := dx*dx + dy*dy; r >= 7*7 && r <= 11*11 {
				img.Set(x, y, ink)
			}
		}
	}
	for i := 0; i < 9; i++ {
		for j := 0; j < 3; j++ {
			img.Set(21+i+j, 21+i, ink)
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}
