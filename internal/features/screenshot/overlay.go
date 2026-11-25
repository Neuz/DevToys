package screenshot

import (
	"image"
	"math"

	"github.com/Cyenoch/mygoui/theme"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// overlay 是单个显示器的覆盖层窗口。状态流转：选择态（框选/点选窗口）
// → 标注态（工具栏 + Shape 编辑）→ 动作（完成/保存/取消）。
// 坐标约定：pressAt/dragCur/region 一律是虚拟桌面物理像素；
// 绘制时按 scale 换算成 DIP。
// 本文件只含状态结构体与生命周期（开/关/每帧入口）；
// 交互状态机在 input.go，几何调整在 adjust.go，绘制在 paint.go。
type overlay struct {
	module *Module
	window *mygo.Window
	shot   *image.RGBA
	// bitmap 持有 mygo 位图；首帧 Draw 时创建。
	bitmap *ui.Bitmap
	// bounds 是显示器在虚拟桌面中的物理像素矩形；shot 是同一矩形内的截图。
	bounds image.Rectangle
	scale  float64

	state    int // stateSelect / stateAnnotate
	dragging bool
	pressAt  image.Point
	dragCur  image.Point
	region   image.Rectangle
	windows  []windowRect
	hover    int // -1 无高亮

	// shapes 是全部标注对象（含尾部正在绘制的临时项）。
	shapes *shapeList
	// tool 是当前标注工具；thick/fontSize 是滑杆值（DIP）。
	tool       int
	thick      float64
	fontSize   float64
	mosaicMode int // mosaicSmear / mosaicBox
	// color 是形状/画笔色；textColor 是文本字色。
	color     ui.Color
	textColor ui.Color
	// drawing 指示 MouseDown 后正在绘制（临时 Shape 已在列表里）。
	drawing bool
	// textEditing 指示文本工具正在输入；textAt 是锚点；
	// textFresh 标记刚进入编辑（首帧需要聚焦输入）。
	textEditing bool
	textAt      image.Point
	textValue   string
	textFresh   bool
	// lastSmear 是涂抹马赛克最近一次路径长度，用于增量节流。
	smearPath int

	// resizeHandle 是正在拖动的选区调整手柄（-1 无）：
	// 0=左上 1=上 2=右上 3=右 4=右下 5=下 6=左下 7=左。
	resizeHandle int
	// selectedID 是 toolNone 状态下选中的 Shape ID（-1 无）；
	// moving 拖拽中，moveStart 为按下时物理点，moveOrig 为按下时几何快照。
	selectedID int
	moving     bool
	moveStart  image.Point
	moveOrig   Shape
	// regionDrag 拖拽平移整个选区中；regionStart 按下物理点，regionOrig 按下时选区快照。
	regionDrag  bool
	regionStart image.Point
	regionOrig  image.Rectangle

	// pointerDIPX/Y 是指针在画布内的 DIP 坐标（等于窗口内坐标），
	// 由 handleInput 在每帧构建前更新；pressed 是主键按住状态。
	pointerDIPX float32
	pointerDIPY float32
	pointerHas  bool
	pressed     bool
	// toolbarRectBox / textBoxRect 是工具栏与文本输入框的实际矩形，
	// 由各自包装元素的绘制回调捕获，用于把其上的按压排除在画布外。
	toolbarRectBox ui.Rect
	textBoxRect    ui.Rect
}

// 覆盖层状态。
const (
	stateSelect = iota
	stateAnnotate
)

// minRegionPixels 是选区调整后的最小边长（物理像素）。
const minRegionPixels = 20

// openOverlay 为每个显示器创建覆盖窗口并显示。
// 调用方保证抓屏已经完成（见 startCapture 的顺序约束）。
func (m *Module) openOverlay(shots []image.RGBA, displays []mygo.Display) {
	m.closeOverlays()
	for i := range shots {
		shot := shots[i]
		// ov.bounds 统一用虚拟桌面物理像素：与截图（kbinani）、窗口枚举
		//（GetWindowRect）、指针换算同一坐标系。
		bounds := shot.Bounds()
		scale := 1.0
		// 窗口的 X/Y/Width/Height 是 DIP，用 mygo 的显示器信息
		//（Displays() 的 Bounds 是 per-display 的 DIP）。
		var wx, wy, ww, wh int
		if d, s, ok := matchDisplay(i, bounds.Dx(), bounds.Dy(), displays); ok {
			b := d.Bounds
			wx, wy, ww, wh = b.X, b.Y, b.Width, b.Height
			scale = s
		} else {
			ww, wh = bounds.Dx(), bounds.Dy()
		}
		ov := &overlay{
			module: m,
			shot:   &shot,
			bounds: bounds,
			scale:  scale,
			hover:  -1,
			tool:   toolNone,
			thick:  thicknessDef,
			color:  swatchColors[0],

			resizeHandle: -1,
			selectedID:   -1,
		}
		ov.fontSize = fontSizeDef
		ov.mosaicMode = mosaicBox
		ov.textColor = swatchColors[4]
		ov.shapes = newShapeList()
		ov.window = mygo.NewWindow(mygo.WindowOptions{
			Title:         "screenshot-overlay",
			Panel:         true,
			Frameless:     true,
			AlwaysOnTop:   true,
			SkipTaskbar:   true,
			Hidden:        true,
			Width:         ww,
			Height:        wh,
			DisableResize: true,
			Content:       ui.View(ov.view),
		})
		ov.window.SetPosition(wx, wy)
		m.overlays = append(m.overlays, ov)
	}
	for _, ov := range m.overlays {
		ov.window.Show()
		ov.window.Focus()
	}
}

// matchDisplay 为物理尺寸 physW×physH 的截图找到对应的显示器：
// 按物理 = round(DIP × scale) 换算唯一匹配；kbinani 与 mygo 的显示器
// 顺序可能不同，不能直接按下标对应。找不到时回退下标 i。
func matchDisplay(i, physW, physH int, displays []mygo.Display) (mygo.Display, float64, bool) {
	for _, d := range displays {
		if d.Bounds.Width <= 0 || d.Bounds.Height <= 0 {
			continue
		}
		s := float64(physW) / float64(d.Bounds.Width)
		if s <= 0 {
			continue
		}
		if math.Round(float64(physW)/s) == float64(d.Bounds.Width) &&
			math.Round(float64(physH)/s) == float64(d.Bounds.Height) {
			return d, s, true
		}
	}
	if i < len(displays) {
		return displays[i], displays[i].ScaleFactor, true
	}
	return mygo.Display{}, 1.0, false
}

// closeOverlays 销毁全部覆盖窗口。
func (m *Module) closeOverlays() {
	for _, ov := range m.overlays {
		if ov.window != nil {
			ov.window.Destroy()
		}
	}
	m.overlays = nil
}

// activeOverlay 返回聚焦的覆盖层；没有则回退第一个（单显示器场景）。
func (m *Module) activeOverlay() *overlay {
	for _, ov := range m.overlays {
		if ov.window != nil && ov.window.IsFocused() {
			return ov
		}
	}
	if len(m.overlays) > 0 {
		return m.overlays[0]
	}
	return nil
}

// view 是覆盖窗口的全部内容：全屏自绘画布 + 浮动组件工具栏。
// 指针交互走 HandleInput（画布需要每一个指针事件，包括未按压的移动）；
// 输出快捷键：Enter 完成、Ctrl+S 保存、Esc 取消，都是窗口级。
func (ov *overlay) view(c *ui.Context) {
	theme.Apply(c)
	c.OnWindowShortcut(0, ui.KeyEscape, func() {
		switch {
		case ov.textEditing:
			// 文本输入中：丢弃输入，退出编辑。
			ov.textEditing = false
			ov.textValue = ""
			ov.shapes.discard()
		case ov.drawing:
			ov.drawing = false
			ov.shapes.discard()
		default:
			ov.module.cancel()
		}
	})
	c.OnWindowShortcut(0, ui.KeyEnter, func() {
		// 文本输入中 Enter 由 Input 的 OnSubmit 提交，这里不处理。
		if !ov.textEditing && ov.state == stateAnnotate {
			ov.module.finish(ov)
		}
	})
	c.OnWindowShortcut(ui.Ctrl, ui.KeyS, func() {
		if ov.state == stateAnnotate {
			ov.module.saveBy(ov)
		}
	})
	ui.Box(c).Key("overlay-root").Fill().Children(func() {
		// 画布：底图 / 遮罩 / 选区 / Shape 矢量预览。
		ui.Box(c).Key("overlay-canvas").Fill().Draw(func(p *ui.Painter, r ui.Rect) {
			ov.handleFrame()
			ov.paint(p, r)
		}).HandleInput(ov.handleInput)
		// 浮动工具栏（真实 mygoui 组件，画布之上）。
		if ov.state == stateAnnotate {
			ov.buildToolbarFrame(c, ui.Rect{X: 0, Y: 0, W: float32(ov.bounds.Dx()) / float32(ov.scale), H: float32(ov.bounds.Dy()) / float32(ov.scale)})
		}
		// 文本输入框。
		if ov.textEditing {
			ov.buildTextEditor(c)
		}
	})
}

// handleFrame 每帧开始时按最新指针状态推进状态机。
func (ov *overlay) handleFrame() {
	cur := ov.pointerPhys()
	if ov.state == stateSelect {
		ov.handleSelectPointer(cur)
		return
	}
	ov.handleAnnotatePointer(cur)
}

// pointerPhys 返回指针的物理像素坐标（虚拟桌面坐标，含显示器偏移）。
func (ov *overlay) pointerPhys() image.Point {
	if !ov.pointerHas {
		return image.Point{X: -1, Y: -1}
	}
	return image.Point{
		X: ov.bounds.Min.X + ov.dipToPhysical(ov.pointerDIPX),
		Y: ov.bounds.Min.Y + ov.dipToPhysical(ov.pointerDIPY),
	}
}
