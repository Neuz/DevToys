package qrcode

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/Cyenoch/mygoui/content"
	"github.com/egoist/mygo/ui"
)

func TestEncodePNG(t *testing.T) {
	data, err := encodePNG("https://example.com", content.QRMedium, 160, ui.Hex("#000000"), ui.Hex("#ffffff"))
	if err != nil {
		t.Fatalf("encodePNG: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := img.Bounds().Dx(); got != 160 {
		t.Fatalf("宽 = %d, want 160", got)
	}
	if got := img.Bounds().Dy(); got != 160 {
		t.Fatalf("高 = %d, want 160", got)
	}
	// 左上角落在留白区，应当是背景色。
	if r, g, b, _ := img.At(0, 0).RGBA(); r != 0xffff || g != 0xffff || b != 0xffff {
		t.Fatalf("留白像素 = %04x %04x %04x, want 白色", r, g, b)
	}
}

func TestEncodePNGTooLong(t *testing.T) {
	value := string(bytes.Repeat([]byte("x"), 10_000))
	if _, err := encodePNG(value, content.QRHigh, 160, ui.Hex("#000000"), ui.Hex("#ffffff")); err == nil {
		t.Fatal("超长内容应当返回错误")
	}
}
