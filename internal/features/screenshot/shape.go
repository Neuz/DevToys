package screenshot

import (
	"image"
	"math"

	"github.com/egoist/mygo/ui"
)

// ShapeType 是标注图形的类别。
type ShapeType int

// 标注图形类别。
const (
	ShapeRect ShapeType = iota
	ShapeEllipse
	ShapeLine
	ShapeArrow
	ShapeBrush
	ShapeMosaic
	ShapeText
)

// Shape 是一个独立的标注对象。整个标注态就是一张 []Shape 有序切片：
// 每次渲染先画冻结背景，再按序遍历切片把矢量图形画上去；
// 只有「完成 / 保存」时才把全部 Shape 光栅化合并到输出图像。
//
// 坐标约定：Start/End/Points 用虚拟桌面物理像素；马赛克的块列表
// 相对截图原点。交互三段式：MouseDown 创建临时 Shape 推入列表
// （drawing 中）→ MouseMove 不断更新 End / 追加点并重绘（预览态）→
// MouseUp 固化。
type Shape struct {
	ID    int
	Type  ShapeType
	Start image.Point // 起点（线 / 箭头的尾端；文本锚点）
	End   image.Point // 终点（矩形 / 椭圆的对角点）
	Color ui.Color    // 线条 / 字色
	Width float64     // 物理像素线宽
	Text  string      // 仅文本有效
	// FontSize 是文本字号（DIP）；Scale 是创建时的显示器缩放，
	// 光栅化时用 FontSize × Scale 得物理像素字号。
	FontSize float64
	Scale    float64
	// IsSelected 标记选中编辑状态（首版不做移动/改形编辑，仅模型预留）。
	IsSelected bool
	// Points 是画笔路径。
	Points []image.Point
	// Blocks 是马赛克的块坐标列表（相对截图原点），Block 是块尺寸。
	Blocks []image.Point
	Block  int
	// BoxMode 标记框选马赛克：块网格按整块对齐会超出框选矩形，
	// 绘制 / 光栅化时用 boundsRect 裁剪，框选范围即打码最大区域。
	BoxMode bool
}

// boundsRect 返回 Start/End 围成的规范矩形。
func (sh Shape) boundsRect() image.Rectangle {
	return image.Rect(
		min(sh.Start.X, sh.End.X), min(sh.Start.Y, sh.End.Y),
		max(sh.Start.X, sh.End.X), max(sh.Start.Y, sh.End.Y),
	)
}

// shapeList 是 Shape 有序切片 + 重做栈。
// 临时 Shape 存在列表尾部；固化和撤销都会清掉重做。
type shapeList struct {
	shapes  []Shape
	redo    []Shape
	nextID  int
	drawing int // 正在绘制的临时 Shape 在 shapes 中的下标，-1 表示无
}

func newShapeList() *shapeList {
	return &shapeList{drawing: -1}
}

// begin 创建一个临时 Shape 推入列表尾部并标记为正在绘制。
// scale 供文本光栅化换算物理字号。
func (s *shapeList) begin(t ShapeType, pt image.Point, c ui.Color, width float64, scale float64) {
	s.discard()
	sh := Shape{
		ID:    s.nextID,
		Type:  t,
		Start: pt,
		End:   pt,
		Color: c,
		Width: width,
		Scale: scale,
	}
	s.nextID++
	s.shapes = append(s.shapes, sh)
	s.drawing = len(s.shapes) - 1
}

// update 更新正在绘制的 Shape 的终点（矩形 / 椭圆 / 线 / 箭头）。
func (s *shapeList) update(pt image.Point) {
	if s.drawing < 0 || s.drawing >= len(s.shapes) {
		return
	}
	s.shapes[s.drawing].End = pt
}

// appendPoint 给正在绘制的画笔追加点。
func (s *shapeList) appendPoint(pt image.Point) {
	if s.drawing < 0 || s.drawing >= len(s.shapes) {
		return
	}
	s.shapes[s.drawing].Points = append(s.shapes[s.drawing].Points, pt)
}

// setMosaic 更新正在绘制的马赛克块列表与块尺寸。
func (s *shapeList) setMosaic(blocks []image.Point, block int) {
	if s.drawing < 0 || s.drawing >= len(s.shapes) {
		return
	}
	s.shapes[s.drawing].Blocks = blocks
	s.shapes[s.drawing].Block = block
}

// markBox 把正在绘制的马赛克标记为框选模式（块需按 boundsRect 裁剪）。
func (s *shapeList) markBox() {
	if s.drawing < 0 || s.drawing >= len(s.shapes) {
		return
	}
	s.shapes[s.drawing].BoxMode = true
}

// setText 填写正在绘制的文本内容。
func (s *shapeList) setText(text string, fontSize float64) {
	if s.drawing < 0 || s.drawing >= len(s.shapes) {
		return
	}
	s.shapes[s.drawing].Text = text
	s.shapes[s.drawing].FontSize = fontSize
}

// commit 确认绘制完成（MouseUp / OnSubmit）：固化临时 Shape 并清空重做。
// 位移过小的拖拽、单点画笔、无块马赛克、空文本视为无效，直接丢弃。
func (s *shapeList) commit(minDragPixels int) bool {
	if s.drawing < 0 {
		return false
	}
	sh := s.shapes[s.drawing]
	s.drawing = -1
	valid := true
	switch sh.Type {
	case ShapeRect, ShapeEllipse, ShapeLine, ShapeArrow:
		valid = max(abs(sh.End.X-sh.Start.X), abs(sh.End.Y-sh.Start.Y)) > minDragPixels
	case ShapeBrush:
		valid = len(sh.Points) > 1
	case ShapeMosaic:
		valid = len(sh.Blocks) > 0
	case ShapeText:
		valid = sh.Text != ""
	}
	if !valid {
		s.shapes = s.shapes[:len(s.shapes)-1]
		return false
	}
	s.redo = nil
	return true
}

