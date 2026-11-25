package screenshot

import (
	"image"

	"github.com/egoist/mygo/ui"
)

// 本文件是覆盖层的交互状态机：指针事件记录（handleInput）与每帧推进
// （handleSelectPointer / handleAnnotatePointer）。几何调整见 adjust.go，
// 绘制见 paint.go。

// handleInput 是画布的输入处理器：记录指针位置与按键状态，供下一帧的状态机
// 读取。返回 true 表示事件在此终结，不再冒泡。
func (ov *overlay) handleInput(ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputPointerDown:
		ov.pointerDIPX, ov.pointerDIPY = ev.X, ev.Y
		ov.pointerHas = true
		switch ev.Button {
		case 1: // 右键：文本编辑中退出编辑；否则取消整个截图。
			if ov.textEditing {
				ov.textEditing = false
				ov.textValue = ""
				ov.shapes.discard()
			} else {
				ov.module.cancel()
			}
		case 0:
			ov.pressed = true
		}
		return true
	case ui.InputPointerMove:
		ov.pointerDIPX, ov.pointerDIPY = ev.X, ev.Y
		ov.pointerHas = true
		return true
	case ui.InputPointerUp:
		ov.pointerDIPX, ov.pointerDIPY = ev.X, ev.Y
		if ev.Button == 0 {
			ov.pressed = false
		}
		return true
	}
	return false
}

// handleSelectPointer 处理选择态：按下即开始框选，松开时位移够大确认矩形，
// 否则按点击处理（点选悬停的窗口）。
func (ov *overlay) handleSelectPointer(cur image.Point) {
	if ov.dragging {
		ov.dragCur = cur
		if !ov.pressed {
			ov.dragging = false
			if region, ok := ov.pickRegion(); ok && !region.Empty() {
				ov.confirmRegion(region)
				return
			}
			// 几乎没拖动：视为点击，选中按下时悬停的窗口。
			if ov.hover >= 0 && ov.hover < len(ov.windows) {
				ov.confirmRegion(ov.windows[ov.hover].bounds.Intersect(ov.bounds))
			}
		}
		return
	}

	ov.hover = hitWindow(ov.windows, cur)
	if ov.pressed {
		ov.dragging = true
		ov.pressAt = cur
		ov.dragCur = cur
	}
}

// pickRegion 返回当前拖拽围成的矩形（与本显示器取交集），
// 位移超过 minDragPixels 才算有效选区。
func (ov *overlay) pickRegion() (image.Rectangle, bool) {
	r := normRect(ov.pressAt, ov.dragCur).Intersect(ov.bounds)
	return r, r.Dx() > minDragPixels && r.Dy() > minDragPixels
}

// confirmRegion 进入标注态；默认不激活任何标注工具。
func (ov *overlay) confirmRegion(region image.Rectangle) {
	ov.region = region
	ov.state = stateAnnotate
	ov.tool = toolNone
	ov.drawing = false
	ov.textEditing = false
	ov.resizeHandle = -1
	ov.selectedID = -1
	ov.moving = false
	ov.regionDrag = false
}

// selectTool 切换标注工具，中断进行中的绘制 / 文本输入 / 选择移动。
func (ov *overlay) selectTool(tool int) {
	ov.tool = tool
	ov.drawing = false
	ov.shapes.discard()
	ov.selectedID = -1
	ov.moving = false
	if ov.textEditing {
		ov.textEditing = false
		ov.textValue = ""
	}
}

// pointerOnUI 报告物理像素点是否落在工具栏或文本输入框上。
func (ov *overlay) pointerOnUI(cur image.Point) bool {
	p := ov.physPointToDip(cur)
	if ov.toolbarRectBox.W > 0 && ov.toolbarRectBox.Contains(p.X, p.Y) {
		return true
	}
	if ov.textEditing && ov.textBoxRect.W > 0 && ov.textBoxRect.Contains(p.X, p.Y) {
		return true
	}
	return false
}

