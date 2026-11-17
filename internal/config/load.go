package config

import (
	"encoding/json"
	"errors"
	"io/fs"
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
// JSON 损坏时返回 Default() 和错误。
func LoadFrom(dir string) (Config, error) {
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
