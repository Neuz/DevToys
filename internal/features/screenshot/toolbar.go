package screenshot

import (
	"fmt"

	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Cyenoch/mygoui"
	"github.com/Cyenoch/mygoui/color"
	"github.com/Cyenoch/mygoui/form"
	"github.com/Cyenoch/mygoui/navigation"
	"github.com/Cyenoch/mygoui/theme"
	"github.com/Neuz/DevToys/internal/features/screenshot/locales"
	"github.com/egoist/mygo/ui"
)

// toolbarEstWidth / toolbarEstHeight 是工具栏估算尺寸（DIP）：绝对定位时用于防越界，
// 实际矩形由绘制回调捕获。
// sliderWidth 是子工具栏所有 Slider（粗细/字号/马赛克）的统一宽度（DIP）。
// valueMinWidth 是粗细/字号当前值文字的最小宽度（DIP，两位数），避免 9↔10
// 跳变时行宽变化导致整行右移。
// subBarHeight 是子工具栏行的统一最小高度（与 theme.ButtonSmall 按钮同高），
// 保证马赛克等不同子工具栏高度一致。
const (
	toolbarEstWidth  = 560.0
	toolbarEstHeight = 100.0
	sliderWidth      = 100.0
	valueMinWidth    = 16.0
	subBarHeight     = 28.0
)

// buildToolbarFrame 在选区下方浮动真实组件工具栏。
// 外层 frame 全屏 PassThrough，内部用 padding 把工具栏推到目标位置；
// 工具栏矩形通过绘制回调捕获，供画布排除工具栏上的按压。
func (ov *overlay) buildToolbarFrame(c *ui.Context, screen ui.Rect) {
	tx, ty := ov.toolbarOffset(screen)
	frame := ui.Column(c).Key("toolbar-frame").Absolute().
		Left(0).Top(0).Right(0).Bottom(0).
		PaddingStart(tx).PaddingY(ty).
		Justify(ui.Start).AlignItems(ui.Start).PassThrough()
	frame.Children(func() {
		ui.Column(c).Key("toolbar-wrap").Gap(theme.Space(2)).Children(func() {
			ov.buildMainBar(c)
			ov.buildSubBar(c)
		})
	})
}

// toolbarOffset 计算工具栏位置：选区下方居中，越界时翻转到上方或移入屏内。
func (ov *overlay) toolbarOffset(screen ui.Rect) (float32, float32) {
	sr := ov.physRectToDip(ov.region)
	x := sr.X + sr.W/2 - toolbarEstWidth/2
	y := sr.Y + sr.H + 8
	if y+toolbarEstHeight > screen.Y+screen.H {
		y = sr.Y - toolbarEstHeight - 8
	}
	if y < screen.Y {
		y = sr.Y + screen.H - toolbarEstHeight - 8
	}
	x = clampf(x, screen.X+8, screen.X+screen.W-toolbarEstWidth-8)
	return x, y
}

