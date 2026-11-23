package launcher

import (
	"math"
	"strings"

	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Cyenoch/mygoui/list"
	"github.com/Cyenoch/mygoui/navigation"
	"github.com/Cyenoch/mygoui/theme"
	"github.com/Neuz/DevToys/internal/features/launcher/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// iconSize 是面板结果行里图标的边长（DIP）。
const iconSize = 24

// panelView 绘制面板内部：搜索框与结果列表。列表由搜索框驱动，
// Escape 在查询为空时冒泡到根快捷键，直接隐藏面板。
func (m *Module) panelView(c *ui.Context) {
	theme.Apply(c)
	if c.Shortcut(0, ui.KeyEscape) {
		m.hidePanel()
	}
	t := theme.Colors(c)
	scale := theme.TextScale(c)
	m.textScale = scale
	ui.Column(c).Fill().Padding(theme.Space(4)).Gap(theme.Space(3)).
		Background(t.Background).TextColor(t.Foreground).Children(func() {
		var input ui.Element
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(theme.Space(2)).Children(func() {
			ui.Icon(c, lucide.Search()).Size(16, 16).TextColor(t.MutedForeground)
			input = ui.TextInputBase(c, &m.query).FillWidth().Grow(1).MinWidth(0).
				Height(float32(panelInputHeight) * scale).
				Label(locales.T("title")).
				Placeholder(locales.T("panel_placeholder")).
				Bind(&m.input)
			input.OnTextKey(m.onTextKey)
		})
		// 默认状态（空查询）只显示搜索框：分类与结果都随查询出现。
		if strings.TrimSpace(m.query) == "" {
			m.setPanelHeight(panelHeightForScale(scale, 0, false))
			return
		}
		m.renderCategories(c)
		entries := m.visibleEntries()
		m.setPanelHeight(panelHeightForScale(scale, len(entries), true))
		// 当前分类没有结果时保留搜索框与分类切换，不渲染列表。
		if len(entries) == 0 {
			return
		}
		if input.Changed() {
			m.list.Highlight = 0
		}
		if m.list.Highlight >= len(entries) {
			m.list.Highlight = max(0, len(entries)-1)
		}
		rows := make([]list.ListBoxItem, 0, len(entries))
		for _, e := range entries {
			entry := e
			rows = append(rows, list.ListBoxItem{
				Key:         entry.Path,
				Label:       entry.Title,
				Description: m.describe(entry),
				Options:     list.ListBoxItemOptions{Height: panelRowHeight},
				Start: func() {
					if entry.Category == categorySystem {
						ui.Icon(c, lucide.Settings2()).Size(iconSize, iconSize).TextColor(t.MutedForeground)
						return
					}
					if bitmap := m.icon(entry); bitmap != nil {
						ui.Image(c, bitmap).Size(iconSize, iconSize)
						return
					}
					ui.Avatar(c, entry.Title, nil).Size(iconSize, iconSize)
				},
			})
		}
		// 搜索框驱动列表：列表不是第二个 Tab 停留点。
		results := list.ListBox(c, list.ListBoxOptions{
			Label:             locales.T("title"),
			Items:             rows,
			State:             &m.list,
			Padding:           theme.Space(2),
			MaxHeight:         panelMaxListHeight(scale),
			NoFocusOnClick:    true,
			KeyboardHighlight: m.input.Focused(c),
			Editor:            &m.input,
			OnAction:          m.launchByPath,
		}).TabStop(false)
		input.Controls(results)
	})
}

// renderCategories 在搜索框下方绘制【全部】【应用】【系统】分段控件。
// 切换分类时重置高亮，下一次查询按新分类过滤。
func (m *Module) renderCategories(c *ui.Context) {
	values := []string{"", categoryApp, categorySystem}
	labels := []string{
		locales.T("panel_category_all"),
		locales.T("panel_category_apps"),
		locales.T("panel_category_system"),
	}
	selected := 0
	for i, value := range values {
		if value == m.category {
			selected = i
		}
	}
	items := make([]navigation.TabOptions, len(labels))
	for i, label := range labels {
		items[i] = navigation.TabOptions{Label: label}
	}
	navigation.SegmentedControl(c, navigation.SegmentedControlOptions{
		Selected: &selected,
		Items:    items,
		Label:    locales.T("panel_category"),
		OnChange: func(index int) {
			m.category = values[index]
			m.list.Highlight = 0
		},
	})
}

// panelHeightForScale 计算面板总高度：上下内边距 + 搜索框，再按需加上分类与列表。
// rows 为 0 表示没有结果；tabs 表示是否显示分类控件。scale 是 theme.TextScale(c)。
func panelHeightForScale(scale float32, rows int, tabs bool) int {
	if scale <= 0 {
		scale = 1
	}
	height := 2*theme.Space(4) + float32(panelInputHeight)*scale
	if tabs {
		height += theme.Space(3) + float32(panelTabsHeight)*scale
	}
	if rows > 0 {
		height += theme.Space(3) + 2*theme.Space(2) +
			float32(min(rows, panelMaxRows))*float32(panelRowHeight)*scale
	}
	return int(math.Ceil(float64(height)))
}

// panelMaxListHeight 是列表展示 panelMaxRows 行时的最大高度（DIP）。
func panelMaxListHeight(scale float32) float32 {
	return 2*theme.Space(2) + float32(panelMaxRows)*float32(panelRowHeight)*scale
}

// onTextKey 把方向键、回车与 Escape 转成面板导航。
func (m *Module) onTextKey(k ui.TextKey) bool {
	if k.Mods != 0 {
		return false
	}
	switch k.Key {
	case ui.KeyDown, ui.KeyUp:
		entries := m.visibleEntriesFor(k.Text())
		if len(entries) == 0 {
			return true
		}
		if k.Key == ui.KeyDown {
			m.list.Highlight = min(m.list.Highlight+1, len(entries)-1)
		} else {
			m.list.Highlight = max(m.list.Highlight-1, 0)
		}
		return true
	case ui.KeyEnter:
		m.activate(k.Text())
		return true
	case ui.KeyEscape:
		// 查询清空后才把 Escape 交给外层隐藏面板。
		if k.Text() == "" {
			return false
		}
		k.SetText("")
		return true
	}
	return false
}

// highlightedEntry 返回当前高亮的条目；没有结果时 ok 为 false。
func (m *Module) highlightedEntry(text string) (Entry, bool) {
	entries := m.visibleEntriesFor(text)
	if len(entries) == 0 {
		return Entry{}, false
	}
	return entries[min(max(m.list.Highlight, 0), len(entries)-1)], true
}

// activate 启动当前高亮的结果；没有结果时什么也不做。
func (m *Module) activate(text string) {
	if e, ok := m.highlightedEntry(text); ok {
		m.launchEntry(e)
	}
}

// visibleEntries 返回当前查询下的结果，已按使用记录排序并截断。
func (m *Module) visibleEntries() []Entry {
	return m.visibleEntriesFor(m.query)
}

// visibleEntriesFor 用给定文本计算结果；空查询不展示任何条目。
// 按键事件里的文本可能领先于帧，所以按传入文本而不是 m.query 计算。
func (m *Module) visibleEntriesFor(text string) []Entry {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return rank(filter(filterCategory(m.entries, m.category), text), m.usage, m.options().MaxResults)
}

// describe 返回条目描述：系统设置固定为「系统设置」，应用显示可执行文件名，
// 打包应用回退到「应用」。
func (m *Module) describe(e Entry) string {
	if e.Category == categorySystem {
		return locales.T("description_system")
	}
	if e.Description != "" {
		return e.Description
	}
	return locales.T("description_app")
}

// icon 返回条目图标；第一次绘制时按显示缩放取 PNG 并缓存，取不到时记住 nil。
func (m *Module) icon(e Entry) *ui.Bitmap {
	if m.icons == nil {
		m.icons = make(map[string]*ui.Bitmap)
	}
	if bitmap, ok := m.icons[e.Path]; ok {
		return bitmap
	}
	scale := 1.0
	for _, display := range mygo.Screen.Displays() {
		scale = max(scale, display.ScaleFactor)
	}
	var bitmap *ui.Bitmap
	if data, err := mygo.Shell.FileIcon(e.Path, int(float64(iconSize)*scale)); err == nil {
		bitmap, _ = ui.DecodeBitmap(data)
	}
	m.icons[e.Path] = bitmap
	return bitmap
}
