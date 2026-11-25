package screenshot

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// newAdjustOverlay 构造标注态最小 overlay：物理 1920x1080、scale 1、选区 (100,100)-(500,400)。
func newAdjustOverlay(handle int) *overlay {
	return &overlay{
		bounds:       image.Rect(0, 0, 1920, 1080),
		scale:        1,
		region:       image.Rect(100, 100, 500, 400),
		resizeHandle: handle,
		shapes:       newShapeList(),
	}
}

func TestApplyResizeCorner(t *testing.T) {
	ov := newAdjustOverlay(4) // 右下角
	ov.applyResize(image.Pt(600, 500))
	want := image.Rect(100, 100, 600, 500)
	if ov.region != want {
		t.Fatalf("右下拖拽 region = %v, want %v", ov.region, want)
	}
}

func TestApplyResizeEdgeAndBounds(t *testing.T) {
	ov := newAdjustOverlay(7) // 左边缘
	ov.applyResize(image.Pt(450, 0))
	want := image.Rect(450, 100, 500, 400)
	if ov.region != want {
		t.Fatalf("左边缘拖拽 region = %v, want %v", ov.region, want)
	}
	// 越过对边：钉在最小边长。
	ov = newAdjustOverlay(3) // 右边缘
	ov.applyResize(image.Pt(110, 0))
	if ov.region.Dx() != minRegionPixels {
		t.Fatalf("右边缘过界 Dx = %d, want %d", ov.region.Dx(), minRegionPixels)
	}
	// 越出屏幕：钳到 bounds。
	ov = newAdjustOverlay(4)
	ov.applyResize(image.Pt(3000, 3000))
	if ov.region.Max != (image.Point{X: 1920, Y: 1080}) {
		t.Fatalf("越屏钳制 Max = %v", ov.region.Max)
	}
}

func TestHitResizeHandle(t *testing.T) {
	ov := newAdjustOverlay(-1)
	if h := ov.hitResizeHandle(image.Pt(100, 100)); h != 0 {
		t.Fatalf("左上角命中 %d, want 0", h)
	}
	if h := ov.hitResizeHandle(image.Pt(300, 400)); h != 5 {
		t.Fatalf("下边中点命中 %d, want 5", h)
	}
	if h := ov.hitResizeHandle(image.Pt(300, 250)); h != -1 {
		t.Fatalf("选区内部不应命中手柄, got %d", h)
	}
}

func TestShapeListTranslate(t *testing.T) {
	s := newShapeList()
	s.begin(ShapeRect, image.Pt(10, 10), ui.RGB(0, 0, 0), 3, 1)
	s.update(image.Pt(50, 50))
	if !s.commit(minDragPixels) {
		t.Fatal("commit 失败")
	}
	orig, ok := s.find(s.shapes[0].ID)
	if !ok {
		t.Fatal("find 失败")
	}
	if !s.translate(orig.ID, orig, 5, -3) {
		t.Fatal("translate 失败")
	}
	sh, _ := s.find(orig.ID)
	if sh.Start != (image.Point{X: 15, Y: 7}) || sh.End != (image.Point{X: 55, Y: 47}) {
		t.Fatalf("平移后 Start/End = %v/%v", sh.Start, sh.End)
	}
	// 从快照重算：再次 translate 同一 delta 结果一致（无累积）。
	if !s.translate(orig.ID, orig, 5, -3) {
		t.Fatal("translate 失败")
	}
	sh, _ = s.find(orig.ID)
	if sh.Start != (image.Point{X: 15, Y: 7}) {
		t.Fatalf("快照重算 Start = %v", sh.Start)
	}
}

func TestShapeListTranslateMosaic(t *testing.T) {
	s := newShapeList()
	s.begin(ShapeMosaic, image.Pt(0, 0), ui.RGB(0, 0, 0), 0, 1)
	s.setMosaic([]image.Point{{X: 1, Y: 1}}, 16)
	s.update(image.Pt(100, 100))
	if !s.commit(minDragPixels) {
		t.Fatal("commit 失败")
	}
	orig, _ := s.find(s.shapes[0].ID)
	// 20px 位移按块 16 量化为 1 格。
	if !s.translate(orig.ID, orig, 20, 0) {
		t.Fatal("translate 失败")
	}
	sh, _ := s.find(orig.ID)
	if len(sh.Blocks) != 1 || sh.Blocks[0] != (image.Point{X: 2, Y: 1}) {
		t.Fatalf("马赛克块平移结果 = %v", sh.Blocks)
	}
}

func TestRoundDiv(t *testing.T) {
	cases := []struct{ a, b, want int }{
		{20, 16, 1}, {10, 16, 1}, {7, 16, 0}, {-20, 16, -1}, {-7, 16, 0}, {0, 16, 0},
	}
	for _, c := range cases {
		if got := roundDiv(c.a, c.b); got != c.want {
			t.Errorf("roundDiv(%d,%d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestApplyRegionMove(t *testing.T) {
	ov := newAdjustOverlay(-1)
	ov.regionDrag = true
	ov.regionStart = image.Pt(300, 300)
	ov.regionOrig = ov.region
	ov.applyRegionMove(image.Pt(320, 280))
	want := image.Rect(120, 80, 520, 380)
	if ov.region != want {
		t.Fatalf("平移选区 = %v, want %v", ov.region, want)
	}
	// 越屏钳制：往右上大幅拖拽钉在屏幕内。
	ov.applyRegionMove(image.Pt(-5000, -5000))
	if ov.region.Min != (image.Point{X: 0, Y: 0}) {
		t.Fatalf("越屏钳制 Min = %v", ov.region.Min)
	}
}

func TestShapeBoundsHit(t *testing.T) {
	ov := newAdjustOverlay(-1)
	// 一条水平线：包围盒退化，靠容差命中。
	ov.shapes.begin(ShapeLine, image.Pt(200, 200), ui.RGB(0, 0, 0), 4, 1)
	ov.shapes.update(image.Pt(400, 200))
	ov.shapes.commit(minDragPixels)
	if _, ok := ov.hitShape(image.Pt(300, 202)); !ok {
		t.Fatal("线上应命中")
	}
	if _, ok := ov.hitShape(image.Pt(300, 230)); ok {
		t.Fatal("远离线不应命中")
	}
}