// handleAnnotatePointer 处理标注态的三段式鼠标逻辑：
// MouseDown 创建临时 Shape 推入列表 → MouseMove 更新（预览）→ MouseUp 固化。
func (ov *overlay) handleAnnotatePointer(cur image.Point) {
	// 工具栏 / 输入框上的按压归组件处理；文本编辑中点击框外先提交。
	if ov.textEditing && ov.pressed && !ov.pointerOnUI(cur) {
		ov.commitText()
		return
	}
	// 选区调整拖拽中（优先于一切，指针经过工具栏也不中断）。
	if ov.resizeHandle >= 0 {
		ov.applyResize(cur)
		if !ov.pressed {
			ov.resizeHandle = -1
		}
		return
	}
	// Shape 移动拖拽中。
	if ov.moving {
		ov.applyMove(cur)
		if !ov.pressed {
			ov.moving = false
		}
		return
	}
	// 选区整体平移中。
	if ov.regionDrag {
		ov.applyRegionMove(cur)
		if !ov.pressed {
			ov.regionDrag = false
		}
		return
	}
	if ov.pointerOnUI(cur) && !ov.drawing {
		return
	}
	inside := pointInRect(cur, ov.region)

	// 按下调整手柄：优先于工具绘制 / 选择。
	if ov.pressed {
		if h := ov.hitResizeHandle(cur); h >= 0 {
			ov.resizeHandle = h
			return
		}
		// toolNone：点选已固化 Shape 进入移动；点选区空白拖动平移选区（并取消选中）。
		if ov.tool == toolNone {
			if sh, ok := ov.hitShape(cur); ok {
				ov.selectedID = sh.ID
				ov.moving = true
				ov.moveStart = cur
				ov.moveOrig = sh
			} else {
				ov.selectedID = -1
				ov.regionDrag = true
				ov.regionStart = cur
				ov.regionOrig = ov.region
			}
			return
		}
	}

	switch ov.tool {
	case toolRect, toolEllipse, toolLine, toolArrow:
		if ov.drawing {
			// MouseMove：更新 End 触发重绘（预览态）。
			ov.shapes.update(ov.clampToRegion(cur))
			if !ov.pressed {
				ov.drawing = false
				ov.shapes.commit(minDragPixels)
			}
			return
		}
		if !ov.pressed || !inside {
			return
		}
		// MouseDown：创建临时 Shape 并标记正在绘制。
		ov.shapes.begin(ov.shapeType(), cur, ov.color, ov.physThick(), ov.scale)
		ov.drawing = true
	case toolBrush:
		if ov.drawing {
			ov.shapes.appendPoint(ov.clampToRegion(cur))
			if !ov.pressed {
				ov.drawing = false
				ov.shapes.commit(minDragPixels)
			}
			return
		}
		if !ov.pressed || !inside {
			return
		}
		ov.shapes.begin(ShapeBrush, cur, ov.color, ov.physThick(), ov.scale)
		ov.shapes.appendPoint(cur)
		ov.drawing = true
	case toolMosaic:
		switch ov.mosaicMode {
		case mosaicSmear:
			if ov.drawing {
				ov.smearTo(cur)
				if !ov.pressed {
					ov.drawing = false
					ov.shapes.commit(minDragPixels)
				}
				return
			}
			if !ov.pressed || !inside {
				return
			}
			ov.shapes.begin(ShapeMosaic, cur, ov.color, ov.physThick(), ov.scale)
			ov.shapes.setMosaic(nil, ov.mosaicBlockSize())
			ov.drawing = true
			ov.smearTo(cur)
		case mosaicBox:
			if ov.drawing {
				ov.shapes.update(ov.clampToRegion(cur))
				if !ov.pressed {
					// 松开：位移过小视为误触；否则把框选矩形换算成整块马赛再固化。
					if sh, ok := ov.shapes.drawingShape(); ok {
						dmax := max(abs(sh.End.X-sh.Start.X), abs(sh.End.Y-sh.Start.Y))
						if dmax <= minDragPixels {
							ov.shapes.discard()
						} else {
							ov.shapes.setMosaic(boxBlocks(sh.boundsRect(), ov.bounds.Min, ov.mosaicBlockSize()), sh.Block)
							ov.shapes.markBox()
							ov.shapes.commit(minDragPixels)
						}
					}
					ov.drawing = false
				}
				return
			}
			if !ov.pressed || !inside {
				return
			}
			ov.shapes.begin(ShapeMosaic, cur, ov.color, ov.physThick(), ov.scale)
			ov.shapes.setMosaic(nil, ov.mosaicBlockSize())
			ov.drawing = true
		}
	case toolText:
		if ov.pressed && inside && !ov.textEditing {
			// MouseDown：锚定文本输入位置。
			ov.textAt = cur
			ov.textEditing = true
			ov.textFresh = true
			ov.textValue = ""
			ov.shapes.begin(ShapeText, cur, ov.textColor, 0, ov.scale)
		}
	}
}

// shapeType 把形状类工具映射为 Shape 类别。
func (ov *overlay) shapeType() ShapeType {
	switch ov.tool {
	case toolRect:
		return ShapeRect
	case toolEllipse:
		return ShapeEllipse
	case toolLine:
		return ShapeLine
	case toolArrow:
		return ShapeArrow
	default:
		return ShapeBrush
	}
}

// mosaicBlockSize 是马赛克块尺寸（物理像素，随缩放走）。
func (ov *overlay) mosaicBlockSize() int {
	return max(int(mosaicBlockSize*ov.scale), 8)
}

// smearTo 把指针位置追加进涂抹马赛克路径并重算块列表。
func (ov *overlay) smearTo(cur image.Point) {
	sh, ok := ov.shapes.drawingShape()
	if !ok || len(sh.Points) == 0 {
		ov.shapes.appendPoint(cur)
	} else {
		last := sh.Points[len(sh.Points)-1]
		// 距离过近不追加点，减少重算。
		if max(abs(cur.X-last.X), abs(cur.Y-last.Y)) < 4 {
			return
		}
		ov.shapes.appendPoint(cur)
	}
	sh, _ = ov.shapes.drawingShape()
	// 粗细滑杆（1–12 DIP）映射为笔刷半径 0.5–3 块，块内圆判支持小数。
	radius := max(ov.thick/4, 0.5)
	ov.shapes.setMosaic(smearBlocks(sh.Points, ov.bounds.Min, sh.Block, radius), sh.Block)
}

// clampToRegion 把指针钳制在选区内，避免拖出边界画出越界图形。
func (ov *overlay) clampToRegion(cur image.Point) image.Point {
	return image.Point{
		X: min(max(cur.X, ov.region.Min.X), ov.region.Max.X-1),
		Y: min(max(cur.Y, ov.region.Min.Y), ov.region.Max.Y-1),
	}
}

// commitText 提交正在输入的文本（空文本丢弃）。
func (ov *overlay) commitText() {
	if !ov.textEditing {
		return
	}
	ov.shapes.setText(ov.textValue, ov.fontSize)
	ov.textEditing = false
	ov.textValue = ""
	ov.shapes.commit(minDragPixels)
}
