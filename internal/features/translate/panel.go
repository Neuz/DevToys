package translate

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Cyenoch/mygoui"
	"github.com/Cyenoch/mygoui/form"
	"github.com/Cyenoch/mygoui/list"
	"github.com/Cyenoch/mygoui/theme"
	"github.com/Neuz/DevToys/internal/features/translate/locales"
	"github.com/Neuz/DevToys/internal/task"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	panelWidth          = 400
	panelHeightDefault  = 560
	panelMinHeight      = 400
	panelMaxHeight      = 720
	panelMaxHeightRatio = 0.8
	panelEdgeMargin     = 8
	panelOffsetX        = 16
	panelOffsetY        = 20
	blurHideDelay       = 100 * time.Millisecond
	selectionTimeout    = 3 * time.Second
)

// Rect 是工作区或窗口的屏幕矩形（物理像素）。
type Rect struct{ X, Y, Width, Height int }

// toggle 由全局快捷键调用；回调可能不在主线程，统一回到主线程执行。
func (m *Module) toggle() {
	mygo.RunOnMain(m.toggleMain)
}

// toggleMain 在面板隐藏时读取选区并显示；已显示时只刷新选区，
// 无选区则保持当前内容。
func (m *Module) toggleMain() {
	if m.lifeCtx == nil {
		return
	}
	m.readSelectionThenShow()
}

// readSelectionThenShow 在 Runner 上读取选区，结果回到主线程应用；
// 同 key 的新任务会取消旧读取，被顶替或关闭时不再回调。
func (m *Module) readSelectionThenShow() {
	if m.tasks == nil {
		return
	}
	task.Go(m.tasks, "selection", func(ctx context.Context) (struct{}, error) {
		ctx, cancel := context.WithTimeout(ctx, selectionTimeout)
		defer cancel()
		selection, err := Read(ctx)
		if ctx.Err() != nil {
			return struct{}{}, ctx.Err()
		}
		mygo.RunOnMain(func() { m.applySelection(selection, err) })
		return struct{}{}, nil
	})
}

// applySelection 把取词结果落到面板：有文本替换并翻译，无文本只提示。
func (m *Module) applySelection(selection Selection, err error) {
	if m.lifeCtx == nil || m.lifeCtx.Err() != nil {
		return
	}
	if m.panel != nil && m.panel.IsVisible() {
		if err == nil && strings.TrimSpace(selection.Text) != "" {
			m.query = strings.TrimSpace(selection.Text)
			m.notice = ""
			m.translateNow()
			m.input.SelectAll()
		}
		return
	}
	m.notice = selectionNotice(err)
	m.query = strings.TrimSpace(selection.Text)
	m.showPanel(selection.Point, err == nil)
}

// selectionNotice 把取词错误转成面板顶部的一句提示。
func selectionNotice(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, ErrNoSelection):
		return ""
	case errors.Is(err, ErrUnsupported):
		return locales.T("panel_notice_unsupported")
	case errors.Is(err, context.DeadlineExceeded):
		return locales.T("panel_notice_timeout")
	default:
		return locales.T("panel_notice_failed")
	}
}

// createPanel 创建一个 Spotlight 式面板：无边框、置顶、不占任务栏。
func (m *Module) createPanel() {
	m.panel = mygo.NewWindow(mygo.WindowOptions{
		Title:         locales.T("title"),
		Panel:         true,
		Frameless:     true,
		AlwaysOnTop:   true,
		SkipTaskbar:   true,
		Hidden:        true,
		CornerRadius:  12,
		Width:         panelWidth,
		Height:        panelMinHeight,
		DisableResize: true,
		Content:       ui.View(m.panelView),
	})
	m.panelFlyouts.Window = m.panel
	m.panel.OnClose(func(e *mygo.CloseEvent) {
		e.PreventDefault()
		m.hidePanel()
	})
	// 失焦后稍等再隐藏；固定时保持打开。
	m.panel.OnBlur(func() {
		if m.pinned {
			return
		}
		time.AfterFunc(blurHideDelay, func() {
			mygo.RunOnMain(func() {
				if m.panel != nil && !m.pinned && !m.sourceSelect.Open &&
					!m.targetSelect.Open && !m.panel.IsFocused() {
					m.hidePanel()
				}
			})
		})
	})
}

