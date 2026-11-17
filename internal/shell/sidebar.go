package shell

import (
	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Cyenoch/mygoui"
	"github.com/Cyenoch/mygoui/theme"
	"github.com/Neuz/DevToys/internal/contract"
	"github.com/Neuz/DevToys/locales"
	"github.com/egoist/mygo/ui"
	"github.com/samber/lo"
)

// 侧栏宽度：展开时的宽度与折叠后的图标轨道宽度。
const (
	siderbarWidth          float32 = 180
	siderbarCollapsedWidth float32 = 48
)

// sidebar 绘制模块导航：按模块声明的分组聚合，分组顺序取注册顺序中首次出现的位置。
// 整个侧栏同时是窗口拖动区域，空白处按下即可移动窗口。
func (s *Shell) sidebar(c *ui.Context) {
	labels := lo.Uniq(lo.Map(s.modules, func(m contract.Module, _ int) string { return m.Group() }))
	byGroup := lo.GroupBy(s.modules, func(m contract.Module) string { return m.Group() })
	groups := lo.Map(labels, func(label string, _ int) mygoui.SidebarGroupOptions {
		return mygoui.SidebarGroupOptions{
			Key:   label,
			Label: label,
			Items: lo.Map(byGroup[label], sidebarItem),
		}
	})
	mygoui.Sidebar(c, mygoui.SidebarOptions{
		Selected:       &s.current,
		Collapsed:      &s.collapsed,
		Width:          siderbarWidth,
		CollapsedWidth: siderbarCollapsedWidth,
		Groups:         groups,
		Footer:         func() { s.settingsButton(c) },
		OnSelect:       s.activate,
	}).DragWindow()
}

// sidebarItem 把一个模块转成侧栏条目；没有图标的模块使用通用工具图标。
func sidebarItem(m contract.Module, _ int) mygoui.SidebarItemOptions {
	icon := m.Icon()
	if icon == nil {
		icon = lucide.Wrench()
	}
	return mygoui.SidebarItemOptions{ID: m.ID(), Label: m.Title(), Icon: icon}
}

// settingsButton 是侧栏底部的设置入口，外观与导航条目一致：
// 同样的行高、内边距、圆角与悬停底色；侧栏折叠时只留图标并显示提示。
func (s *Shell) settingsButton(c *ui.Context) {
	t := theme.Colors(c)
	label := locales.SettingsTitle()
	e := ui.ButtonBase(c.Key("sidebar-settings")).FillWidth().MinWidth(0).
		MinHeight(theme.Scale(c, 32)).Padding(theme.Space(2)).
		Gap(theme.Space(2)).AlignItems(ui.Center).Justify(ui.Start).
		Label(label).Role(ui.RoleButton).Cursor(ui.CursorPointer)

	ink, fill := t.SidebarForeground, ui.Transparent
	if e.Hovered() || e.Pressed() {
		ink, fill = t.SidebarAccentForeground, t.SidebarAccent
	}
	radius := theme.RadiusScale().MD
	e.TextColor(ink).Radius(radius).Clip().Background(fill).FocusRing(false)
	if e.FocusVisible() {
		ring := t.SidebarRing
		e.DrawOver(func(p *ui.Painter, r ui.Rect) { p.Stroke(r, ring, radius, 2) })
	}
	if s.collapsed {
		e.Tooltip(label)
	}
	e.Children(func() {
		size := theme.Scale(c, theme.IconMD)
		ui.Icon(c, lucide.Settings()).Size(size, size).Shrink(0)
		if !s.collapsed {
			// 取 theme.SM() 的度量而不是 Typography(TypeBodySM)：
			// 后者行高 24，会让这一行比导航条目高 4 DIP。
			ts, scale := theme.SM(), theme.TextScale(c)
			ui.Text(c, label).
				Font(theme.FontFamily(c)).
				FontSize(ts.Size * scale).
				FixedLineHeight(ts.LineHeight * scale).
				FontWeight(ts.Weight).
				MinWidth(0).Grow(1).SingleLine()
		}
	})
	e.OnClick(s.openSettings)
}