// buildMainBar 是主工具栏：全部工具 + 撤销/重做 + 保存/完成/取消。
// 允许重复代码：每个按钮直接构造，不做封装。
func (ov *overlay) buildMainBar(c *ui.Context) {
	navigation.Toolbar(c, navigation.ToolbarOptions{
		Orientation: mygoui.Horizontal,
		Attached:    true,
		Label:       locales.T("toolbar_label"),
		Items: func() {
			// 每个按钮独立的选中变量：闭包持有 &selXxx，若共用一个变量，
			// 点击时读到的是构建闭包末次赋值（≠ 该按钮构建时的值），导致切换失效。
			// off 回调回到 toolNone：重复点击已激活工具即取消选择。
			selRect := ov.tool == toolRect
			mygoui.ToggleButton(c, mygoui.ToggleButtonOptions{
				Label:    locales.T("toolbar_rect"),
				Icon:     lucide.Square(),
				IconOnly: true,
				Variant:  mygoui.ToggleGhost,
				Selected: &selRect,
				Tooltip:  locales.T("toolbar_rect"),
				OnChange: func(on bool) {
					if on {
						ov.selectTool(toolRect)
					} else {
						ov.selectTool(toolNone)
					}
				},
			})
			selEllipse := ov.tool == toolEllipse
			mygoui.ToggleButton(c, mygoui.ToggleButtonOptions{
				Label:    locales.T("toolbar_ellipse"),
				Icon:     lucide.Circle(),
				IconOnly: true,
				Variant:  mygoui.ToggleGhost,
				Selected: &selEllipse,
				Tooltip:  locales.T("toolbar_ellipse"),
				OnChange: func(on bool) {
					if on {
						ov.selectTool(toolEllipse)
					} else {
						ov.selectTool(toolNone)
					}
				},
			})
			selLine := ov.tool == toolLine
			mygoui.ToggleButton(c, mygoui.ToggleButtonOptions{
				Label:    locales.T("toolbar_line"),
				Icon:     lucide.Slash(),
				IconOnly: true,
				Variant:  mygoui.ToggleGhost,
				Selected: &selLine,
				Tooltip:  locales.T("toolbar_line"),
				OnChange: func(on bool) {
					if on {
						ov.selectTool(toolLine)
					} else {
						ov.selectTool(toolNone)
					}
				},
			})
			selArrow := ov.tool == toolArrow
			mygoui.ToggleButton(c, mygoui.ToggleButtonOptions{
				Label:    locales.T("toolbar_arrow"),
				Icon:     lucide.ArrowUpRight(),
				IconOnly: true,
				Variant:  mygoui.ToggleGhost,
				Selected: &selArrow,
				Tooltip:  locales.T("toolbar_arrow"),
				OnChange: func(on bool) {
					if on {
						ov.selectTool(toolArrow)
					} else {
						ov.selectTool(toolNone)
					}
				},
			})
			selBrush := ov.tool == toolBrush
			mygoui.ToggleButton(c, mygoui.ToggleButtonOptions{
				Label:    locales.T("toolbar_brush"),
				Icon:     lucide.Brush(),
				IconOnly: true,
				Variant:  mygoui.ToggleGhost,
				Selected: &selBrush,
				Tooltip:  locales.T("toolbar_brush"),
				OnChange: func(on bool) {
					if on {
						ov.selectTool(toolBrush)
					} else {
						ov.selectTool(toolNone)
					}
				},
			})
			selMosaic := ov.tool == toolMosaic
			mygoui.ToggleButton(c, mygoui.ToggleButtonOptions{
				Label:    locales.T("toolbar_mosaic"),
				Icon:     lucide.Grid2x2(),
				IconOnly: true,
				Variant:  mygoui.ToggleGhost,
				Selected: &selMosaic,
				Tooltip:  locales.T("toolbar_mosaic"),
				OnChange: func(on bool) {
					if on {
						ov.selectTool(toolMosaic)
					} else {
						ov.selectTool(toolNone)
					}
				},
			})
			selText := ov.tool == toolText
			mygoui.ToggleButton(c, mygoui.ToggleButtonOptions{
				Label:    locales.T("toolbar_text"),
				Icon:     lucide.Type(),
				IconOnly: true,
				Variant:  mygoui.ToggleGhost,
				Selected: &selText,
				Tooltip:  locales.T("toolbar_text"),
				OnChange: func(on bool) {
					if on {
						ov.selectTool(toolText)
					} else {
						ov.selectTool(toolNone)
					}
				},
			})
			navigation.ToolbarSeparator(c, mygoui.Horizontal)
			mygoui.Button(c, mygoui.ButtonOptions{
				Label:    locales.T("toolbar_undo"),
				Icon:     lucide.Undo2(),
				IconOnly: true,
				Variant:  mygoui.ButtonGhost,
				Tooltip:  locales.T("toolbar_undo"),
				OnClick:  func() { ov.shapes.pop() },
			})
			mygoui.Button(c, mygoui.ButtonOptions{
				Label:    locales.T("toolbar_redo"),
				Icon:     lucide.Redo2(),
				IconOnly: true,
				Variant:  mygoui.ButtonGhost,
				Tooltip:  locales.T("toolbar_redo"),
				OnClick:  func() { ov.shapes.pushRedo() },
			})
			navigation.ToolbarSeparator(c, mygoui.Horizontal)
			mygoui.Button(c, mygoui.ButtonOptions{
				Label:    locales.T("toolbar_save"),
				Icon:     lucide.Save(),
				IconOnly: true,
				Variant:  mygoui.ButtonGhost,
				Tooltip:  locales.T("toolbar_save"),
				OnClick:  func() { ov.module.saveBy(ov) },
			})
			mygoui.Button(c, mygoui.ButtonOptions{
				Label:    locales.T("toolbar_finish"),
				Icon:     lucide.Check(),
				IconOnly: true,
				Variant:  mygoui.ButtonGhost,
				Tooltip:  locales.T("toolbar_finish"),
				OnClick:  func() { ov.module.finish(ov) },
			})
			mygoui.Button(c, mygoui.ButtonOptions{
				Label:    locales.T("toolbar_cancel"),
				Icon:     lucide.X(),
				IconOnly: true,
				Variant:  mygoui.ButtonGhost,
				Tooltip:  locales.T("toolbar_cancel"),
				OnClick:  func() { ov.module.cancel() },
			})
		},
	})
}

