package screenshot

import (
	"image"
	"math"

	"github.com/egoist/mygo/ui"
)

// 本文件是选区与图形的几何调整：8 向手柄命中 / 选区缩放 / 选区平移 /
// Shape 命中与移动。事件分派入口在 input.go。

// handlePoints 返回选区矩形 8 个调整手柄的 DIP 坐标：
// 0=左上 1=上 2=右上 3=右 4=右下 5=下 6=左下 7=左。
func handlePoints(r ui.Rect) [8]ui.Point {
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	return [8]ui.Point{
		{X: r.X, Y: r.Y}, {X: cx, Y: r.Y}, {X: r.X + r.W, Y: r.Y}, {X: r.X + r.W, Y: cy},
		{X: r.X + r.W, Y: r.Y + r.H}, {X: cx, Y: r.Y + r.H}, {X: r.X, Y: r.Y + r.H}, {X: r.X, Y: cy},
	}
}

// hitResizeHandle 报告物理像素点命中的手柄索引（-1 无）。
func (ov *overlay) hitResizeHandle(cur image.Point) int {
	const hit = 6.0 // 命中半径（DIP）
	p := ov.physPointToDip(cur)
	for i, hp := range handlePoints(ov.physRectToDip(ov.region)) {
		if absf(p.X-hp.X) <= hit && absf(p.Y-hp.Y) <= hit {
			return i
		}
	}
	return -1
}

// applyResize 按当前手柄调整选区：对边保持不动，钳制最小边长与屏幕边界。
func (ov *overlay) applyResize(cur image.Point) {
	x := clampi(cur.X, ov.bounds.Min.X, ov.bounds.Max.X)
	y := clampi(cur.Y, ov.bounds.Min.Y, ov.bounds.Max.Y)
	r := ov.region
	switch ov.resizeHandle {
	case 0:
		r.Min.X, r.Min.Y = x, y
	case 1:
		r.Min.Y = y
	case 2:
		r.Max.X, r.Min.Y = x, y
	case 3:
		r.Max.X = x
	case 4:
		r.Max.X, r.Max.Y = x, y
	case 5:
		r.Max.Y = y
	case 6:
		r.Min.X, r.Max.Y = x, y
	case 7:
		r.Min.X = x
	}
	// 最小边长：拖动左边/上边时钉住 Min，否则钉住 Max。
	left := ov.resizeHandle == 0 || ov.resizeHandle == 6 || ov.resizeHandle == 7
	top := ov.resizeHandle == 0 || ov.resizeHandle == 1 || ov.resizeHandle == 2
	if r.Dx() < minRegionPixels {
		if left {
			r.Min.X = r.Max.X - minRegionPixels
		} else {
			r.Max.X = r.Min.X + minRegionPixels
		}
	}
	if r.Dy() < minRegionPixels {
		if top {
			r.Min.Y = r.Max.Y - minRegionPixels
		} else {
			r.Max.Y = r.Min.Y + minRegionPixels
		}
	}
	ov.region = r
}

// hitShape 返回最上层命中的已固化 Shape（物理像素，后画的优先）。
func (ov *overlay) hitShape(cur image.Point) (Shape, bool) {
	list := ov.shapes.committed()
	for i := len(list) - 1; i >= 0; i-- {
		if pointInRect(cur, ov.shapeBounds(list[i])) {
			return list[i], true
		}
	}
	return Shape{}, false
}

// shapeBounds 计算 Shape 的物理像素包围盒；细线类外扩命中容差。
func (ov *overlay) shapeBounds(sh Shape) image.Rectangle {
	switch sh.Type {
	case ShapeBrush:
		if len(sh.Points) == 0 {
			return image.Rectangle{}
		}
		r := image.Rect(sh.Points[0].X, sh.Points[0].Y, sh.Points[0].X+1, sh.Points[0].Y+1)
		for _, p := range sh.Points[1:] {
			r = r.Union(image.Rect(p.X, p.Y, p.X+1, p.Y+1))
		}
		return padRect(r, max(sh.Width/2, 6*ov.scale))
	case ShapeLine, ShapeArrow:
		return padRect(sh.boundsRect(), max(sh.Width/2, 6*ov.scale))
	case ShapeMosaic:
		if sh.BoxMode {
			// 框选模式以框选矩形为准（块网格会整块外扩）。
			return sh.boundsRect()
		}
		if len(sh.Blocks) == 0 {
			return sh.boundsRect()
		}
		b := sh.Blocks[0]
		minX, minY, maxX, maxY := b.X, b.Y, b.X, b.Y
		for _, blk := range sh.Blocks[1:] {
			minX = min(minX, blk.X)
			minY = min(minY, blk.Y)
			maxX = max(maxX, blk.X)
			maxY = max(maxY, blk.Y)
		}
		size := sh.Block
		return image.Rect(ov.bounds.Min.X+minX*size, ov.bounds.Min.Y+minY*size,
			ov.bounds.Min.X+(maxX+1)*size, ov.bounds.Min.Y+(maxY+1)*size)
	case ShapeText:
		size := sh.FontSize * sh.Scale
		w := textWidthPx(sh.Text, size)
		return image.Rect(sh.Start.X, sh.Start.Y, sh.Start.X+int(w), sh.Start.Y+int(size*1.4))
	default:
		return sh.boundsRect()
	}
}

// applyMove 平移选中的 Shape：位移从按下时快照计算并钳制包围盒不越出选区。
func (ov *overlay) applyMove(cur image.Point) {
	ob := ov.shapeBounds(ov.moveOrig)
	if ob.Empty() {
		ov.moving = false
		return
	}
	dx, dy := cur.X-ov.moveStart.X, cur.Y-ov.moveStart.Y
	if ob.Dx() <= ov.region.Dx() {
		dx = clampi(dx, ov.region.Min.X-ob.Min.X, ov.region.Max.X-ob.Max.X)
	}
	if ob.Dy() <= ov.region.Dy() {
		dy = clampi(dy, ov.region.Min.Y-ob.Min.Y, ov.region.Max.Y-ob.Max.Y)
	}
	ov.shapes.translate(ov.selectedID, ov.moveOrig, dx, dy)
}

// applyRegionMove 平移选区：从按下时快照计算位移并钳制在屏幕内。
func (ov *overlay) applyRegionMove(cur image.Point) {
	dx, dy := cur.X-ov.regionStart.X, cur.Y-ov.regionStart.Y
	r := ov.regionOrig
	if r.Dx() <= ov.bounds.Dx() {
		dx = clampi(dx, ov.bounds.Min.X-r.Min.X, ov.bounds.Max.X-r.Max.X)
	}
	if r.Dy() <= ov.bounds.Dy() {
		dy = clampi(dy, ov.bounds.Min.Y-r.Min.Y, ov.bounds.Max.Y-r.Max.Y)
	}
	ov.region = r.Add(image.Point{X: dx, Y: dy})
}

// textWidthPx 估算文本物理像素宽度（命中与选框的粗略近似）。
func textWidthPx(s string, sizePx float64) float64 {
	w := 0.0
	for _, r := range s {
		if r < 128 {
			w += 0.6
		} else {
			w += 1.0
		}
	}
	return w * sizePx
}

// padRect 把矩形向四周外扩 pad 物理像素。
func padRect(r image.Rectangle, pad float64) image.Rectangle {
	p := int(math.Ceil(pad))
	return image.Rect(r.Min.X-p, r.Min.Y-p, r.Max.X+p, r.Max.Y+p)
}
