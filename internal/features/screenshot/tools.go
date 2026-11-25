package screenshot

import "github.com/egoist/mygo/ui"

// 标注工具。toolNone 表示未选中任何工具（确认选区后的默认态）。
const (
	toolNone = -1
	toolRect = iota
	toolEllipse
	toolLine
	toolArrow
	toolBrush
	toolMosaic
	toolText
)

// 马赛克模式。
const (
	mosaicSmear = iota // 沿笔迹涂抹
	mosaicBox          // 拖矩形整块打码（默认模式）
)

// colorCount 是色板颜色数。
const colorCount = 6

// swatchColors 是预设标注颜色（红黄蓝绿黑白）。
var swatchColors = [colorCount]ui.Color{
	ui.RGB(0xE8, 0x11, 0x2D),
	ui.RGB(0xFF, 0xC5, 0x0F),
	ui.RGB(0x2D, 0x7D, 0xF6),
	ui.RGB(0x0F, 0xA9, 0x68),
	ui.RGB(0x1B, 0x1B, 0x1B),
	ui.RGB(0xFF, 0xFF, 0xFF),
}

// 线宽滑杆范围（DIP）；物理像素 = 值 × scale。
const (
	thicknessMin = 1.0
	thicknessMax = 12.0
	thicknessDef = 3.0
)

// 字号滑杆范围（DIP）。
const (
	fontSizeMin = 1.0
	fontSizeMax = 48.0
	fontSizeDef = 18.0
)

// mosaicBlockSize 是马赛克块尺寸（物理像素，乘 scale 前）。
const mosaicBlockSize = 16

// needThickness 报告工具是否需要线宽滑杆（马赛克涂抹用它控制笔刷块半径）。
func needThickness(t int) bool {
	switch t {
	case toolRect, toolEllipse, toolLine, toolArrow, toolBrush, toolMosaic:
		return true
	}
	return false
}

// needColor 报告工具是否需要颜色选择（文本用独立的字色 / 填充色选择）。
func needColor(t int) bool {
	switch t {
	case toolRect, toolEllipse, toolLine, toolArrow, toolBrush:
		return true
	}
	return false
}
