package screenshot

import (
	"fmt"
	"image"

	"github.com/kbinani/screenshot"
)

// captureDisplays 抓取所有显示器，返回每块的物理像素截图。
// 顺序与 mygo.Screen.Displays() 一一对应由调用方保证；这里只按
// 虚拟桌面坐标抓取，多显示器返回多张。
func captureDisplays() ([]image.RGBA, error) {
	n := screenshot.NumActiveDisplays()
	if n <= 0 {
		return nil, fmt.Errorf("没有活动的显示器")
	}
	shots := make([]image.RGBA, 0, n)
	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)
		if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
			continue
		}
		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			return nil, fmt.Errorf("抓取显示器 %d 失败: %w", i, err)
		}
		shots = append(shots, *img)
	}
	if len(shots) == 0 {
		return nil, fmt.Errorf("没有可抓取的显示器")
	}
	return shots, nil
}

// windowRect 是一个候选窗口在虚拟桌面中的物理像素矩形。
type windowRect struct {
	bounds image.Rectangle
}

// hitWindow 返回包含 point 的第一个窗口（z-order 优先，自上而下）；
// 没有命中返回 -1。
func hitWindow(windows []windowRect, point image.Point) int {
	for i, w := range windows {
		if point.In(w.bounds) {
			return i
		}
	}
	return -1
}
