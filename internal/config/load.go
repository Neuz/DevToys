package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Neuz/DevToys/internal/fsx"
	"github.com/egoist/mygo"
)

const fileName = "config.json"

// defaultDir 返回应用配置目录
func defaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, mygo.App.Name()), nil
}

func Load() (Config, error) {
	dir, err := defaultDir()
	if err != nil {
		return Default(), err
	}
	return LoadFrom(dir)
}

func (c Config) Save() error {
	dir, err := defaultDir()
	if err != nil {
		return err
	}
	return c.SaveTo(dir)
}

// LoadFrom 读取 dir 下的 config.json。文件不存在时返回 Default() 且 err 为 nil；
// JSON 损坏时把原文件改名为 config.json.bad 备份，记一条 warn，然后返回
// Default() 和 nil，避免一份坏配置让应用再也起不来。
func LoadFrom(dir string) (Config, error) {
	path := filepath.Join(dir, fileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Default(), err
	}
	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		backup := path + ".bad"
		if rerr := os.Rename(path, backup); rerr != nil {
			return Default(), fmt.Errorf("配置损坏且无法备份: %w", errors.Join(err, rerr))
		}
		slog.Warn("config: 配置损坏，已回退默认值", "path", path, "backup", backup, "error", err)
		return Default(), nil
	}
	return cfg.Normalize(), nil
}

// SaveTo 把配置原子地写入 dir/config.json。
func (c Config) SaveTo(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return fsx.WriteFile(filepath.Join(dir, fileName), data, 0o644)
}
