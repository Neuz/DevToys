package shell

import (
	"github.com/Cyenoch/mygoui"
	"github.com/Cyenoch/mygoui/layout"
	"github.com/Cyenoch/mygoui/theme"
	"github.com/egoist/mygo/ui"
)

// titleBarHeight 是创建窗口时为原生标题栏预留的高度（DIP）。
const titleBarHeight float32 = 44

// titleBarOptions 配置标题栏。
type titleBarOptions struct {
	Icon     *ui.SVG
	Title    string
	Subtitle string
	// Trailing 是标题右侧的插槽，用于放操作按钮；nil 时留空。
	Trailing func(c *ui.Context)
}

// titleBar 用 mygoui 的标准标题栏绘制右侧内容区顶部：高度取 titleBarHeight
// 与原生带的较大值，系统窗口控件的让位、空间不足时的堆叠都由 layout.TitleBar 处理。
// 侧栏占起始侧，所以只让开结束侧的系统窗口按钮。
func (s *Shell) titleBar(c *ui.Context, o titleBarOptions) {
	var leading func()
	if o.Icon != nil {
		leading = func() {
			size := theme.Scale(c, theme.IconLG)
			ui.Icon(c, o.Icon).Size(size, size).Shrink(0).TextColor(theme.Colors(c).MutedForeground)
		}
	}
	var trailing func()
	if o.Trailing != nil {
		trailing = func() { o.Trailing(c) }
	}
	layout.TitleBar(c, layout.TitleBarOptions{
		Span:       layout.SpanEnd,
		MinHeight:  titleBarHeight,
		Borderless: true,
		Leading:    leading,
		Center: func() {
			ui.Column(c).MinWidth(0).Children(func() {
				mygoui.Typography(c, o.Title, mygoui.TypographyOptions{
					Type:     theme.TypeBodySM,
					Weight:   theme.WeightMedium,
					Truncate: true,
				})
				if o.Subtitle != "" && fitsSubtitle(c) {
					mygoui.Typography(c, o.Subtitle, mygoui.TypographyOptions{
						Type:     theme.TypeBodyXS,
						Muted:    true,
						Truncate: true,
					})
				}
			})
		},
		Trailing: trailing,
	})
}

// fitsSubtitle 报告标题栏是否高到能同时放下标题与副标题两行。
// 高度按 layout.TitleBar 的规则估算：原生带（含 MinHeight）与按钮行高取大；
// 两行文字的高度取自语义排版令牌，跟随文字缩放一起变大。
func fitsSubtitle(c *ui.Context) bool {
	height := max(titleBarHeight/c.Zoom(), c.TitleBar().Height)
	height = max(height, theme.Scale(c, theme.HeightButtonSM)+2*theme.Scale(c, theme.Space(2)))
	lines := theme.TypeBodySM.Style().LineHeight + theme.TypeBodyXS.Style().LineHeight
	return height >= theme.Scale(c, lines)
}
