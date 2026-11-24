package launcher

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Neuz/DevToys/internal/config"
	"github.com/Neuz/DevToys/internal/features/launcher/locales"
	"github.com/Neuz/DevToys/internal/task"
	"github.com/egoist/mygo"
	"github.com/mozillazg/go-pinyin"
)

// Entry 是索引里的一个可启动应用。Title 是系统显示的名称，Path 交给
// mygo.Shell 画图标与启动，ID 用作去重键；搜索只匹配 Keywords。
type Entry struct {
	Title       string `json:"title"`
	ID          string `json:"id"`
	Path        string `json:"path"`
	Keywords    string `json:"keywords"`    // 标题 + 别名 / exe 名 / 分类补充词 / 拼音
	Description string `json:"description"` // 可执行文件名；打包应用为空
	Category    string `json:"category"`    // app | system
}

// 条目的分类，对应面板里的【全部】【应用】【系统】。
const (
	categoryApp    = "app"
	categorySystem = "system"
)

// builtin 是写死在索引里的入口：settings 为 true 时 target 是 ms-settings URI，
// 否则 target 是 System32 下的可执行文件名。标题按当前语言读取，中英文补充
// 关键词都写进 Keywords，保证切换语言后仍可搜索。
type builtin struct {
	key      string // 标题的本地化 key
	target   string // ms-settings URI 或 System32 下的 exe
	keywords string // 中英文补充关键词
	settings bool   // true: 设置页；false: 系统程序
}

// builtins 是内置入口表：先列常用设置页，再列拿不到开始菜单快捷方式的程序。
var builtins = []builtin{
	// 设置首页与 Windows 安全中心不在这里：开始菜单已把它们作为应用索引，
	// 再加一份会出现同名重复项。
	{key: "system_settings_display", target: "ms-settings:display", settings: true, keywords: "display screen brightness 显示 屏幕 亮度"},
	{key: "system_settings_sound", target: "ms-settings:sound", settings: true, keywords: "sound volume audio 声音 音量"},
	{key: "system_settings_notifications", target: "ms-settings:notifications", settings: true, keywords: "notification 通知"},
	{key: "system_settings_power", target: "ms-settings:powersleep", settings: true, keywords: "power sleep battery 电源 睡眠 电池"},
	{key: "system_settings_storage", target: "ms-settings:storagesense", settings: true, keywords: "storage disk 存储 磁盘"},
	{key: "system_settings_apps", target: "ms-settings:appsfeatures", settings: true, keywords: "apps applications 应用 程序"},
	{key: "system_settings_network", target: "ms-settings:network", settings: true, keywords: "network wifi internet 网络 无线"},
	{key: "system_settings_bluetooth", target: "ms-settings:bluetooth", settings: true, keywords: "bluetooth devices 蓝牙 设备"},
	{key: "system_settings_personalization", target: "ms-settings:personalization", settings: true, keywords: "personalization theme wallpaper 个性化 主题 壁纸"},
	{key: "system_settings_privacy", target: "ms-settings:privacy", settings: true, keywords: "privacy security 隐私 安全"},
	{key: "system_settings_update", target: "ms-settings:windowsupdate", settings: true, keywords: "windows update 更新"},
	{key: "system_settings_time", target: "ms-settings:dateandtime", settings: true, keywords: "date time timezone 日期 时间 时区"},
	{key: "system_settings_mouse", target: "ms-settings:mousetouchpad", settings: true, keywords: "mouse touchpad 鼠标 触摸板"},
	{key: "system_settings_keyboard", target: "ms-settings:keyboard", settings: true, keywords: "keyboard input 键盘 输入"},
	{key: "system_settings_about", target: "ms-settings:about", settings: true, keywords: "about system info version 关于 系统 信息 版本"},
	{key: "system_settings_accounts", target: "ms-settings:accounts", settings: true, keywords: "accounts account 账户 账号"},
	{key: "system_settings_signin", target: "ms-settings:signinoptions", settings: true, keywords: "sign in login password pin 登录 密码"},
	{key: "system_settings_default_apps", target: "ms-settings:appsdefaults", settings: true, keywords: "default apps 默认 应用"},
	{key: "system_settings_startup", target: "ms-settings:startupapps", settings: true, keywords: "startup boot 启动 开机"},
	{key: "system_settings_optional", target: "ms-settings:optionalfeatures", settings: true, keywords: "optional features 可选 功能"},
	{key: "system_settings_taskbar", target: "ms-settings:taskbar", settings: true, keywords: "taskbar 任务栏"},
	{key: "system_settings_multitasking", target: "ms-settings:multitasking", settings: true, keywords: "multitasking snap 多任务 分屏"},
	{key: "system_settings_clipboard", target: "ms-settings:clipboard", settings: true, keywords: "clipboard 剪贴板"},
	{key: "system_settings_language", target: "ms-settings:regionlanguage", settings: true, keywords: "language region 语言 区域"},
	{key: "system_settings_fonts", target: "ms-settings:fonts", settings: true, keywords: "fonts 字体"},
	{key: "system_settings_recovery", target: "ms-settings:recovery", settings: true, keywords: "recovery reset 恢复 重置"},
	{key: "system_settings_activation", target: "ms-settings:activation", settings: true, keywords: "activation license 激活 许可证"},
	{key: "system_settings_troubleshoot", target: "ms-settings:troubleshoot", settings: true, keywords: "troubleshoot 疑难 解答 排除"},
	{key: "system_settings_developers", target: "ms-settings:developers", settings: true, keywords: "developer 开发者"},
	{key: "system_settings_accessibility", target: "ms-settings:easeofaccess", settings: true, keywords: "accessibility ease of access 辅助功能 无障碍"},
	{key: "system_settings_printers", target: "ms-settings:printers", settings: true, keywords: "printer scanner 打印机 扫描仪"},
	// mstsc 没有开始菜单快捷方式，InstalledApps 拿不到，只能按 exe 路径补。
	{key: "tool_remote_desktop", target: "mstsc.exe", keywords: "mstsc 远程桌面连接 remote desktop rdp"},
}

