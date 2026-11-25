package screenshot

import (
	"fmt"
	"image"
	"math"

	"github.com/Neuz/DevToys/internal/features/screenshot/locales"
	"github.com/egoist/mygo/ui"
)

// paint 按渲染顺序（Z-Order）画整帧：
// 1 底图冻结截图 → 2 暗色遮罩 → 3 选区高亮（选区内无遮罩）
// → 4 遍历 []Shape 标注 → 5 预览辅助线 → 6 提示文字（工具栏是真实组件）。
func (ov *overlay) paint(p *ui.Painter, r ui.Rect) {
	if ov.bitmap == nil {
		ov.bitmap = ui.NewBitmap(ov.shot)
	}
	// 1 底图。
	p.Image(ov.bitmap, r, ui.FillBox)
	// 2 暗色遮罩。
	p.Fill(r, ui.RGBA(0, 0, 0, 0.45), 0)

	if ov.state == stateSelect {
		ov.paintSelect(p, r)
		return
	}
	ov.paintAnnotate(p)
}

// drawImageRegion 把冻结截图的物理像素矩形 1:1 画回它对应的 DIP 位置。
// ImageRegion 的 src 是位图像素空间（原点 = bounds.Min），dst 是 DIP。
func (ov *overlay) drawImageRegion(p *ui.Painter, sel image.Rectangle) {
	src := ui.Rect{
		X: float32(sel.Min.X - ov.bounds.Min.X),
		Y: float32(sel.Min.Y - ov.bounds.Min.Y),
		W: float32(sel.Dx()),
		H: float32(sel.Dy()),
	}
	p.ImageRegion(ov.bitmap, src, ov.physRectToDip(sel), ui.ImageOptions{})
}

// paintSelect 画选择态：窗口高亮或拖拽矩形（选区内保持明亮），顶部有操作提示。
func (ov *overlay) paintSelect(p *ui.Painter, r ui.Rect) {
	paintHint(p, r, locales.T("overlay_hint"))

	var sel image.Rectangle
	if ov.dragging {
		sel, _ = ov.pickRegion()
	} else if ov.hover >= 0 && ov.hover < len(ov.windows) {
		sel = ov.windows[ov.hover].bounds.Intersect(ov.bounds)
	}
	if sel.Empty() {
		return
	}
	ov.drawImageRegion(p, sel)
	p.Stroke(ov.physRectToDip(sel), ui.RGB(255, 255, 255), 0, 2)
	ov.paintRegionSize(p, sel)
}

// paintHint 在屏幕顶部居中画提示文字。
func paintHint(p *ui.Painter, r ui.Rect, hint string) {
	hx := r.X + r.W/2 - float32(len([]rune(hint)))*7
	p.Text(hx, r.Y+12, hint, 14, ui.RGBA(255, 255, 255, 0.9))
}

// paintRegionSize 在选区左上角画尺寸标签（如 1920*1080）；越出屏幕顶部时移入选区内。
func (ov *overlay) paintRegionSize(p *ui.Painter, sel image.Rectangle) {
	label := fmt.Sprintf("%d*%d", sel.Dx(), sel.Dy())
	sr := ov.physRectToDip(sel)
	const (
		tw = 72.0
		th = 20.0
	)
	x := sr.X
	y := sr.Y - th - 4
	if y < 2 {
		y = sr.Y + 4
	}
	box := ui.Rect{X: x, Y: y, W: tw, H: th}
	p.Fill(box, ui.RGBA(32, 32, 32, 0.9), 4)
	p.Text(x+8, y+3, label, 12, ui.RGB(255, 255, 255))
}

