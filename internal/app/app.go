// Package app 装配应用的各层，并启动 mygo。
package app

import (
	"log/slog"

	"github.com/Neuz/DevToys/internal/config"
	"github.com/Neuz/DevToys/internal/contract"
	"github.com/Neuz/DevToys/internal/features/launcher"
	qrcode "github.com/Neuz/DevToys/internal/features/qrcode"
	"github.com/Neuz/DevToys/internal/features/welcome"
	"github.com/Neuz/DevToys/internal/logging"
	"github.com/Neuz/DevToys/internal/network"
	"github.com/Neuz/DevToys/internal/shell"
	"github.com/Neuz/DevToys/resources"
	"github.com/egoist/mygo"
)

func setupModules() []contract.Module {
	return []contract.Module{
		welcome.New(),
		launcher.New(),
		qrcode.New(),
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
	logger, closeLog := logging.New(cfg)
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { _ = closeLog() })
	slog.SetDefault(logger)

	// 代理必须在任何网络请求之前安装；配置无效时回退系统代理。
	if err := network.Init(cfg.Network.Proxy); err != nil {
		logger.Warn("network: 代理配置无效，回退到系统代理", "error", err)
		_ = network.Init(config.ProxyConfig{Mode: config.ProxySystem})
	}

	icon, err := resources.AppIconPNG()
	if err != nil {
		return err
	}

	opts := shell.Options{
		Title: mygo.App.Name(),
		// Subtitle:  fmt.Sprintf("v%s", mygo.App.Version()),
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
