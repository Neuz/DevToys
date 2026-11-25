package screenshot

import (
	"image"
	"math"

	"github.com/egoist/mygo/ui"
)

// 坐标约定：业务状态（region / pressAt / 指针）一律是虚拟桌面物理像素，
// 覆盖层元素布局是 DIP（原点 = 本显示器物理 bounds 左上角）。
// 本文件集中两类换算与纯数学 helper，全部无副作用、可单测。

// dipToPhysical 把本显示器内 DIP 坐标换回物理像素偏移。
func (ov *overlay) dipToPhysical(v float32) int { return int(float64(v) * ov.scale) }

// physPointToDip 把虚拟桌面物理像素点换算成覆盖层元素内的 DIP 坐标
// （元素原点 = 本显示器物理 bounds 的左上角）。
func (ov *overlay) physPointToDip(pt image.Point) ui.Point {
	return ui.Point{
		X: float32(float64(pt.X-ov.bounds.Min.X) / ov.scale),
		Y: float32(float64(pt.Y-ov.bounds.Min.Y) / ov.scale),
	}
}

// physRectToDip 把虚拟桌面物理像素矩形换算成覆盖层元素内的 DIP 矩形。
func (ov *overlay) physRectToDip(r image.Rectangle) ui.Rect {
	return ui.Rect{
		X: float32(float64(r.Min.X-ov.bounds.Min.X) / ov.scale),
		Y: float32(float64(r.Min.Y-ov.bounds.Min.Y) / ov.scale),
		W: float32(float64(r.Dx()) / ov.scale),
		H: float32(float64(r.Dy()) / ov.scale),
	}
}

// sqrtf 是 float32 平方根。
func sqrtf(v float32) float32 { return float32(math.Sqrt(float64(v))) }

// absf 是 float32 绝对值。
func absf(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// abs 是 int 绝对值。
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// clampf 把 v 限制在 [lo, hi]。
func clampf(v, lo, hi float32) float32 {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// clampi 把 v 限制在 [lo, hi]。
func clampi(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// pointInRect 报告点是否在矩形内（开区间）。
func pointInRect(pt image.Point, r image.Rectangle) bool {
	return pt.X >= r.Min.X && pt.Y >= r.Min.Y && pt.X < r.Max.X && pt.Y < r.Max.Y
}

// normRect 返回两点围成的规范矩形。
func normRect(a, b image.Point) image.Rectangle {
	return image.Rect(min(a.X, b.X), min(a.Y, b.Y), max(a.X, b.X), max(a.Y, b.Y))
}

// roundDiv 四舍五入取整除（马赛克块平移量化用）。
func roundDiv(a, b int) int {
	if b <= 0 {
		return 0
	}
	if a >= 0 {
		return (a + b/2) / b
	}
	return -((-a + b/2) / b)
}
