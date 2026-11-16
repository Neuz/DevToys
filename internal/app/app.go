// Package app 装配应用的各层，并启动 mygo。
package app

import (
	"log/slog"
	"path/filepath"

	"github.com/Neuz/DevToys/internal/config"
	"github.com/Neuz/DevToys/internal/contract"
	"github.com/Neuz/DevToys/internal/features/launcher"
	"github.com/Neuz/DevToys/internal/features/settings"
	"github.com/Neuz/DevToys/internal/features/welcome"
	"github.com/Neuz/DevToys/internal/kit"
	"github.com/Neuz/DevToys/internal/logging"
	"github.com/Neuz/DevToys/internal/prefs"
	"github.com/Neuz/DevToys/internal/shell"
	"github.com/egoist/mygo"
)

// Run 创建配置、日志、模块与外壳，然后进入 mygo 的事件循环。
func Run() error {
	mygo.App.SetName("DevToys")

	dir, err := config.DefaultDir("DevToys")
	if err != nil {
		return err
	}
	cfg, cfgErr := config.Load(dir)
	if cfgErr != nil {
		cfg = config.Default()
	}

	logger, closeLog, err := logging.New(filepath.Join(dir, "logs"), mygo.IsDev())
	if err != nil {
		return err
	}
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { _ = closeLog() })
	slog.SetDefault(logger)
	if cfgErr != nil {
		logger.Warn("config: using defaults", "error", cfgErr)
	}

	// 在 App.Run 之前设置，让首帧就是保存的外观。
	prefs.ApplyTheme(cfg.Theme)

	modules := []contract.Module{
		welcome.New(),
	}

	opts := shell.Options{
		Name:         "DevToys",
		Version:      mygo.App.Version(),
		Modules:      modules,
		DefaultID:    cfg.LastModule,
		SidebarWidth: cfg.SidebarWidth,
		ThemeMode:    func() config.ThemeMode { return cfg.Theme },
		Log:          logger,
		OnThemeChange: func(mode config.ThemeMode) {
			if cfg.Theme == mode {
				return
			}
			cfg.Theme = mode
			if err := cfg.Save(dir); err != nil {
				logger.Warn("config: saving the theme", "error", err)
			}
		},
		OnSwitch: func(id string) {
			if cfg.LastModule == id {
				return
			}
			cfg.LastModule = id
			if err := cfg.Save(dir); err != nil {
				logger.Warn("config: saving the last module", "error", err)
			}
		},
		Tray: &shell.Tray{
			Icon:    kit.TrayIcon(),
			ToolTip: "DevToys",
			Width:   560,
			Panel: func(ctl contract.Controller) contract.Panel {
				return launcher.New(ctl)
			},
		},
		Settings: func(ctl contract.Controller) contract.Settings {
			return settings.New(settings.Options{
				Config: cfg,
				Theme:  func() config.ThemeMode { return cfg.Theme },
				Locale: func() string { return mygo.App.ChosenLocale() },
				Log:    logger.With("module", "settings"),
				// 设置页只拥有主题，其余字段以 app 持有的 cfg 为准。
				Save: func(c config.Config) error {
					cfg.Theme = c.Theme
					return cfg.Save(dir)
				},
			})
		},
	}
	if err := shell.Run(opts); err != nil {
		return err
	}
	return mygo.App.Run()
}