// showPanel 重置面板状态、定位并显示。hasAnchor 为 false 时居中显示。
func (m *Module) showPanel(anchor Point, hasAnchor bool) {
	if m.panel == nil {
		m.createPanel()
	}
	m.pinned = false
	m.copiedID = ""
	m.cards = nil
	m.gen = 0
	m.resetCardsScroll()
	if m.service != nil {
		m.service.Cancel()
	}
	m.fitPanelHeight()
	m.positionPanel(anchor, hasAnchor)
	m.panel.Show()
	if strings.TrimSpace(m.query) != "" {
		// 原文来自选区时全选，直接输入即可替换。
		m.input.SelectAll()
	} else {
		m.input.Focus()
	}
	if strings.TrimSpace(m.query) != "" {
		m.translateNow()
	}
}

// positionPanel 按鼠标或选区位置放置面板，拿不到工作区时回退到窗口居中。
func (m *Module) positionPanel(anchor Point, hasAnchor bool) {
	screenPoint := mygo.Screen.CursorScreenPoint()
	point := Point{X: screenPoint.X, Y: screenPoint.Y}
	if hasAnchor {
		point = anchor
	}
	display := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	area := display.WorkArea
	rect := Rect{X: area.X, Y: area.Y, Width: area.Width, Height: area.Height}
	height := m.panelHeight
	if height <= 0 {
		height = panelMinHeight
	}
	follow := m.opts.PanelFollowsCursor
	if !follow {
		point = Point{X: rect.X + rect.Width/2, Y: rect.Y + rect.Height/2}
	}
	x, y := panelPosition(point, panelWidth, height, rect, follow)
	m.panel.SetPosition(x, y)
}

// panelPosition 计算面板左上角。follows 为 false 时在工作区居中；
// 否则放在锚点右下方，越界时翻到左上方，再夹紧到工作区。
func panelPosition(anchor Point, width, height int, area Rect, follows bool) (int, int) {
	if area.Width <= 0 || area.Height <= 0 {
		return anchor.X, anchor.Y
	}
	if !follows {
		return area.X + (area.Width-width)/2, area.Y + (area.Height-height)/2
	}
	x := anchor.X + panelOffsetX
	y := anchor.Y + panelOffsetY
	if x+width > area.X+area.Width-panelEdgeMargin {
		x = anchor.X - panelOffsetX - width
	}
	if y+height > area.Y+area.Height-panelEdgeMargin {
		y = anchor.Y - panelOffsetY - height
	}
	maxX := area.X + area.Width - width - panelEdgeMargin
	if maxX < area.X+panelEdgeMargin {
		maxX = area.X + panelEdgeMargin
	}
	maxY := area.Y + area.Height - height - panelEdgeMargin
	if maxY < area.Y+panelEdgeMargin {
		maxY = area.Y + panelEdgeMargin
	}
	return clamp(x, area.X+panelEdgeMargin, maxX), clamp(y, area.Y+panelEdgeMargin, maxY)
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

// panelHeightFor 返回固定面板高度，并夹在工作区比例内。
// 内容超出时由结果区滚动，不再按卡片数量改变窗口高度。
func panelHeightFor(scale float32, areaHeight int) int {
	if scale <= 0 {
		scale = 1
	}
	height := float64(panelHeightDefault) * float64(scale)
	low := float64(panelMinHeight)
	high := float64(panelMaxHeight)
	if areaHeight > 0 {
		high = math.Min(high, float64(areaHeight)*panelMaxHeightRatio)
	}
	if high < low {
		high = low
	}
	return int(math.Ceil(math.Min(math.Max(height, low), high)))
}

// workAreaHeight 返回鼠标所在显示器的工作区高度；未知时返回 0。
func workAreaHeight() int {
	if display := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint()); display.WorkArea.Height > 0 {
		return display.WorkArea.Height
	}
	return 0
}

// fitPanelHeight 把面板高度调整到固定布局高度；窗口左上角保持不变。
func (m *Module) fitPanelHeight() {
	if m.panel == nil {
		return
	}
	m.setPanelHeight(panelHeightFor(m.textScale, workAreaHeight()))
}

