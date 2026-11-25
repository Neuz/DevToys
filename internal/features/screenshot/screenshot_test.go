package screenshot

import (
	"bytes"
	"image"
	gopng "image/png"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestShapeListUndoRedo(t *testing.T) {
	s := newShapeList()
	if s.pop() {
		t.Fatal("空栈 pop 不应成功")
	}
	if s.pushRedo() {
		t.Fatal("空重做栈 pushRedo 不应成功")
	}

	c := ui.RGB(255, 0, 0)
	// 三段式：begin → update → commit。
	s.begin(ShapeRect, image.Pt(10, 10), c, 3, 1)
	if _, ok := s.drawingShape(); !ok {
		t.Fatal("begin 后应有临时 Shape")
	}
	s.update(image.Pt(30, 40))
	if !s.commit(minDragPixels) {
		t.Fatal("有效拖动应固化成功")
	}
	if len(s.shapes) != 1 || s.drawing != -1 {
		t.Fatalf("固化后 shapes = %v drawing = %d", s.shapes, s.drawing)
	}
	if sh := s.shapes[0]; sh.ID != 0 || sh.Start != image.Pt(10, 10) || sh.End != image.Pt(30, 40) {
		t.Fatalf("固化 Shape = %v", sh)
	}

	// 误触（位移过小）丢弃。
	s.begin(ShapeRect, image.Pt(50, 50), c, 3, 1)
	s.update(image.Pt(51, 50))
	if s.commit(minDragPixels) {
		t.Fatal("位移过小不应固化")
	}
	if len(s.shapes) != 1 {
		t.Fatalf("误触后 shapes = %v", s.shapes)
	}

	// 撤销 / 重做。
	if !s.pop() || len(s.shapes) != 0 {
		t.Fatalf("pop 后 shapes = %v", s.shapes)
	}
	if !s.pushRedo() || len(s.shapes) != 1 {
		t.Fatalf("pushRedo 后 shapes = %v", s.shapes)
	}
	// begin 丢弃残留临时 Shape；新提交清空重做栈。
	s.begin(ShapeRect, image.Pt(1, 1), c, 3, 1)
	s.update(image.Pt(9, 9))
	s.commit(minDragPixels)
	if s.pushRedo() {
		t.Fatal("新提交后重做栈应为空")
	}
}

func TestShapeListBrush(t *testing.T) {
	s := newShapeList()
	c := ui.RGB(0, 0, 255)
	s.begin(ShapeBrush, image.Pt(0, 0), c, 3, 1)
	s.appendPoint(image.Pt(10, 10))
	s.appendPoint(image.Pt(20, 20))
	if !s.commit(minDragPixels) {
		t.Fatal("多点画笔应固化成功")
	}
	sh := s.shapes[0]
	if sh.Type != ShapeBrush || len(sh.Points) != 2 {
		t.Fatalf("画笔 Shape = %v", sh)
	}
	// 单点画笔丢弃。
	s.begin(ShapeBrush, image.Pt(0, 0), c, 3, 1)
	s.appendPoint(image.Pt(1, 1))
	if s.commit(minDragPixels) {
		t.Fatal("单点画笔不应固化")
	}
}

func TestShapeListMosaic(t *testing.T) {
	s := newShapeList()
	c := ui.RGBA(0, 0, 0, 0)
	origin := image.Rect(0, 0, 400, 300)

	// 框选模式：update 出矩形，commit 前生成整框块。
	s.begin(ShapeMosaic, image.Pt(10, 10), c, 0, 1)
	s.setMosaic(nil, 16)
	s.update(image.Pt(100, 80))
	sh, _ := s.drawingShape()
	s.setMosaic(boxBlocks(sh.boundsRect(), origin.Min, 16), 16)
	if !s.commit(minDragPixels) {
		t.Fatal("框选马赛克应固化成功")
	}
	sh = s.shapes[0]
	if len(sh.Blocks) == 0 || sh.Block != 16 {
		t.Fatalf("框选马赛克块 = %v", sh)
	}
	// (10,10)-(100,80) / 16px 块：x 0..6 共 7 列，y 0..4 共 5 行。
	if len(sh.Blocks) != 35 {
		t.Fatalf("块数 = %d, want 35", len(sh.Blocks))
	}

	// 涂抹模式：沿路径生成块。
	s.begin(ShapeMosaic, image.Pt(16, 16), c, 0, 1)
	s.setMosaic(nil, 16)
	s.appendPoint(image.Pt(16, 16))
	s.appendPoint(image.Pt(80, 16))
	sh, _ = s.drawingShape()
	blocks := smearBlocks(sh.Points, origin.Min, 16, 1)
	s.setMosaic(blocks, 16)
	if !s.commit(minDragPixels) {
		t.Fatal("涂抹马赛克应固化成功")
	}
	if len(s.shapes[1].Blocks) < 4 {
		t.Fatalf("涂抹块过少 = %v", s.shapes[1].Blocks)
	}

	// 空块马赛克丢弃。
	s.begin(ShapeMosaic, image.Pt(0, 0), c, 0, 1)
	s.setMosaic(nil, 16)
	if s.commit(minDragPixels) {
		t.Fatal("空块马赛克不应固化")
	}
}

func TestShapeListText(t *testing.T) {
	s := newShapeList()
	c := ui.RGB(0, 0, 0)
	s.begin(ShapeText, image.Pt(20, 30), c, 0, 1.5)
	s.setText("你好", 18)
	if !s.commit(minDragPixels) {
		t.Fatal("非空文本应固化成功")
	}
	sh := s.shapes[0]
	if sh.Type != ShapeText || sh.Text != "你好" || sh.FontSize != 18 || sh.Scale != 1.5 {
		t.Fatalf("文本 Shape = %v", sh)
	}
	// 空文本丢弃。
	s.begin(ShapeText, image.Pt(0, 0), c, 0, 1)
	if s.commit(minDragPixels) {
		t.Fatal("空文本不应固化")
	}
}

func TestShapeListCommittedExcludesDrawing(t *testing.T) {
	s := newShapeList()
	c := ui.RGB(0, 255, 0)
	s.begin(ShapeRect, image.Pt(0, 0), c, 3, 1)
	s.update(image.Pt(20, 20))
	s.commit(minDragPixels)
	s.begin(ShapeArrow, image.Pt(1, 1), c, 3, 1)
	s.update(image.Pt(30, 30))
	got := s.committed()
	if len(got) != 1 {
		t.Fatalf("committed 应排除绘制中项，got %d", len(got))
	}
	s.discard()
	if len(s.shapes) != 1 {
		t.Fatalf("discard 后 shapes = %v", s.shapes)
	}
}

func TestSmearBlocks(t *testing.T) {
	// 单点、半径 1：3x3 圆形邻域（圆内 5 块）。
	blocks := smearBlocks([]image.Point{{X: 0, Y: 0}}, image.Point{}, 16, 1.0)
	if len(blocks) != 5 {
		t.Fatalf("单点半径1块数 = %d, want 5", len(blocks))
	}
	// 块坐标相对 origin 偏移。
	shifted := smearBlocks([]image.Point{{X: 100, Y: 50}}, image.Point{X: 16, Y: 0}, 16, 0.5)
	if len(shifted) != 1 || shifted[0].X != (100-16)/16 {
		t.Fatalf("偏移块 = %v", shifted)
	}
	// 半径越大块越多。
	wide := smearBlocks([]image.Point{{X: 0, Y: 0}}, image.Point{}, 16, 2.0)
	if len(wide) != 13 {
		t.Fatalf("单点半径2块数 = %d, want 13", len(wide))
	}
}

func TestBoxBlocks(t *testing.T) {
	r := image.Rect(20, 20, 100, 60)
	blocks := boxBlocks(r, image.Point{}, 16)
	// x: 20/16=1 .. 99/16=6 → 1..6；y: 20/16=1 .. 59/16=3 → 1..3。
	if len(blocks) != 6*3 {
		t.Fatalf("框选块数 = %d, want 18", len(blocks))
	}
}

func TestHitWindow(t *testing.T) {
	windows := []windowRect{
		{bounds: image.Rect(0, 0, 100, 100)},
		{bounds: image.Rect(50, 50, 200, 200)},
	}
	tests := []struct {
		pt   image.Point
		want int
	}{
		{image.Pt(10, 10), 0},
		{image.Pt(60, 60), 0}, // z-order 优先。
		{image.Pt(150, 150), 1},
		{image.Pt(300, 300), -1},
	}
	for _, tt := range tests {
		if got := hitWindow(windows, tt.pt); got != tt.want {
			t.Fatalf("hitWindow(%v) = %d, want %d", tt.pt, got, tt.want)
		}
	}
}

func TestPickRegion(t *testing.T) {
	ov := &overlay{
		bounds: image.Rect(0, 0, 800, 600),
	}

	// 位移太小：无效。
	ov.pressAt = image.Pt(100, 100)
	ov.dragCur = image.Pt(102, 101)
	if _, ok := ov.pickRegion(); ok {
		t.Fatal("小于最小拖拽距离不应产生选区")
	}
	// 位移够大：返回交集矩形。
	ov.dragCur = image.Pt(500, 400)
	r, ok := ov.pickRegion()
	if !ok || r != image.Rect(100, 100, 500, 400) {
		t.Fatalf("pickRegion = %v, %v", r, ok)
	}
	// 越界部分被裁掉。
	ov.pressAt = image.Pt(700, 500)
	ov.dragCur = image.Pt(900, 700)
	r, ok = ov.pickRegion()
	if !ok || r != image.Rect(700, 500, 800, 600) {
		t.Fatalf("pickRegion 越界 = %v, %v", r, ok)
	}
}

func TestRasterizeProducesPNG(t *testing.T) {
	shot := image.NewRGBA(image.Rect(0, 0, 60, 40))
	shapes := []Shape{
		{Type: ShapeRect, Start: image.Pt(5, 5), End: image.Pt(30, 20), Width: 3, Color: ui.RGB(232, 17, 45)},
		{Type: ShapeEllipse, Start: image.Pt(2, 2), End: image.Pt(50, 38), Width: 2, Color: ui.RGB(45, 125, 246)},
		{Type: ShapeLine, Start: image.Pt(1, 1), End: image.Pt(58, 38), Width: 3, Color: ui.RGB(15, 169, 104)},
		{Type: ShapeArrow, Start: image.Pt(5, 30), End: image.Pt(40, 35), Width: 3, Color: ui.RGB(255, 197, 15)},
		{Type: ShapeBrush, Points: []image.Point{{X: 2, Y: 2}, {X: 10, Y: 30}, {X: 30, Y: 10}}, Width: 3, Color: ui.RGB(27, 27, 27)},
		{Type: ShapeMosaic, Blocks: []image.Point{{X: 0, Y: 0}}, Block: 16},
		{Type: ShapeText, Start: image.Pt(10, 10), Text: "Hi", FontSize: 14, Scale: 1, Color: ui.RGB(0, 0, 0)},
	}
	data, err := rasterize(shot, image.Rect(0, 0, 60, 40), shapes)
	if err != nil {
		t.Fatalf("rasterize: %v", err)
	}
	img, err := gopng.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解码 PNG: %v", err)
	}
	if got := img.Bounds(); got.Dx() != 60 || got.Dy() != 40 {
		t.Fatalf("PNG 尺寸 = %v, want 60x40", got)
	}
}

func TestRasterizeNonZeroOrigin(t *testing.T) {
	// 截图带非零原点（模拟副屏）：裁剪结果应规整到原点。
	shot := image.NewRGBA(image.Rect(100, 200, 220, 280))
	data, err := rasterize(shot, image.Rect(120, 220, 180, 260), nil)
	if err != nil {
		t.Fatalf("rasterize: %v", err)
	}
	img, err := gopng.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解码 PNG: %v", err)
	}
	if got := img.Bounds(); got.Dx() != 60 || got.Dy() != 40 {
		t.Fatalf("PNG 尺寸 = %v, want 60x40", got)
	}
}

func TestClampf(t *testing.T) {
	tests := []struct {
		v, lo, hi, want float32
	}{
		{5, 0, 10, 5},
		{-1, 0, 10, 0},
		{11, 0, 10, 10},
		{5, 10, 0, 10}, // hi < lo 时取 lo。
	}
	for _, tt := range tests {
		if got := clampf(tt.v, tt.lo, tt.hi); got != tt.want {
			t.Fatalf("clampf(%v, %v, %v) = %v, want %v", tt.v, tt.lo, tt.hi, got, tt.want)
		}
	}
}
