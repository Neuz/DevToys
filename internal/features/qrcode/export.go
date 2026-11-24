package qrcode

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"

	"github.com/Cyenoch/mygoui/content"
	"github.com/egoist/mygo/ui"
	"rsc.io/qr"
)

// qrQuietModules 是二维码四周留白的标准模块数。
const qrQuietModules = 2

var qrLevels = [...]qr.Level{
	content.QRMedium:   qr.M,
	content.QRLow:      qr.L,
	content.QRQuartile: qr.Q,
	content.QRHigh:     qr.H,
}

// encodePNG 按当前参数生成 side×side 的 PNG。模块按整数倍缩放以保持锐利；
// 当符号本身大于 side 时会放大画布，而不是模糊缩放。
func encodePNG(value string, level content.QRLevel, side int, fg, bg ui.Color) ([]byte, error) {
	code, err := qr.Encode(value, qrLevels[level])
	if err != nil {
		return nil, fmt.Errorf("qrcode: encode %d-byte value: %w", len(value), err)
	}
	total := code.Size + 2*qrQuietModules
	scale := side / total
	if scale < 1 {
		scale = 1
	}
	dim := total * scale
	if dim < side {
		dim = side
	}

	img := image.NewRGBA(image.Rect(0, 0, dim, dim))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: toRGBA(bg)}, image.Point{}, draw.Src)
	dark := &image.Uniform{C: toRGBA(fg)}
	offset := (dim - total*scale) / 2
	for y := range code.Size {
		for x := range code.Size {
			if !code.Black(x, y) {
				continue
			}
			x0 := offset + (x+qrQuietModules)*scale
			y0 := offset + (y+qrQuietModules)*scale
			draw.Draw(img, image.Rect(x0, y0, x0+scale, y0+scale), dark, image.Point{}, draw.Src)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// toRGBA 把界面颜色转成不透明的 PNG 像素；颜色选择器隐藏了 Alpha。
func toRGBA(c ui.Color) color.RGBA {
	return color.RGBA{R: c.R, G: c.G, B: c.B, A: 0xff}
}
