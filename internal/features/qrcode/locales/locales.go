package locales

import (
	"embed"
	"io/fs"

	"github.com/egoist/mygo/i18n"
)

//go:embed *.json
var files embed.FS

// Catalog 是QRCode模块的消息目录；注册后它的语言成为应用的语言。
var Catalog = mustCatalog(files, "zh-Hans")

func mustCatalog(fsys fs.FS, source string) *i18n.Catalog {
	c, err := i18n.NewCatalog(fsys, source)
	if err != nil {
		panic(err)
	}
	i18n.Register(c)
	return c
}

// T 用应用当前语言读出 key；无参数的消息由 i18n 缓存，重复调用不分配。
func T(key string) string { return Catalog.Format(i18n.Locale(), key, nil) }