// setPanelHeight 调整面板高度；高度没变时跳过重复布局。
func (m *Module) setPanelHeight(height int) {
	if m.panel == nil || height <= 0 || height == m.panelHeight {
		return
	}
	m.panelHeight = height
	m.panel.SetSize(panelWidth, height)
}

// hidePanel 隐藏面板并取消当前请求；窗口保留到模块退出。
func (m *Module) hidePanel() {
	m.stopDebounce()
	if m.service != nil {
		m.service.Cancel()
	}
	m.pinned = false
	m.cards = nil
	if m.panel != nil {
		m.panel.Hide()
	}
}

const (
	inputDebounce  = 600 * time.Millisecond
	copiedFeedback = 1500 * time.Millisecond
)

// card 是一个服务在面板里的状态。
type card struct {
	id       string
	name     string
	text     string
	err      string
	detected string
	latency  time.Duration
	loading  bool
}

// panelView 绘制翻译面板：顶部工具行、原文输入、语言选择、结果卡片。
func (m *Module) panelView(c *ui.Context) {
	theme.Apply(c)
	if c.Shortcut(0, ui.KeyEscape) {
		m.hidePanel()
	}
	t := theme.Colors(c)
	scale := theme.TextScale(c)
	m.textScale = scale
	ui.Column(c).Fill().Padding(theme.Space(2.5)).Gap(theme.Space(1.5)).
		Background(t.Background).TextColor(t.Foreground).Children(func() {
		m.renderHeader(c, t)
		m.renderInput(c)
		m.renderLanguages(c, t)
		if m.notice != "" {
			mygoui.Typography(c, m.notice, mygoui.TypographyOptions{
				Type:  theme.TypeBodySM,
				Muted: true,
			})
		}
		if strings.TrimSpace(m.query) != "" && (m.service == nil || len(m.service.IDs()) == 0) {
			mygoui.Typography(c, locales.T("settings_no_provider"), mygoui.TypographyOptions{
				Type:  theme.TypeBodySM,
				Muted: true,
			})
		}
		// 结果区固定占用剩余高度，长译文与多服务卡片在区域内滚动。
		mygoui.ScrollArea(c, &m.cardsScroll, mygoui.ScrollAreaOptions{
			Axis:  mygoui.ScrollVertical,
			Label: locales.T("panel_results"),
		}, func() {
			ui.Column(c).FillWidth().Gap(theme.Space(1.5)).Children(func() {
				for _, item := range m.cards {
					m.renderCard(c, t, item)
				}
			})
		}).FillWidth().Grow(1).TabStop(false)
		mygoui.Typography(c, locales.T("panel_hint"), mygoui.TypographyOptions{
			Type:  theme.TypeBodyXS,
			Muted: true,
		})
	})
	m.fitPanelHeight()
}

// renderHeader 绘制可拖动的顶部工具行：固定与关闭。
func (m *Module) renderHeader(c *ui.Context, t theme.Palette) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).DragWindow().Children(func() {
		icon, label := lucide.Pin(), locales.T("panel_pin")
		if m.pinned {
			icon, label = lucide.PinOff(), locales.T("panel_unpin")
		}
		m.iconButton(c, icon, label, m.togglePin)
		ui.Row(c).Grow(1)
		m.iconButton(c, lucide.X(), locales.T("panel_close"), m.hidePanel)
	})
	ui.Row(c).FillWidth().Height(1).Background(t.Border)
}

// iconButton 绘制图标按钮，仅图标但仍保留可访问名称。
func (m *Module) iconButton(c *ui.Context, icon *ui.SVG, label string, onClick func()) {
	mygoui.Button(c, mygoui.ButtonOptions{
		Label:    label,
		Icon:     icon,
		IconOnly: true,
		Variant:  mygoui.ButtonGhost,
		Size:     theme.ButtonSmall,
		OnClick:  onClick,
	})
}

// renderInput 绘制多行原文输入框：3 行起，最多 6 行，再长则在框内滚动。
func (m *Module) renderInput(c *ui.Context) {
	form.TextArea(c, &m.query, form.TextAreaOptions{
		AriaLabel:   locales.T("title"),
		Placeholder: locales.T("panel_placeholder"),
		FullWidth:   true,
		MinLines:    3,
		MaxLines:    6,
		OnChange:    m.onInputChanged,
	}).Bind(&m.input).FillWidth().MinWidth(0).OnTextKey(m.onTextKey)
}