// discard 丢弃正在绘制的临时 Shape。
func (s *shapeList) discard() {
	if s.drawing < 0 || s.drawing >= len(s.shapes) {
		return
	}
	s.shapes = append(s.shapes[:s.drawing], s.shapes[s.drawing+1:]...)
	s.drawing = -1
}

// drawingShape 返回正在绘制的临时 Shape；没有则 ok=false。
func (s *shapeList) drawingShape() (Shape, bool) {
	if s.drawing < 0 || s.drawing >= len(s.shapes) {
		return Shape{}, false
	}
	return s.shapes[s.drawing], true
}

// committed 返回全部已固化的 Shape（不含正在绘制的临时项）。
func (s *shapeList) committed() []Shape {
	if s.drawing < 0 {
		return s.shapes
	}
	return s.shapes[:s.drawing]
}

// pop 撤销栈顶已固化 Shape；没有可撤销时返回 false。
func (s *shapeList) pop() bool {
	fixed := s.committed()
	if len(fixed) == 0 {
		return false
	}
	last := fixed[len(fixed)-1]
	s.shapes = fixed[:len(fixed)-1]
	s.redo = append(s.redo, last)
	return true
}

// pushRedo 恢复一次撤销；没有可重做时返回 false。
func (s *shapeList) pushRedo() bool {
	if len(s.redo) == 0 {
		return false
	}
	s.shapes = append(s.shapes, s.redo[len(s.redo)-1])
	s.redo = s.redo[:len(s.redo)-1]
	return true
}

// find 按 ID 返回 Shape（含正在绘制的临时项）。
func (s *shapeList) find(id int) (Shape, bool) {
	for _, sh := range s.shapes {
		if sh.ID == id {
			return sh, true
		}
	}
	return Shape{}, false
}

// translate 把指定 ID 的 Shape 几何从按下时的快照 orig 平移 (dx,dy)。
// 每帧从快照重算避免累积误差；马赛克块按块尺寸量化平移。
func (s *shapeList) translate(id int, orig Shape, dx, dy int) bool {
	for i := range s.shapes {
		if s.shapes[i].ID != id {
			continue
		}
		sh := &s.shapes[i]
		d := image.Point{X: dx, Y: dy}
		sh.Start = orig.Start.Add(d)
		sh.End = orig.End.Add(d)
		if len(orig.Points) > 0 {
			sh.Points = make([]image.Point, len(orig.Points))
			for j, p := range orig.Points {
				sh.Points[j] = p.Add(d)
			}
		}
		if len(orig.Blocks) > 0 && sh.Block > 0 {
			q := image.Point{X: roundDiv(dx, sh.Block), Y: roundDiv(dy, sh.Block)}
			sh.Blocks = make([]image.Point, len(orig.Blocks))
			for j, b := range orig.Blocks {
				sh.Blocks[j] = b.Add(q)
			}
		}
		return true
	}
	return false
}

// boxBlocks 生成矩形区域内全部马赛克块（框选模式）。
// origin 是截图原点（= bounds.Min），块坐标相对它。
func boxBlocks(r image.Rectangle, origin image.Point, block int) []image.Point {
	if block <= 0 {
		return nil
	}
	var blocks []image.Point
	bx0, by0 := (r.Min.X-origin.X)/block, (r.Min.Y-origin.Y)/block
	bx1, by1 := (r.Max.X-origin.X+block-1)/block, (r.Max.Y-origin.Y+block-1)/block
	for by := by0; by < by1; by++ {
		for bx := bx0; bx < bx1; bx++ {
			blocks = append(blocks, image.Point{X: bx, Y: by})
		}
	}
	return blocks
}

// smearBlocks 沿画笔路径生成马赛克块（涂抹模式）。
// radius 是笔刷半径（块数，可为小数，由粗细滑杆映射），路径点之间的空隙按步长补点。
func smearBlocks(points []image.Point, origin image.Point, block int, radius float64) []image.Point {
	if block <= 0 || len(points) == 0 {
		return nil
	}
	seen := map[image.Point]bool{}
	var blocks []image.Point
	add := func(p image.Point) {
		bx := (p.X - origin.X) / block
		by := (p.Y - origin.Y) / block
		for dy := -int(radius) - 1; dy <= int(radius)+1; dy++ {
			for dx := -int(radius) - 1; dx <= int(radius)+1; dx++ {
				if float64(dx*dx+dy*dy) > radius*radius {
					continue
				}
				b := image.Point{X: bx + dx, Y: by + dy}
				if !seen[b] {
					seen[b] = true
					blocks = append(blocks, b)
				}
			}
		}
	}
	for i, p := range points {
		if i > 0 {
			prev := points[i-1]
			d := max(abs(p.X-prev.X), abs(p.Y-prev.Y))
			for step := block; step < d; step += block {
				t := float64(step) / float64(d)
				add(image.Point{
					X: prev.X + int(math.Round(t*float64(p.X-prev.X))),
					Y: prev.Y + int(math.Round(t*float64(p.Y-prev.Y))),
				})
			}
		}
		add(p)
	}
	return blocks
}