// builtinEntries 把内置入口转换成索引条目；程序 exe 不存在时跳过。
func builtinEntries() []Entry {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	out := make([]Entry, 0, len(builtins))
	for _, b := range builtins {
		title := locales.T(b.key)
		entry := Entry{
			Title:    title,
			ID:       b.target,
			Path:     b.target,
			Keywords: strings.TrimSpace(title + " " + b.keywords + " " + pinyinText(title)),
			Category: categorySystem,
		}
		if !b.settings {
			entry.Path = filepath.Join(root, "System32", b.target)
			if _, err := os.Stat(entry.Path); err != nil {
				continue
			}
			entry.ID = entry.Path
			entry.Description = b.target
			entry.Category = categoryApp
		}
		out = append(out, entry)
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

// normalized 把缺失或无效的配置修正为安全值。
func (o indexOptions) normalized() indexOptions {
	def := defaultIndexOptions()
	if o.MaxResults <= 0 {
		o.MaxResults = def.MaxResults
	}
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

// dedupeByTitle 去掉标题相同（不区分大小写）的条目，保留先出现的。
// 调用方把内置条目排在应用之前，因此 mstsc 会取代同名的 Store 应用。
func dedupeByTitle(entries []Entry) []Entry {
	seen := make(map[string]struct{}, len(entries))
	out := entries[:0]
	for _, e := range entries {
		key := strings.ToLower(strings.TrimSpace(e.Title))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, e)
	}
	return out
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
		alias := appAlias(app)
		if alias != "" && !strings.EqualFold(alias, title) {
			keywords += " " + alias
		}
		// 别名可能来自包标识，exe 名单独补一份，保证 mstsc.exe 能用 mstsc 搜到。
		if stem := executableStem(app); stem != "" &&
			!strings.EqualFold(stem, title) && !strings.EqualFold(stem, alias) {
			keywords += " " + stem
		}
		keywords = strings.TrimSpace(keywords + " " + pinyinText(title))
		out = append(out, Entry{
			Title:       title,
			ID:          app.ID,
			Path:        app.Path,
			Keywords:    keywords,
			Description: appDescription(app),
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
	// 只有桌面应用的 ID 是路径或裸文件名，才把 .exe 之类的扩展名去掉；
	// 打包应用的包标识里本来就有很多点，不能当扩展名切。
	if hasDir || strings.EqualFold(filepath.Ext(id), ".exe") {
		id = strings.TrimSuffix(id, filepath.Ext(id))
	}
	return strings.TrimSpace(id)
}

// executableStem 返回可执行文件名（去掉 .exe）：桌面应用的 ID 可能直接是
// mstsc.exe，也可能只在 Path 或显示名里出现；打包应用没有可执行文件名。
func executableStem(app mygo.InstalledApp) string {
	for _, candidate := range []string{app.ID, app.Path, app.Name} {
		name := strings.TrimSpace(candidate)
		if i := strings.LastIndexAny(name, `\/`); i >= 0 {
			name = name[i+1:]
		}
		if strings.EqualFold(filepath.Ext(name), ".exe") {
			return strings.TrimSuffix(name, filepath.Ext(name))
		}
	}
	return ""
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

// indexTaskKey 是索引重建任务在 Runner 上的 key；同 key 新任务会取消旧任务。
const indexTaskKey = "index"

// rebuildIndex 在后台重建索引，结果经 Runner 的 post 回到主线程，
// 面板在下一帧通过 applyIndexResult 采用。notify 表示完成后是否弹 Toast，
// 启动与配置变更触发的自动重建不需要提示。
//
// 注意 InstalledApps 不接受 context：退出时 Runner.Close 会等它返回，
// 因此退出可能短暂等待；post 是异步的，不会与主线程互相等待。
func (m *Module) rebuildIndex(notify bool) {
	if m.host == nil {
		return
	}
	m.index = task.Go(m.tasks, indexTaskKey, m.buildIndex)
	if notify {
		m.manualIndex = m.index
	}
}

// applyIndexResult 在主线程采用后台重建的结果；任务未完成或已采用过时跳过。
func (m *Module) applyIndexResult() {
	r := m.index
	if r == nil || r == m.appliedIndex || r.Running() {
		return
	}
	m.appliedIndex = r
	if err := r.Err(); err != nil {
		if m.log != nil {
			m.log.Debug("launcher: 重建索引失败", "error", err)
		}
		return
	}
	entries, ok := r.Value()
	if !ok {
		return
	}
	m.entries = entries
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
		// 内置条目排在应用前，同名时优先保留系统程序或设置页。
		entries = append(builtinEntries(), entries...)
	}
	entries = dedupeByTitle(entries)
	entries = applyExclude(entries, m.options().Exclude)
	if err := m.store.SaveIndex(entries); err != nil && m.log != nil {
		m.log.Debug("launcher: 写入索引", "error", err)
	}
	return entries, nil
}