// renderLanguages 绘制源语言、互换、目标语言与手动翻译按钮。
func (m *Module) renderLanguages(c *ui.Context, t theme.Palette) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(theme.Space(1.5)).Children(func() {
		form.Select(c, form.SelectOptions{
			Items:   sourceItems(),
			State:   &m.sourceSelect,
			Variant: form.ControlSecondary,
			Flyout:  &m.panelFlyouts,
			OnChange: func(keys []string) {
				if len(keys) == 0 {
					return
				}
				m.source = keys[0]
				m.retranslate()
			},
		}).Width(theme.Scale(c, 104))
		m.iconButton(c, lucide.ArrowRightLeft(), locales.T("panel_swap"), m.swapLanguages)
		form.Select(c, form.SelectOptions{
			Items:   targetItems(),
			State:   &m.targetSelect,
			Variant: form.ControlSecondary,
			Flyout:  &m.panelFlyouts,
			OnChange: func(keys []string) {
				if len(keys) == 0 {
					return
				}
				m.opts.Target = keys[0]
				m.changeOptions()
			},
		}).Width(theme.Scale(c, 104))
		ui.Row(c).Grow(1)
		mygoui.Button(c, mygoui.ButtonOptions{
			Label:    locales.T("panel_translate"),
			Variant:  mygoui.ButtonPrimary,
			Size:     theme.ButtonSmall,
			Disabled: strings.TrimSpace(m.query) == "",
			OnClick:  m.retranslate,
		})
	})
}

// renderCard 绘制单个服务的头部、译文/错误与底部操作。
func (m *Module) renderCard(c *ui.Context, t theme.Palette, item card) {
	collapsed := m.collapsed[item.id]
	ui.Column(c).FillWidth().Padding(theme.Space(1.75)).Gap(theme.Space(1)).
		Background(t.Card).Border(1, t.Border).Radius(theme.RadiusScale().MD).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(theme.Space(1.5)).Children(func() {
			ui.Icon(c, providerIcon(item.id)).Size(16, 16).TextColor(t.MutedForeground)
			mygoui.Typography(c, item.name, mygoui.TypographyOptions{Type: theme.TypeBodySM, Weight: 500})
			ui.Row(c).Grow(1)
			icon, label := lucide.ChevronUp(), locales.T("panel_collapse")
			if collapsed {
				icon, label = lucide.ChevronDown(), locales.T("panel_expand")
			}
			m.iconButton(c, icon, label, func() { m.toggleCard(item.id) })
		})
		if collapsed {
			return
		}
		switch {
		case item.loading:
			mygoui.Typography(c, locales.T("panel_translating"), mygoui.TypographyOptions{
				Type:  theme.TypeBodySM,
				Muted: true,
			})
		case item.err != "":
			mygoui.Typography(c, item.err, mygoui.TypographyOptions{
				Type:  theme.TypeBodySM,
				Muted: true,
			})
			m.renderCardMeta(c, item)
		default:
			mygoui.Typography(c, item.text, mygoui.TypographyOptions{
				Type:       theme.TypeBodySM,
				Selectable: true,
			})
			m.renderCardMeta(c, item)
		}
	})
}

// renderCardMeta 绘制“检测到语言 · 延迟”和重试/复制操作。
func (m *Module) renderCardMeta(c *ui.Context, item card) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(theme.Space(1)).Children(func() {
		meta := cardMeta(item)
		if meta != "" {
			mygoui.Typography(c, meta, mygoui.TypographyOptions{Type: theme.TypeBodyXS, Muted: true})
		}
		ui.Row(c).Grow(1)
		m.iconButton(c, lucide.RefreshCw(), locales.T("panel_retry"), func() {
			m.translateProvider(item.id)
		})
		if item.err == "" && item.text != "" {
			icon, label := lucide.Copy(), locales.T("panel_copy")
			if m.copiedID == item.id {
				icon, label = lucide.Check(), locales.T("panel_copied")
			}
			m.iconButton(c, icon, label, func() { m.copyResult(item.id) })
		}
	})
}

// cardMeta 拼出卡片底部的检测语言与耗时。
func cardMeta(item card) string {
	var parts []string
	if item.detected != "" {
		parts = append(parts, locales.T("panel_detected")+" "+languageName(item.detected))
	}
	if item.latency > 0 {
		parts = append(parts, fmt.Sprintf("%d ms", item.latency.Milliseconds()))
	}
	return strings.Join(parts, " · ")
}