// buildSubBar 是子工具栏：按当前工具显示线宽 / 颜色 / 字号等属性。
// 高度与主工具栏一致（Attached Toolbar 同款），控件单行排布：
// 粗细 (slider) 粗细值 | ColorSwatchPicker，宽度随控件自然变化。
func (ov *overlay) buildSubBar(c *ui.Context) {
	if ov.tool == toolNone {
		return
	}
	// 工具栏底色：Attached 默认是 Card（浅色主题近纯白），而色板未选中项
	// 边框透明，白色色块会隐形。改用比 Card 更深的中性灰（叠加 Card 保证
	// 不透明，深色主题同样成立），让所有色块都有对比。
	t := theme.Colors(c)
	navigation.Toolbar(c, navigation.ToolbarOptions{
		Orientation: mygoui.Horizontal,
		Attached:    true,
		Label:       locales.T("toolbar_props"),
		Items: func() {
			// 滑杆变化需要立即重绘（粗细值文字 / 预览），闭包捕获 Context。
			invalidate := func() { c.Invalidate() }
			// 统一子工具栏行高：所有工具的属性行最小高度一致。
			// PaddingStart 让最左侧文本不贴边。
			ui.Row(c).Gap(theme.Space(3)).AlignItems(ui.Center).
				PaddingStart(theme.Space(2)).
				MinHeight(theme.Scale(c, subBarHeight)).Children(func() {
				if needThickness(ov.tool) {
					// Shrink(0)+SingleLine 防止 Row 压缩文本宽度导致竖排。
					mygoui.Typography(c, locales.T("toolbar_thickness"), mygoui.TypographyOptions{Type: theme.TypeBodyXS}).Shrink(0).SingleLine()
					form.Slider(c, form.SliderOptions{
						State:      form.SliderState{Value: &ov.thick},
						Min:        thicknessMin,
						Max:        thicknessMax,
						Step:       1,
						HideOutput: true,
						OnChange:   invalidate,
						// Shrink(0) 抵消 Slider 根元素自带的 FillWidth（可伸缩），
						// 防止被右侧色板/按钮挤压，宽度固定为 sliderWidth。
					}).Width(theme.Scale(c, sliderWidth)).Shrink(0)
					// 粗细当前值：固定最小宽度（两位数），避免 9↔10 跳变时整行右移。
					val := fmt.Sprintf("%d", int(ov.thick))
					mygoui.Typography(c, val, mygoui.TypographyOptions{Type: theme.TypeBodyXS, Muted: true}).
						Shrink(0).SingleLine().MinWidth(theme.Scale(c, valueMinWidth))
					navigation.ToolbarSeparator(c, mygoui.Horizontal)
				}
				if ov.tool == toolMosaic {
					// 马赛克模式切换（单选段，图标按钮，ButtonSmall 与子栏行高一致）。
					// 独立选中变量：原理同主工具栏，共享变量会让点击读到末次赋值。
					selSmear := ov.mosaicMode == mosaicSmear
					mygoui.ToggleButton(c, mygoui.ToggleButtonOptions{
						Label:    locales.T("mosaic_smear"),
						Icon:     lucide.Brush(),
						IconOnly: true,
						Variant:  mygoui.ToggleGhost,
						Size:     theme.ButtonSmall,
						Selected: &selSmear,
						Tooltip:  locales.T("mosaic_smear"),
						OnChange: func(on bool) {
							if on {
								ov.mosaicMode = mosaicSmear
							}
						},
					})
					selBox := ov.mosaicMode == mosaicBox
					mygoui.ToggleButton(c, mygoui.ToggleButtonOptions{
						Label:    locales.T("mosaic_box"),
						Icon:     lucide.SquareDashedX(),
						IconOnly: true,
						Variant:  mygoui.ToggleGhost,
						Size:     theme.ButtonSmall,
						Selected: &selBox,
						Tooltip:  locales.T("mosaic_box"),
						OnChange: func(on bool) {
							if on {
								ov.mosaicMode = mosaicBox
							}
						},
					})
				}
				if ov.tool == toolText {
					mygoui.Typography(c, locales.T("toolbar_font_size"), mygoui.TypographyOptions{Type: theme.TypeBodyXS}).Shrink(0).SingleLine()
					form.Slider(c, form.SliderOptions{
						State:      form.SliderState{Value: &ov.fontSize},
						Min:        fontSizeMin,
						Max:        fontSizeMax,
						Step:       1,
						HideOutput: true,
						OnChange:   invalidate,
					}).Width(theme.Scale(c, sliderWidth)).Shrink(0)
					// 字号当前值：固定最小宽度（两位数），避免跳变时整行右移。
					val := fmt.Sprintf("%d", int(ov.fontSize))
					mygoui.Typography(c, val, mygoui.TypographyOptions{Type: theme.TypeBodyXS, Muted: true}).
						Shrink(0).SingleLine().MinWidth(theme.Scale(c, valueMinWidth))
					navigation.ToolbarSeparator(c, mygoui.Horizontal)
					// 字色。
					color.ColorSwatchPicker(c, color.ColorSwatchPickerOptions{
						Label:  locales.T("toolbar_color"),
						Colors: swatchColors[:],
						Value:  &ov.textColor,
						Size:   color.SwatchXS,
					})
					return
				}
				if needColor(ov.tool) {
					color.ColorSwatchPicker(c, color.ColorSwatchPickerOptions{
						Label:  locales.T("toolbar_color"),
						Colors: swatchColors[:],
						Value:  &ov.color,
						Size:   color.SwatchXS,
					})
				}
			})
		},
	}).Background(t.Input.Over(t.Card))
}

