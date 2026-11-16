package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Neuz/DevToys/internal/fsx"
)

const fileName = "config.json"

// DefaultDir 返回应用配置目录：os.UserConfigDir()/appName。
func DefaultDir(appName string) (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appName), nil
}

// Load 读取 dir 下的 config.json。文件不存在时返回 Default() 且 err 为 nil；
// JSON 损坏时返回 Default() 和错误。
func Load(dir string) (Config, error) {
	data, err := os.ReadFile(filepath.Join(dir, fileName))
	if errors.Is(err, fs.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Default(), err
	}
	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), err
	}
	return cfg.Normalize(), nil
}

// Save 把配置原子地写入 dir/config.json。
func (c Config) Save(dir string) error {
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