// paintAnnotate 画标注态：选区明亮、Shape 矢量预览。
func (ov *overlay) paintAnnotate(p *ui.Painter) {
	// 3 选区明亮（选区内不做遮罩）。
	ov.drawImageRegion(p, ov.region)
	rr := ov.physRectToDip(ov.region)
	p.Stroke(rr, ui.RGB(255, 255, 255), 0, 2)
	// 4 遍历 []Shape 依次绘制（含正在绘制的预览项）。
	ov.paintShapes(p)
	// 5 选中 Shape 高亮框（toolNone 选择态）。
	if ov.tool == toolNone && ov.selectedID >= 0 {
		if sh, ok := ov.shapes.find(ov.selectedID); ok {
			b := ov.shapeBounds(sh)
			if !b.Empty() {
				p.Stroke(ov.physRectToDip(b), ui.RGB(45, 125, 246), 0, 1.5)
			}
		}
	}
	// 6 选区 8 向调整手柄；拖拽中同步显示尺寸标签。
	for _, hp := range handlePoints(rr) {
		const hs = 4.0
		box := ui.Rect{X: hp.X - hs, Y: hp.Y - hs, W: hs * 2, H: hs * 2}
		p.Fill(box, ui.RGB(255, 255, 255), 1)
		p.Stroke(box, ui.RGB(45, 125, 246), 1, 1)
	}
	if ov.resizeHandle >= 0 || ov.regionDrag {
		ov.paintRegionSize(p, ov.region)
	}
}

// physThick 把线宽滑杆值（DIP）换算为物理像素线宽。
func (ov *overlay) physThick() float64 { return ov.thick * ov.scale }

// paintShapes 按序遍历 Shape 列表绘制全部已固化图形，
// 再叠加正在绘制的临时 Shape（预览态）。矢量绘制不做光栅化。
func (ov *overlay) paintShapes(p *ui.Painter) {
	for _, sh := range ov.shapes.committed() {
		ov.paintShape(p, sh, false)
	}
	if sh, ok := ov.shapes.drawingShape(); ok {
		ov.paintShape(p, sh, true)
	}
}

// paintShape 画一个 Shape；live 表示正在绘制的预览态。
func (ov *overlay) paintShape(p *ui.Painter, sh Shape, live bool) {
	switch sh.Type {
	case ShapeRect:
		r := ov.physRectToDip(sh.boundsRect())
		if live {
			p.Stroke(r, ui.RGBA(255, 255, 255, 0.5), 0, 1)
		}
		p.Stroke(r, sh.Color, 0, ov.dipThick(sh.Width))
	case ShapeEllipse:
		r := ov.physRectToDip(sh.boundsRect())
		if live {
			p.Stroke(r, ui.RGBA(255, 255, 255, 0.5), 0, 1)
		}
		ov.paintEllipse(p, r, sh.Color, ov.dipThick(sh.Width))
	case ShapeLine:
		a, b := ov.physPointToDip(sh.Start), ov.physPointToDip(sh.End)
		p.StrokePolyline([][2]float32{{a.X, a.Y}, {b.X, b.Y}}, ov.dipThick(sh.Width), sh.Color)
	case ShapeArrow:
		ov.paintArrow(p, sh.Start, sh.End, sh.Color, sh.Width)
	case ShapeBrush:
		ov.paintPolyline(p, sh.Points, sh.Color, sh.Width)
	case ShapeMosaic:
		if live && len(sh.Blocks) == 0 {
			// 框选模式拖拽中：矩形样式预览，粗细固定 2，松开才打码。
			p.Stroke(ov.physRectToDip(sh.boundsRect()), sh.Color, 0, 2)
			return
		}
		clip := image.Rectangle{}
		if sh.BoxMode {
			clip = sh.boundsRect()
		}
		ov.paintMosaic(p, sh.Blocks, sh.Block, clip)
	case ShapeText:
		ov.paintText(p, sh)
	}
}

// paintEllipse 用参数方程采样画椭圆轮廓（Painter 无原生椭圆）。
func (ov *overlay) paintEllipse(p *ui.Painter, r ui.Rect, c ui.Color, w float32) {
	const steps = 64
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	rx, ry := r.W/2, r.H/2
	if rx <= 0 || ry <= 0 {
		return
	}
	pts := make([][2]float32, 0, steps+1)
	for i := 0; i <= steps; i++ {
		a := float64(i) / steps * 2 * math.Pi
		pts = append(pts, [2]float32{
			cx + float32(math.Cos(a))*rx,
			cy + float32(math.Sin(a))*ry,
		})
	}
	p.StrokePolyline(pts, w, c)
}

