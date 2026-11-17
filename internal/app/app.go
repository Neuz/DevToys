// Package app 装配应用的各层，并启动 mygo。
package app

import (
	"log/slog"

	"github.com/Neuz/DevToys/internal/config"
	"github.com/Neuz/DevToys/internal/contract"
	"github.com/Neuz/DevToys/internal/features/welcome"
	"github.com/Neuz/DevToys/internal/logging"
	"github.com/Neuz/DevToys/internal/shell"
	"github.com/Neuz/DevToys/resources"
	"github.com/egoist/mygo"
)

func setupModules() []contract.Module {
	return []contract.Module{
		welcome.New(),
	}
}

// Run 创建配置、日志、模块与外壳，然后进入 mygo 的事件循环。
func Run() error {
	// 配置
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	// 日志
	logger, closeLog, err := logging.New(cfg)
	if err != nil {
		return err
	}
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { _ = closeLog() })
	slog.SetDefault(logger)

	icon, err := resources.AppIconPNG()
	if err != nil {
		return err
	}

	opts := shell.Options{
		Name:      mygo.App.Name(),
		Modules:   setupModules(),
		Width:     800,
		Height:    550,
		MinWidth:  800,
		MinHeight: 550,
		Config:    &cfg,
		Log:       logger,
		Tray: &shell.Tray{
			Icon:    icon,
			ToolTip: mygo.App.Name(),
		},
	}
	if err := shell.Run(opts); err != nil {
		return err
	}
	return mygo.App.Run()
}