// onTextKey 处理面板里的 Escape 与 Ctrl+Enter。
func (m *Module) onTextKey(k ui.TextKey) bool {
	if k.Key == ui.KeyEscape {
		m.hidePanel()
		return true
	}
	if k.Key == ui.KeyEnter && k.Mods == ui.Ctrl {
		m.retranslate()
		return true
	}
	return false
}

// retranslate 取消在途请求并立即翻译当前原文。
func (m *Module) retranslate() {
	if m.service != nil {
		m.service.Cancel()
	}
	m.stopDebounce()
	m.translateNow()
}

// swapLanguages 把当前源语言与目标语言互换后重新翻译。
func (m *Module) swapLanguages() {
	text := strings.TrimSpace(m.query)
	if text == "" {
		return
	}
	source, target := resolveLanguagesFrom(text, m.source, m.opts.Target)
	if source == target {
		return
	}
	m.source = target
	m.sourceSelect.List.SelectedKey = m.source
	m.opts.Target = source
	m.changeOptions()
}

// toggleCard 折叠或展开单个服务的结果。
func (m *Module) toggleCard(id string) {
	if m.collapsed == nil {
		m.collapsed = make(map[string]bool)
	}
	m.collapsed[id] = !m.collapsed[id]
	if m.panel != nil {
		m.panel.Invalidate()
	}
}

// onInputChanged 让当前结果失效，并在停止输入 600ms 后翻译。
func (m *Module) onInputChanged() {
	if m.service != nil {
		m.service.Cancel()
	}
	m.stopDebounce()
	m.debounceSeq++
	m.cards = nil
	m.copiedID = ""
	m.gen = 0
	m.resetCardsScroll()
	if m.panel != nil {
		m.panel.Invalidate()
	}
	if strings.TrimSpace(m.query) == "" {
		return
	}
	seq := m.debounceSeq
	m.debounce = time.AfterFunc(inputDebounce, func() {
		mygo.RunOnMain(func() {
			if m.debounceSeq != seq {
				return
			}
			m.debounce = nil
			m.translateNow()
		})
	})
}

// translateNow 立即翻译当前原文；空文本或没有启用服务时清空卡片。
func (m *Module) translateNow() {
	m.stopDebounce()
	if m.service == nil {
		return
	}
	text := strings.TrimSpace(m.query)
	ids := m.service.IDs()
	if text == "" || len(ids) == 0 {
		m.cards = nil
		m.resetCardsScroll()
		if m.panel != nil {
			m.panel.Invalidate()
		}
		return
	}
	source, _ := resolveLanguagesFrom(text, m.source, m.opts.Target)
	m.cards = make([]card, 0, len(ids))
	for _, id := range ids {
		m.cards = append(m.cards, card{
			id:       id,
			name:     providerTitle(id),
			detected: source,
			loading:  true,
		})
	}
	m.copiedID = ""
	m.resetCardsScroll()
	m.gen = m.service.Translate(text, m.source, m.opts.Target, func(gen uint64, p Provider, res Result, err error) {
		mygo.RunOnMain(func() { m.applyResult(gen, p, res, err) })
	})
	if m.panel != nil {
		m.panel.Invalidate()
	}
}

// translateProvider 只重试一个服务，保留其他卡片的内容。
func (m *Module) translateProvider(id string) {
	if m.service == nil || m.lifeCtx == nil {
		return
	}
	var provider Provider
	for _, candidate := range m.service.providers {
		if candidate.ID() == id {
			provider = candidate
			break
		}
	}
	if provider == nil {
		return
	}
	text := strings.TrimSpace(m.query)
	if text == "" {
		return
	}
	for i := range m.cards {
		if m.cards[i].id != id {
			continue
		}
		m.cards[i].loading = true
		m.cards[i].text = ""
		m.cards[i].err = ""
		m.cards[i].latency = 0
	}
	source, target := resolveLanguagesFrom(text, m.source, m.opts.Target)
	gen := m.gen
	if m.panel != nil {
		m.panel.Invalidate()
	}
	task.Go(m.tasks, "retry:"+id, func(ctx context.Context) (struct{}, error) {
		ctx, cancel := context.WithTimeout(ctx, providerTimeout)
		defer cancel()
		res, err := provider.Translate(ctx, Request{Text: text, Source: source, Target: target})
		if ctx.Err() != nil {
			return struct{}{}, ctx.Err()
		}
		mygo.RunOnMain(func() {
			if gen != m.gen {
				return
			}
			m.applyResult(gen, provider, res, err)
		})
		return struct{}{}, nil
	})
}

