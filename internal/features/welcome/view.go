package welcome

import (
	"fmt"

	"github.com/Cyenoch/mygoui"
	"github.com/Cyenoch/mygoui/theme"
	"github.com/Neuz/DevToys/internal/features/welcome/locales"
	"github.com/egoist/mygo/ui"
)

// View 绘制欢迎页：标题、版本、说明与两张提示卡片。
func (m *Module) View(c *ui.Context) {
	theme.Apply(c)
	ui.Column(c).Fill().Padding(theme.Space(8)).Gap(theme.Space(5)).Children(func() {
		ui.Column(c).Gap(theme.Space(1)).Children(func() {
			mygoui.Heading(c, 1, locales.T("title"))
			mygoui.Typography(c, locales.T("subtitle"), mygoui.TypographyOptions{
				Type: theme.TypeBody,
			})
		})
		ui.Row(c).Gap(theme.Space(4)).Children(func() {
			m.card(c, locales.T("hint_sidebar"))
		})
		ui.Row(c).Gap(theme.Space(4)).Children(func() {
			m.card(c, locales.T("hint_sidebar"))
		})
		ui.Row(c).Gap(theme.Space(4)).Children(func() {
			m.card(c, locales.T("hint_sidebar"))
		})
		ui.Row(c).Gap(theme.Space(4)).Children(func() {
			m.card(c, locales.T("hint_sidebar"))
		})
		ui.Row(c).Gap(theme.Space(4)).Children(func() {
			m.card(c, locales.T("hint_sidebar"))
		})
		ui.Row(c).Gap(theme.Space(4)).Children(func() {
			m.card(c, locales.T("hint_sidebar"))
		})
		ui.Row(c).Gap(theme.Space(4)).Children(func() {
			m.card(c, locales.T("hint_sidebar"))
		})
		ui.Row(c).Gap(theme.Space(4)).Children(func() {
			m.card(c, locales.T("hint_sidebar"))
		})
		ui.Row(c).Gap(theme.Space(4)).Children(func() {
			m.card(c, locales.T("hint_sidebar"))
		})
		ui.Row(c).Gap(theme.Space(4)).Children(func() {
			m.card(c, locales.T("hint_sidebar"))
		})
		m.demoSection(c)
	})
}

// demoSection 演示 task.Runner 与 slog：按钮在后台跑 2 秒，期间显示 loading。
func (m *Module) demoSection(c *ui.Context) {
	ui.Row(c).Gap(theme.Space(3)).AlignItems(ui.Center).Children(func() {
		mygoui.Button(c, mygoui.ButtonOptions{
			Label:   locales.T("demo_run"),
			Loading: m.demo != nil && m.demo.Running(),
			OnClick: m.runDemo,
		})
		if m.demo == nil {
			return
		}
		if m.demo.Running() {
			mygoui.Typography(c, locales.T("demo_running"), mygoui.TypographyOptions{
				Type:  theme.TypeBodySM,
				Muted: true,
			})
			return
		}
		if elapsed, ok := m.demo.Value(); ok {
			mygoui.Typography(c, fmt.Sprintf(locales.T("demo_done"), elapsed), mygoui.TypographyOptions{
				Type:  theme.TypeBodySM,
				Muted: true,
			})
		}
	})
}

// card 是一张提示卡片；两张卡片在宽窗口里平分一行。
func (m *Module) card(c *ui.Context, text string) {
	ui.Column(c).Grow(1).Basis(0).MinWidth(220).Children(func() {
		mygoui.Card(c, mygoui.CardOptions{}, func() {
			mygoui.CardContent(c, func() {
				mygoui.Typography(c, text, mygoui.TypographyOptions{
					Type:  theme.TypeBody,
					Muted: true,
				}).Padding(theme.Space(4))
			}).Padding(theme.Space(4), 0)
		})
	})
}
