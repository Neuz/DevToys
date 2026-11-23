package launcher

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Neuz/DevToys/internal/config"
	"github.com/Neuz/DevToys/internal/features/launcher/locales"
	"github.com/egoist/mygo"
	"github.com/mozillazg/go-pinyin"
)

// Entry 是索引里的一个可启动应用。Title 是系统显示的名称，Path 交给
// mygo.Shell 画图标与启动，ID 用作去重键并参与别名搜索。
type Entry struct {
	Title       string `json:"title"`
	ID          string `json:"id"`
	Path        string `json:"path"`
	Keywords    string `json:"keywords"`    // 标题 + 别名或分类补充词
	Description string `json:"description"` // 可执行文件名；打包应用为空
	Pinyin      string `json:"pinyin"`      // 标题的全拼与首字母，小写
	Category    string `json:"category"`    // app | system
}

// 条目的分类，对应面板里的【全部】【应用】【系统】。
const (
	categoryApp    = "app"
	categorySystem = "system"
)

// systemSetting 是一个内置的 Windows 设置页：标题本地化，Path 是 ms-settings URI。
type systemSetting struct {
	key      string // 标题的本地化 key
	uri      string // ms-settings URI，也用作 ID
	keywords string // 中英文补充关键词
}

// windowsSettings 是内置的 Windows 设置入口，覆盖最常用的一批设置页。
var windowsSettings = []systemSetting{
	{key: "system_settings_home", uri: "ms-settings:", keywords: "windows settings 设置 系统"},
	{key: "system_settings_display", uri: "ms-settings:display", keywords: "display screen brightness 显示 屏幕 亮度"},
	{key: "system_settings_sound", uri: "ms-settings:sound", keywords: "sound volume audio 声音 音量"},
	{key: "system_settings_notifications", uri: "ms-settings:notifications", keywords: "notification 通知"},
	{key: "system_settings_power", uri: "ms-settings:powersleep", keywords: "power sleep battery 电源 睡眠 电池"},
	{key: "system_settings_storage", uri: "ms-settings:storagesense", keywords: "storage disk 存储 磁盘"},
	{key: "system_settings_apps", uri: "ms-settings:appsfeatures", keywords: "apps applications 应用 程序"},
	{key: "system_settings_network", uri: "ms-settings:network", keywords: "network wifi internet 网络 无线"},
	{key: "system_settings_bluetooth", uri: "ms-settings:bluetooth", keywords: "bluetooth devices 蓝牙 设备"},
	{key: "system_settings_personalization", uri: "ms-settings:personalization", keywords: "personalization theme wallpaper 个性化 主题 壁纸"},
	{key: "system_settings_privacy", uri: "ms-settings:privacy", keywords: "privacy security 隐私 安全"},
	{key: "system_settings_update", uri: "ms-settings:windowsupdate", keywords: "windows update 更新"},
	{key: "system_settings_time", uri: "ms-settings:dateandtime", keywords: "date time timezone 日期 时间 时区"},
	{key: "system_settings_mouse", uri: "ms-settings:mousetouchpad", keywords: "mouse touchpad 鼠标 触摸板"},
	{key: "system_settings_keyboard", uri: "ms-settings:keyboard", keywords: "keyboard input 键盘 输入"},
}

// systemEntries 把内置的 Windows 设置页转换成索引条目。标题按当前语言读取，
// 中英文关键词都写进 Keywords，保证切换语言后仍可搜索。
func systemEntries() []Entry {
	out := make([]Entry, 0, len(windowsSettings))
	for _, s := range windowsSettings {
		title := locales.T(s.key)
		out = append(out, Entry{
			Title:    title,
			ID:       s.uri,
			Path:     s.uri,
			Keywords: title + " " + s.keywords,
			Category: categorySystem,
			Pinyin:   pinyinText(title),
		})
	}
	return out
}

// indexOptions 是启动器索引的私有配置：结果条数与排除词。
type indexOptions struct {
	MaxResults int      `json:"max_results"`
	Exclude    []string `json:"exclude"`
}

const (
	// defaultResultCount 是面板一次展示的默认结果条数。
	defaultResultCount = 8
	// minResultCount 与 maxResultCount 把配置里的条数夹在合理范围。
	minResultCount = 5
	maxResultCount = 30
)

// defaultIndexOptions 返回未配置时的索引选项。
func defaultIndexOptions() indexOptions {
	return indexOptions{
		MaxResults: defaultResultCount,
		Exclude: []string{
			"uninstall", "卸载", "readme", "help", "帮助", "说明", "文档",
		},
	}
}

// normalized 把缺失或越界的配置修正为安全值。
func (o indexOptions) normalized() indexOptions {
	def := defaultIndexOptions()
	if o.MaxResults == 0 {
		o.MaxResults = def.MaxResults
	}
	o.MaxResults = min(max(o.MaxResults, minResultCount), maxResultCount)
	if o.Exclude == nil {
		o.Exclude = def.Exclude
	}
	return o
}

// options 返回当前生效的索引配置；未初始化时使用默认值。
func (m *Module) options() indexOptions {
	if m.host == nil {
		return defaultIndexOptions()
	}
	return config.DecodeOptions("launcher", m.host.Feature(), defaultIndexOptions()).normalized()
}