// paintArrow 画箭头：主干折线 + V 形头部（Painter 无多边形填充的近似）。
func (ov *overlay) paintArrow(p *ui.Painter, from, to image.Point, c ui.Color, width float64) {
	w := ov.dipThick(width)
	tail := ov.physPointToDip(from)
	head := ov.physPointToDip(to)
	dx := head.X - tail.X
	dy := head.Y - tail.Y
	length := sqrtf(dx*dx + dy*dy)
	if length < 1 {
		return
	}
	headLen := max(float32(width/ov.scale)*3, 12)
	// 主干画到头部后方，避免穿出。
	backX := head.X - dx/length*headLen
	backY := head.Y - dy/length*headLen
	p.StrokePolyline([][2]float32{{tail.X, tail.Y}, {backX, backY}}, w, c)
	// V 形头部。
	perpX := -dy / length
	perpY := dx / length
	half := headLen / 2
	p.StrokePolyline([][2]float32{
		{head.X - dx/length*headLen + perpX*half, head.Y - dy/length*headLen + perpY*half},
		{head.X, head.Y},
		{head.X - dx/length*headLen - perpX*half, head.Y - dy/length*headLen - perpY*half},
	}, w, c)
}

// paintPolyline 把物理像素点列换算成 DIP 后画折线。
func (ov *overlay) paintPolyline(p *ui.Painter, points []image.Point, c ui.Color, width float64) {
	if len(points) < 2 {
		return
	}
	pts := make([][2]float32, len(points))
	for i, pt := range points {
		d := ov.physPointToDip(pt)
		pts[i] = [2]float32{d.X, d.Y}
	}
	p.StrokePolyline(pts, ov.dipThick(width), c)
}

// paintMosaic 画马赛克块：对每块取冻结原图的均值色填充。
// 块坐标相对截图原点（= bounds.Min），采样用绝对像素坐标，
// 填充矩形换算成虚拟桌面坐标再转 DIP。
// paintMosaic 画马赛克块；clip 非空时块矩形按 clip 裁剪（框选模式）。
func (ov *overlay) paintMosaic(p *ui.Painter, blocks []image.Point, size int, clip image.Rectangle) {
	for _, b := range blocks {
		px, py := b.X*size, b.Y*size
		r, g, bl := mosaicMean(ov.shot, px+ov.bounds.Min.X, py+ov.bounds.Min.Y, size)
		rect := image.Rect(px+ov.bounds.Min.X, py+ov.bounds.Min.Y, px+ov.bounds.Min.X+size, py+ov.bounds.Min.Y+size)
		if !clip.Empty() {
			rect = rect.Intersect(clip)
			if rect.Empty() {
				continue
			}
		}
		p.Fill(ov.physRectToDip(rect), ui.RGB(r, g, bl), 0)
	}
}

// paintText 画文本预览。
func (ov *overlay) paintText(p *ui.Painter, sh Shape) {
	if sh.Text == "" {
		return
	}
	size := float32(sh.FontSize * ov.scale)
	origin := ov.physPointToDip(sh.Start)
	p.Text(origin.X, origin.Y, sh.Text, size, sh.Color)
}

// dipThick 把物理像素线宽换算成 DIP。
func (ov *overlay) dipThick(px float64) float32 {
	return float32(px / ov.scale)
}

// mosaicMean 取物理像素块的平均色。
func mosaicMean(img *image.RGBA, x, y, size int) (r, g, b uint8) {
	var rs, gs, bs, n int
	for dy := 0; dy < size; dy++ {
		for dx := 0; dx < size; dx++ {
			c := img.RGBAAt(x+dx, y+dy)
			rs += int(c.R)
			gs += int(c.G)
			bs += int(c.B)
			n++
		}
	}
	if n == 0 {
		return 0, 0, 0
	}
	return uint8(rs / n), uint8(gs / n), uint8(bs / n)
}
