package screenshot

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"

	"github.com/egoist/mygo/ui"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// rasterize 把冻结原图裁剪到选区作底图，再把全部已固化 Shape
// 光栅化合并进去（只在「完成 / 保存」时调用一次），返回 PNG。
// 预览阶段的矢量绘制（overlay.paintShapes）不落位图。
func rasterize(shot *image.RGBA, region image.Rectangle, shapes []Shape) ([]byte, error) {
	// 拷到原点为 (0,0) 的新画布：SubImage 保留原 Bounds().Min，
	// 直接编码会带偏移；draw.Draw 以 region.Min 为源起点规整到原点。
	canvas := image.NewRGBA(image.Rect(0, 0, region.Dx(), region.Dy()))
	draw.Draw(canvas, canvas.Bounds(), shot, region.Min, draw.Src)
	for _, sh := range shapes {
		rasterShape(canvas, shot, region, sh)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return nil, fmt.Errorf("编码 PNG 失败: %w", err)
	}
	return buf.Bytes(), nil
}

// rasterShape 把一个 Shape 光栅化到画布。坐标换算：虚拟桌面物理像素
// 减选区原点得局部坐标；马赛克块坐标本就相对截图原点。
func rasterShape(canvas *image.RGBA, shot *image.RGBA, region image.Rectangle, sh Shape) {
	origin := region.Min
	col := shapeRGBA(sh.Color)
	switch sh.Type {
	case ShapeRect:
		strokeRect(canvas, sh.boundsRect().Sub(origin), col, strokeWidth(sh))
	case ShapeEllipse:
		strokeEllipse(canvas, sh.boundsRect().Sub(origin), col, strokeWidth(sh))
	case ShapeLine:
		drawPath(canvas, []image.Point{sh.Start.Sub(origin), sh.End.Sub(origin)}, col, strokePoint(sh))
	case ShapeArrow:
		drawArrow(canvas, sh.Start.Sub(origin), sh.End.Sub(origin), col, strokePoint(sh))
	case ShapeBrush:
		local := make([]image.Point, len(sh.Points))
		for i, p := range sh.Points {
			local[i] = p.Sub(origin)
		}
		drawPath(canvas, local, col, strokePoint(sh))
	case ShapeMosaic:
		drawMosaicShape(canvas, shot, region, sh)
	case ShapeText:
		drawTextShape(canvas, sh, origin)
	}
}

// strokeWidth 是 Shape 的整数线宽（物理像素）。
func strokeWidth(sh Shape) int { return max(int(sh.Width+0.5), 1) }

// strokePoint 是 Shape 的线宽半径（浮点，物理像素）。
func strokePoint(sh Shape) float64 { return math.Max(sh.Width/2, 0.5) }

// shapeRGBA 把 ui.Color 转为标准 color.RGBA。
func shapeRGBA(c ui.Color) color.RGBA {
	return color.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

// strokeRect 用 image/draw 画矩形边框（四条边各画一条实心带）。
func strokeRect(canvas *image.RGBA, r image.Rectangle, c color.RGBA, thick int) {
	if r.Empty() {
		return
	}
	fill := image.NewUniform(c)
	draw.Draw(canvas, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+thick), fill, image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(r.Min.X, r.Max.Y-thick, r.Max.X, r.Max.Y), fill, image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(r.Min.X, r.Min.Y, r.Min.X+thick, r.Max.Y), fill, image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(r.Max.X-thick, r.Min.Y, r.Max.X, r.Max.Y), fill, image.Point{}, draw.Src)
}

// strokeEllipse 沿椭圆参数方程逐点填充圆盘，形成椭圆轮廓。
func strokeEllipse(canvas *image.RGBA, r image.Rectangle, c color.RGBA, thick int) {
	if r.Empty() {
		return
	}
	cx := float64(r.Min.X+r.Max.X) / 2
	cy := float64(r.Min.Y+r.Max.Y) / 2
	rx := float64(r.Dx()) / 2
	ry := float64(r.Dy()) / 2
	radius := float64(thick) / 2
	// 步数按周长走，保证圆盘相互重叠不出现锯齿。
	perimeter := math.Pi * (3*(rx+ry) - math.Sqrt((3*rx+ry)*(rx+3*ry)))
	steps := max(int(perimeter/2), 32)
	for i := 0; i <= steps; i++ {
		a := float64(i) / float64(steps) * 2 * math.Pi
		fillDisc(canvas, cx+math.Cos(a)*rx, cy+math.Sin(a)*ry, radius, c)
	}
}

// drawPath 沿折线逐点填充圆盘（线 / 画笔 / 箭头杆通用）。
func drawPath(canvas *image.RGBA, points []image.Point, c color.RGBA, radius float64) {
	for i, p := range points {
		if i == 0 {
			fillDisc(canvas, float64(p.X), float64(p.Y), radius, c)
			continue
		}
		prev := points[i-1]
		d := math.Hypot(float64(p.X-prev.X), float64(p.Y-prev.Y))
		steps := max(int(d), 1)
		for s := 0; s <= steps; s++ {
			t := float64(s) / float64(steps)
			fillDisc(canvas,
				float64(prev.X)+t*float64(p.X-prev.X),
				float64(prev.Y)+t*float64(p.Y-prev.Y),
				radius, c)
		}
	}
}

