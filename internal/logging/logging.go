// Package logging 创建应用日志器。
package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// New 在 logDir 下以追加方式打开 devtoys.log。dev 为 true 时输出可读文本
// 并同时写 stderr，级别为 Debug；否则输出 JSON，级别为 Info。
//
// 这里有意不做日志轮转：桌面上单个日志文件足够小，轮转留给后续版本。
// 返回的关闭函数负责关闭文件。
func New(logDir string, dev bool) (*slog.Logger, func() error, error) {
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, nil, err
	}
	f, err := os.OpenFile(filepath.Join(logDir, "devtoys.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, err
	}

	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	var w io.Writer = f
	var handler slog.Handler
	if dev {
		opts.Level = slog.LevelDebug
		w = io.MultiWriter(f, os.Stderr)
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = slog.NewJSONHandler(w, opts)
	}
	return slog.New(handler), f.Close, nil
}