// buildTextEditor 是就地文本编辑（参考 ui.EditableText 的编辑态样式）：
// 主题背景 + 强调色描边 + 正常文字色，字号跟随滑杆预览最终效果；
// Enter 确认，Esc / 右键 / 点框外取消或提交。
func (ov *overlay) buildTextEditor(c *ui.Context) {
	p := ov.physPointToDip(ov.textAt)
	t := theme.Colors(c)
	pad := theme.Space(0.5)
	// TextInput 不接受 Key（构建期即初始化输入状态，Key 会 panic）；
	// Draw 会顶掉原生文字绘制，捕获矩形只能用 DrawOver。
	input := ui.TextInputBase(c, &ov.textValue).
		Absolute().Left(p.X).Top(p.Y).
		Label(locales.T("toolbar_text")).
		TextColor(t.Foreground).
		FontSize(theme.Scale(c, float32(ov.fontSize))).
		MinWidth(theme.Scale(c, 40)).
		Padding(pad, pad*2).
		Radius(theme.Space(1)).
		Background(t.Background).
		Border(1, t.Accent).
		DrawOver(func(pp *ui.Painter, r ui.Rect) { ov.textBoxRect = r })
	if ov.textFresh {
		ov.textFresh = false
		input.Focus()
	}
	if input.Submitted() {
		ov.commitText()
	}
}