// drawArrow 画带实心三角头的箭头。
func drawArrow(canvas *image.RGBA, from, to image.Point, c color.RGBA, radius float64) {
	x0, y0 := float64(from.X), float64(from.Y)
	x1, y1 := float64(to.X), float64(to.Y)
	dx, dy := x1-x0, y1-y0
	length := math.Hypot(dx, dy)
	if length < 1 {
		return
	}
	headLen := math.Max(radius*6, 12)
	// 杆画到头部后方，避免线头穿出三角。
	ux, uy := dx/length, dy/length
	drawPath(canvas, []image.Point{from, {int(x1 - ux*headLen + 0.5), int(y1 - uy*headLen + 0.5)}}, c, radius)
	// 实心三角头。
	perpX, perpY := -uy, ux
	half := headLen / 2
	tri := [3]image.Point{
		{int(x1 + 0.5), int(y1 + 0.5)},
		{int(x1 - ux*headLen + perpX*half + 0.5), int(y1 - uy*headLen + perpY*half + 0.5)},
		{int(x1 - ux*headLen - perpX*half + 0.5), int(y1 - uy*headLen - perpY*half + 0.5)},
	}
	fillTriangle(canvas, tri, c)
}

// fillDisc 以 (cx,cy) 为圆心 r 为半径填充实心圆。
func fillDisc(canvas *image.RGBA, cx, cy, r float64, c color.RGBA) {
	if r < 0.5 {
		r = 0.5
	}
	x0, x1 := int(cx-r), int(cx+r)+1
	y0, y1 := int(cy-r), int(cy+r)+1
	b := canvas.Bounds().Intersect(image.Rect(x0, y0, x1, y1))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			if dx*dx+dy*dy <= r*r {
				canvas.SetRGBA(x, y, c)
			}
		}
	}
}

// fillTriangle 用重心坐标光栅化实心三角形。
func fillTriangle(canvas *image.RGBA, tri [3]image.Point, c color.RGBA) {
	minX := min(tri[0].X, tri[1].X, tri[2].X)
	minY := min(tri[0].Y, tri[1].Y, tri[2].Y)
	maxX := max(tri[0].X, tri[1].X, tri[2].X)
	maxY := max(tri[0].Y, tri[1].Y, tri[2].Y)
	b := canvas.Bounds().Intersect(image.Rect(minX, minY, maxX+1, maxY+1))
	ax, ay := float64(tri[0].X), float64(tri[0].Y)
	bx, by := float64(tri[1].X), float64(tri[1].Y)
	cx, cy := float64(tri[2].X), float64(tri[2].Y)
	area := (bx-ax)*(cy-ay) - (by-ay)*(cx-ax)
	if area == 0 {
		return
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			w0 := ((bx-ax)*(py-ay) - (by-ay)*(px-ax)) / area
			w1 := ((cx-bx)*(py-by) - (cy-by)*(px-bx)) / area
			w2 := 1 - w0 - w1
			if w0 >= 0 && w1 >= 0 && w2 >= 0 {
				canvas.SetRGBA(x, y, c)
			}
		}
	}
}

// drawMosaicShape 把冻结原图按块均值化后盖到画布。
// 块坐标相对截图原点；region 是选区（虚拟桌面坐标）。
func drawMosaicShape(canvas *image.RGBA, shot *image.RGBA, region image.Rectangle, sh Shape) {
	bounds := shot.Bounds()
	size := sh.Block
	if size <= 0 {
		return
	}
	// 框选模式：块网格整块对齐会超出框选矩形，按 boundsRect 裁剪。
	clip := image.Rectangle{}
	if sh.BoxMode {
		clip = sh.boundsRect()
	}
	for _, blk := range sh.Blocks {
		px, py := blk.X*size, blk.Y*size
		ax, ay := bounds.Min.X+px, bounds.Min.Y+py
		if ax < bounds.Min.X || ay < bounds.Min.Y || ax+size > bounds.Max.X || ay+size > bounds.Max.Y {
			continue
		}
		br := image.Rect(ax, ay, ax+size, ay+size)
		if !clip.Empty() {
			br = br.Intersect(clip)
			if br.Empty() {
				continue
			}
		}
		if !br.Overlaps(region) {
			continue
		}
		r, g, b := mosaicMean(shot, px+bounds.Min.X, py+bounds.Min.Y, size)
		fill := image.NewUniform(color.RGBA{R: r, G: g, B: b, A: 255})
		draw.Draw(canvas, br.Intersect(region).Sub(region.Min), fill, image.Point{}, draw.Src)
	}
}

// drawTextShape 在画布上光栅化文本。
// FontSize 是 DIP 字号，乘 Shape 创建时的 scale 得物理像素。
func drawTextShape(canvas *image.RGBA, sh Shape, origin image.Point) {
	if sh.Text == "" {
		return
	}
	pxSize := sh.FontSize * sh.Scale
	face, ok := loadFace(pxSize)
	if !ok {
		return
	}
	defer face.Close()
	drawer := &font.Drawer{
		Dst:  canvas,
		Src:  image.NewUniform(shapeRGBA(sh.Color)),
		Face: face,
		Dot:  fixed.P(sh.Start.X-origin.X, sh.Start.Y-origin.Y),
	}
	drawer.DrawString(sh.Text)
}
