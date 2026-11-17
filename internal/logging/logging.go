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

// filename 是日志目录下的日志文件名。
const filename = "devtoys.log"

// New 创建应用日志器：始终追加写日志文件，开发模式或 debug 级别时同时写
// stderr。无法写日志文件时降级为只写 stderr，保证应用仍能启动；第二个
// 返回值用于关闭日志文件。
func New(cfg config.Config) (*slog.Logger, func() error) {
	lv := parseLevel(string(cfg.LogLevel))
	opts := &slog.HandlerOptions{Level: lv}

	f, err := openLogFile()
	if err != nil {
		logger := slog.New(slog.NewTextHandler(os.Stderr, opts))
		logger.Warn("logging: 无法写日志文件，改为只输出到 stderr", "error", err)
		return logger, func() error { return nil }
	}
	return slog.New(slog.NewTextHandler(output(f, lv), opts)), f.Close
}

// openLogFile 打开（必要时创建）日志文件。
func openLogFile() (*os.File, error) {
	dir, err := logDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建日志目录 %q: %w", dir, err)
	}
	f, err := os.OpenFile(filepath.Join(dir, filename), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("打开日志文件: %w", err)
	}
	return f, nil
}

// logDir 返回日志目录：用户配置目录下的应用名目录。
func logDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("获取用户配置目录: %w", err)
	}
	return filepath.Join(base, mygo.App.Name()), nil
}

// parseLevel 解析配置里的级别；空值或无效值回退到 info。
func parseLevel(s string) slog.Level {
	var lv slog.Level
	if err := lv.UnmarshalText([]byte(s)); err != nil {
		return slog.LevelInfo
	}
	return lv
}

// output 返回日志的写入目标：平时只写文件，开发模式或 debug 级别时同时写
// stderr，方便在终端查看。
func output(f *os.File, lv slog.Level) io.Writer {
	if mygo.IsDev() || lv <= slog.LevelDebug {
		return io.MultiWriter(f, os.Stderr)
	}
	return f
}
