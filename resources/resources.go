package resources

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/egoist/mygo/ui"
)

//go:embed *
var FS embed.FS

// AppIcon 返回应用图标，首次调用时解码，之后复用同一个位图。
func AppIcon() ui.ImageSource {
	return appIcon()
}

// AppIconPNG 返回应用图标的原始 PNG 数据，供只接收 []byte 的 API（如托盘图标）使用。
func AppIconPNG() ([]byte, error) {
	return FS.ReadFile("icon.png")
}

// appIcon 只解码一次：AppIcon 在视图每次重建时都会调用，
// 位图解码成本高，且 MyGo 会按位图缓存 GPU 纹理。
var appIcon = sync.OnceValue(func() ui.ImageSource {
	data, err := FS.ReadFile("icon.png")
	if err != nil {
		slog.Warn("读取应用图标失败", "error", err)
		return nil
	}
	icon, err := ui.DecodeBitmap(data)
	if err != nil {
		slog.Warn("解码应用图标失败", "error", err)
		return nil
	}
	return icon
})

// ExtractEmbedFile 把嵌入文件 embedPath 写到 destPath，并保留原文件权限。
func ExtractEmbedFile(embedPath, destPath string) error {
	return extractSingleFile(FS, embedPath, destPath)
}

// ExtractEmbedDir 把嵌入目录 embedDir 递归写到 destDir，并保留各文件权限。
func ExtractEmbedDir(embedDir, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("创建目标目录 %q 失败: %w", destDir, err)
	}
	return fs.WalkDir(FS, embedDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("遍历嵌入目录 %q 失败: %w", path, err)
		}

		// 剥掉 embedDir 前缀，得到相对路径；embedDir 本身被跳过。
		rel, err := filepath.Rel(embedDir, path)
		if err != nil {
			return fmt.Errorf("计算相对路径失败: %w", err)
		}
		if rel == "." {
			return nil
		}

		targetPath := filepath.Join(destDir, rel)
		if d.IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("创建目录 %q 失败: %w", targetPath, err)
			}
			return nil
		}
		return extractSingleFile(FS, path, targetPath)
	})
}

// extractSingleFile 把 fsys 中的 srcPath 复制到 destPath，保留权限；
// 写完后的 Close 错误也会返回，避免把没落盘的副本当成成功。
func extractSingleFile(fsys fs.FS, srcPath, destPath string) (err error) {
	parent := filepath.Dir(destPath)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("创建父目录 %q 失败: %w", parent, err)
	}

	srcFile, err := fsys.Open(srcPath)
	if err != nil {
		return fmt.Errorf("打开嵌入文件 %q 失败: %w", srcPath, err)
	}
	defer srcFile.Close()

	// 获取原文件信息以保留权限。
	info, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("获取嵌入文件状态失败: %w", err)
	}

	destFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("创建目标文件 %q 失败: %w", destPath, err)
	}
	defer func() {
		if cerr := destFile.Close(); err == nil && cerr != nil {
			err = fmt.Errorf("关闭目标文件 %q 失败: %w", destPath, cerr)
		}
	}()

	if _, err = io.Copy(destFile, srcFile); err != nil {
		return fmt.Errorf("写入目标文件 %q 失败: %w", destPath, err)
	}
	return nil
}
