// Package logging 创建应用日志器。
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Neuz/DevToys/internal/config"
	"github.com/egoist/mygo"
)

const filename = "devtoys.log"

func defaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, mygo.App.Name()), nil
}

// New 创建应用日志器。
// 日志级别根据配置文件中的 LogLevel 设置。
// 日志文件在 defaultDir 下，文件名是 filename。
func New(cfg config.Config) (*slog.Logger, func() error, error) {
	logDir, err := defaultDir()
	if err != nil {
		return nil, nil, err
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, nil, err
	}
	f, err := os.OpenFile(filepath.Join(logDir, filename), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, err
	}

	lv, err := parseLevelFrom(cfg.LogLevel)
	if err != nil {
		lv = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lv}
	var w io.Writer = f
	var handler slog.Handler

	switch opts.Level {
	case slog.LevelDebug:
		w = io.MultiWriter(f, os.Stderr)
		handler = slog.NewTextHandler(w, opts)
	default:
		handler = slog.NewJSONHandler(w, opts)
	}
	return slog.New(handler), f.Close, nil
}

func parseLevelFrom(s string) (slog.Level, error) {
	var lv slog.Level
	if err := lv.UnmarshalText([]byte(s)); err != nil {
		return lv, fmt.Errorf("解析日志级别 %q: %w", s, err)
	}
	return lv, nil
}
