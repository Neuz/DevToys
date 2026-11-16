// Package launcher 是托盘图标打开的启动器面板，本期只搭骨架。
package launcher

import (
	"log/slog"

	"github.com/Cyenoch/mygoui"
	"github.com/Cyenoch/mygoui/theme"
	"github.com/Neuz/DevToys/internal/contract"
	"github.com/Neuz/DevToys/internal/task"
	"github.com/Neuz/DevToys/locales"
	"github.com/egoist/mygo/ui"
)

const height = 420

// Panel 实现 contract.Panel。
type Panel struct {
	ctl   contract.Controller
	tasks *task.Runner
	log   *slog.Logger
}

// New 返回启动器面板。
func New(ctl contract.Controller) *Panel {
	return &Panel{ctl: ctl}
}

func (p *Panel) Init(h contract.Host) {
	p.tasks = h.Tasks()
	p.log = h.Log()
}

// Show 显示面板。
func (p *Panel) Show() {
	p.log.Debug("launcher: opened")
}

func (p *Panel) Hide() {}

// Height 是面板的固定高度。
func (p *Panel) Height() int { return height }

func (p *Panel) View(c *ui.Context) {
	theme.Apply(c)
	ui.Column(c).Fill().Padding(theme.Space(4)).Gap(theme.Space(3)).Children(func() {
		mygoui.Heading(c, 2, locales.WelcomeTitle())
		mygoui.Typography(c, locales.LauncherEmpty(), mygoui.TypographyOptions{
			Type:  theme.TypeBodySM,
			Muted: true,
		})
		ui.Row(c).Gap(theme.Space(2)).Children(func() {
			mygoui.Button(c, mygoui.ButtonOptions{
				Label:   locales.LauncherOpenMain(),
				Variant: mygoui.ButtonPrimary,
				OnClick: func() { p.ctl.ShowMain("welcome") },
			})
			mygoui.Button(c, mygoui.ButtonOptions{
				Label:   locales.LauncherQuit(),
				Variant: mygoui.ButtonSecondary,
				OnClick: p.ctl.Quit,
			})
		})
	})
}
