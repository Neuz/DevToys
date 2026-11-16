// Package welcome 是应用启动后显示的功能页。
package welcome

import (
	"log/slog"

	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Cyenoch/mygoui"
	"github.com/Cyenoch/mygoui/theme"
	"github.com/Neuz/DevToys/internal/contract"
	"github.com/Neuz/DevToys/internal/task"
	"github.com/Neuz/DevToys/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Module 实现 contract.Module。
type Module struct {
	tasks *task.Runner
	log   *slog.Logger
}

// New 返回欢迎页。
func New() *Module { return &Module{} }

func (m *Module) ID() string { return "welcome" }

func (m *Module) Title() string { return locales.WelcomeTitle() }

// Icon 让折叠后的侧栏显示为图标轨道。
func (m *Module) Icon() *ui.SVG { return lucide.House() }

func (m *Module) Init(h contract.Host) {
	m.tasks = h.Tasks()
	m.log = h.Log()
}

func (m *Module) View(c *ui.Context) {
	theme.Apply(c)
	version := mygo.App.Version()
	if version == "" {
		version = "dev"
	}
	ui.Column(c).Fill().Padding(theme.Space(8)).Gap(theme.Space(5)).Children(func() {
		ui.Column(c).Gap(theme.Space(1)).Children(func() {
			mygoui.Heading(c, 1, locales.WelcomeTitle())
			mygoui.Typography(c, "v"+version, mygoui.TypographyOptions{
				Type:  theme.TypeBodySM,
				Muted: true,
			})
			mygoui.Typography(c, locales.WelcomeSubtitle(), mygoui.TypographyOptions{
				Type: theme.TypeBody,
			})
		})
		ui.Row(c).Gap(theme.Space(4)).Children(func() {
			m.card(c, locales.WelcomeHintSidebar())
			m.card(c, locales.WelcomeHintTitlebar())
		})
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
				})
			}).Padding(theme.Space(4), 0)
		})
	})
}