// entriesFromApps 把已安装应用转换成索引条目，按 ID 去重并跳过没有路径的项。
func entriesFromApps(apps []mygo.InstalledApp) []Entry {
	seen := make(map[string]struct{}, len(apps))
	out := make([]Entry, 0, len(apps))
	for _, app := range apps {
		if app.Path == "" {
			continue
		}
		key := app.ID
		if key == "" {
			key = app.Path
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		title := strings.TrimSpace(app.Name)
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(app.Path), filepath.Ext(app.Path))
		}
		keywords := title
		if alias := appAlias(app); alias != "" && !strings.EqualFold(alias, title) {
			keywords = title + " " + alias
		}
		out = append(out, Entry{
			Title:       title,
			ID:          app.ID,
			Path:        app.Path,
			Keywords:    keywords,
			Description: appDescription(app),
			Pinyin:      pinyinText(title),
			Category:    categoryApp,
		})
	}
	return out
}

// appAlias 从 ID 里提炼可搜索的别名：桌面应用是可执行文件名（去掉扩展名），
// 打包应用是去掉 !App 之后的包标识。
func appAlias(app mygo.InstalledApp) string {
	id := strings.TrimSpace(app.ID)
	if id == "" {
		return ""
	}
	hasDir := false
	if i := strings.LastIndexAny(id, `\/`); i >= 0 {
		id = id[i+1:]
		hasDir = true
	}
	if i := strings.IndexByte(id, '!'); i >= 0 {
		id = id[:i]
	}
	// 只有桌面应用的 ID 是路径，才把 .exe 之类的扩展名去掉；打包应用的
	// 包标识里本来就有很多点，不能当扩展名切。
	if hasDir {
		id = strings.TrimSuffix(id, filepath.Ext(id))
	}
	return strings.TrimSpace(id)
}

// appDescription 返回一条简短描述：桌面应用是可执行文件名，打包应用留空，
// 由界面回退到「应用」。
func appDescription(app mygo.InstalledApp) string {
	if i := strings.LastIndexAny(app.ID, `\/`); i >= 0 {
		return strings.TrimSpace(app.ID[i+1:])
	}
	return ""
}

// pinyinArgs 把非中文字符原样保留，这样拉丁标题也能参与首字母匹配。
var pinyinArgs = func() pinyin.Args {
	args := pinyin.NewArgs()
	args.Style = pinyin.Normal
	args.Fallback = func(r rune, _ pinyin.Args) []string {
		return []string{string(r)}
	}
	return args
}()

// pinyinText 返回标题的全拼与首字母，均为小写，用空格分隔；两者相同时只留一份。
func pinyinText(title string) string {
	if title == "" {
		return ""
	}
	syllables := pinyin.Pinyin(title, pinyinArgs)
	var full, initials strings.Builder
	for _, syllable := range syllables {
		if len(syllable) == 0 {
			continue
		}
		part := syllable[0]
		first, _ := utf8.DecodeRuneInString(part)
		if unicode.IsSpace(first) {
			continue
		}
		full.WriteString(part)
		initials.WriteRune(first)
	}
	f := strings.ToLower(full.String())
	i := strings.ToLower(initials.String())
	switch {
	case f == "":
		return ""
	case f == i:
		return f
	default:
		return f + " " + i
	}
}

// applyExclude 去掉 Title 命中任一排除词的条目，匹配不区分大小写。
func applyExclude(entries []Entry, words []string) []Entry {
	if len(words) == 0 {
		return entries
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if !excluded(e.Title, words) {
			out = append(out, e)
		}
	}
	return out
}

// excluded 报告 title 是否包含任一排除词。
func excluded(title string, words []string) bool {
	lower := strings.ToLower(title)
	for _, w := range words {
		if w != "" && strings.Contains(lower, strings.ToLower(w)) {
			return true
		}
	}
	return false
}

// loadCachedEntries 在启动时同步读取缓存，先给面板一帧可用的结果；后台
// 随后会重新列出应用并替换。早期版本以 .lnk 为条目来源，没有 ID，这里
// 直接忽略，等后台刷新。
func (m *Module) loadCachedEntries() []Entry {
	if m.store == nil {
		return nil
	}
	cached := m.store.Entries()
	entries := make([]Entry, 0, len(cached))
	for _, e := range cached {
		if e.ID != "" {
			entries = append(entries, e)
		}
	}
	return applyExclude(entries, m.options().Exclude)
}

// rebuildIndex 在后台重建索引；完成后回到主线程替换内存并刷新面板。
//
// 这里不用 host.Tasks()：runner 关闭时会在主线程 wg.Wait，而后台结果又要
// 通过 RunOnMain 回到主线程，两者相遇会互相等待；索引没有必须完成的生命
// 周期，用独立 goroutine 更安全。
func (m *Module) rebuildIndex() {
	if m.host == nil {
		return
	}
	m.gen++
	gen := m.gen
	go func() {
		entries, err := m.buildIndex(context.Background())
		if err != nil {
			if m.log != nil {
				m.log.Debug("launcher: 重建索引失败", "error", err)
			}
			return
		}
		mygo.RunOnMain(func() { m.applyIndex(gen, entries) })
	}()
}

// buildIndex 列出已安装应用并刷新缓存。缓存只用于启动时的首帧显示，
// 后台总是重列，这样新装或卸载的应用也能反映出来。
func (m *Module) buildIndex(ctx context.Context) ([]Entry, error) {
	apps, err := mygo.Shell.InstalledApps()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries := entriesFromApps(apps)
	if runtime.GOOS == "windows" {
		entries = append(entries, systemEntries()...)
	}
	entries = applyExclude(entries, m.options().Exclude)
	if err := m.store.SaveIndex(entries); err != nil && m.log != nil {
		m.log.Debug("launcher: 写入索引", "error", err)
	}
	return entries, nil
}

// applyIndex 在主线程采用最新一次重建的结果，并让已创建的面板重绘。
func (m *Module) applyIndex(gen uint64, entries []Entry) {
	if gen != m.gen {
		return
	}
	m.entries = entries
	if m.panel != nil {
		m.panel.Invalidate()
	}
}
