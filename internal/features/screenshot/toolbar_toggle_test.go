package screenshot

import (
	"image"
	"testing"

	"github.com/Cyenoch/mygoui/theme"
	"github.com/Neuz/DevToys/internal/features/screenshot/locales"
	"github.com/egoist/mygo/ui"
)

// newToolbarOverlay 构造带标注态字段的最小 overlay，供工具栏交互测试用。
func newToolbarOverlay() *overlay {
	ov := &overlay{
		shapes:     newShapeList(),
		tool:       toolNone,
		thick:      thicknessDef,
		fontSize:   fontSizeDef,
		mosaicMode: mosaicSmear,
		color:      swatchColors[0],
		textColor:  swatchColors[4],
		bounds:     image.Rect(0, 0, 1920, 1080),
		scale:      1,
		hover:      -1,
	}
	return ov
}

// buildToolbarForTest 在测试窗口里构建工具栏（buildToolbarFrame 的独立入口）。
func buildToolbarForTest(ov *overlay, c *ui.Context) {
	ov.buildToolbarFrame(c, ui.Rect{X: 0, Y: 0, W: 800, H: 600})
}

// TestToolbarBaselineClick 基线：toolNone 下点击矩形应切到矩形。
func TestToolbarBaselineClick(t *testing.T) {
	ov := newToolbarOverlay()
	tt := ui.NewTester(func(c *ui.Context) {
		theme.Apply(c)
		buildToolbarForTest(ov, c)
	}, 800, 600)

	if err := tt.Click(locales.T("toolbar_rect")); err != nil {
		t.Fatalf("点击矩形按钮: %v", err)
	}
	if ov.tool != toolRect {
		t.Fatalf("点击矩形后 tool = %d, want %d", ov.tool, toolRect)
	}
}

// TestToolbarSwitchFromText 复现：文本工具下点击其他工具按钮应能切走。
func TestToolbarSwitchFromText(t *testing.T) {
	ov := newToolbarOverlay()
	ov.selectTool(toolText)
	tt := ui.NewTester(func(c *ui.Context) {
		theme.Apply(c)
		buildToolbarForTest(ov, c)
	}, 800, 600)

	if err := tt.Click(locales.T("toolbar_rect")); err != nil {
		t.Fatalf("点击矩形按钮: %v", err)
	}
	if ov.tool != toolRect {
		t.Fatalf("文本下点击矩形后 tool = %d, want %d", ov.tool, toolRect)
	}
}

// TestToolbarRetoggleDeselects 复现：重复点击已激活工具应回到 toolNone。
func TestToolbarRetoggleDeselects(t *testing.T) {
	ov := newToolbarOverlay()
	ov.selectTool(toolRect)
	tt := ui.NewTester(func(c *ui.Context) {
		theme.Apply(c)
		buildToolbarForTest(ov, c)
	}, 800, 600)

	if err := tt.Click(locales.T("toolbar_rect")); err != nil {
		t.Fatalf("再次点击矩形按钮: %v", err)
	}
	if ov.tool != toolNone {
		t.Fatalf("重复点击矩形后 tool = %d, want %d", ov.tool, toolNone)
	}
}

// TestToolbarTextRetoggleDeselects 复现：文本工具激活时点击自身应退出文本模式。
func TestToolbarTextRetoggleDeselects(t *testing.T) {
	ov := newToolbarOverlay()
	ov.selectTool(toolText)
	ov.textEditing = true
	tt := ui.NewTester(func(c *ui.Context) {
		theme.Apply(c)
		buildToolbarForTest(ov, c)
	}, 800, 600)

	if err := tt.Click(locales.T("toolbar_text")); err != nil {
		t.Fatalf("再次点击文本按钮: %v", err)
	}
	if ov.tool != toolNone || ov.textEditing {
		t.Fatalf("重复点击文本后 tool = %d, textEditing = %v, want toolNone + 退出编辑", ov.tool, ov.textEditing)
	}
}