// applyResult 把单个服务的结果写入对应卡片；过期结果直接丢弃。
func (m *Module) applyResult(gen uint64, p Provider, res Result, err error) {
	if gen != m.gen || m.service == nil {
		return
	}
	index := -1
	for i := range m.cards {
		if m.cards[i].id == p.ID() {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	m.cards[index].loading = false
	if err != nil {
		m.cards[index].text = ""
		m.cards[index].err = strings.TrimPrefix(err.Error(), p.ID()+": ")
		if m.log != nil {
			m.log.Debug("translate: 服务失败", "provider", p.ID(), "error", err)
		}
	} else {
		m.cards[index].err = ""
		m.cards[index].text = res.Text
		m.cards[index].latency = res.Latency
		if res.Detected != "" {
			m.cards[index].detected = res.Detected
		}
		if m.log != nil {
			m.log.Debug("translate: 服务完成", "provider", p.ID(), "latency", res.Latency)
		}
	}
	if m.panel != nil {
		m.panel.Invalidate()
	}
}

// copyResult 复制单个服务的译文，并短暂显示“已复制”。
func (m *Module) copyResult(id string) {
	for _, item := range m.cards {
		if item.id != id || item.err != "" || item.text == "" {
			continue
		}
		mygo.Clipboard.WriteText(item.text)
		break
	}
	m.copiedID = id
	m.copiedSeq++
	seq := m.copiedSeq
	if m.copyTimer != nil {
		m.copyTimer.Stop()
	}
	m.copyTimer = time.AfterFunc(copiedFeedback, func() {
		mygo.RunOnMain(func() {
			if m.copiedSeq != seq {
				return
			}
			m.copiedID = ""
			if m.panel != nil {
				m.panel.Invalidate()
			}
		})
	})
	if m.panel != nil {
		m.panel.Invalidate()
	}
}

// togglePin 切换固定状态；固定后失焦不隐藏。
func (m *Module) togglePin() {
	m.pinned = !m.pinned
	if m.panel != nil {
		m.panel.Invalidate()
	}
}

// resetCardsScroll 把结果区滚回顶部，新一轮原文从第一张卡片开始显示。
func (m *Module) resetCardsScroll() {
	m.cardsScroll.X, m.cardsScroll.Y = 0, 0
}

// stopDebounce 停止等待中的去抖任务。
func (m *Module) stopDebounce() {
	if m.debounce != nil {
		m.debounce.Stop()
		m.debounce = nil
	}
}

// providerTitle 返回服务的界面名称。
func providerTitle(id string) string {
	switch id {
	case providerGoogle:
		return locales.T("provider_google")
	case providerBilibili:
		return locales.T("provider_bilibili")
	default:
		return id
	}
}

// providerIcon 返回服务的图标。
func providerIcon(id string) *ui.SVG {
	switch id {
	case providerGoogle:
		return lucide.Search()
	case providerBilibili:
		return lucide.Languages()
	default:
		return lucide.Bot()
	}
}

// sourceItems 返回源语言选项：自动识别 + 具体语言。
func sourceItems() []list.ListBoxItem {
	items := make([]list.ListBoxItem, 0, len(targetLanguages))
	items = append(items, list.ListBoxItem{Key: langAuto, Label: locales.T("panel_source_auto")})
	for _, tag := range targetLanguages {
		if tag == langAuto {
			continue
		}
		items = append(items, list.ListBoxItem{Key: tag, Label: languageName(tag)})
	}
	return items
}

// targetItems 返回目标语言选项，顺序与设置页一致。
func targetItems() []list.ListBoxItem {
	items := make([]list.ListBoxItem, 0, len(targetLanguages))
	for _, tag := range targetLanguages {
		items = append(items, list.ListBoxItem{Key: tag, Label: targetLabel(tag)})
	}
	return items
}
