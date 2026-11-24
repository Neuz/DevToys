# 启动器：设置页增强、bbolt 索引与拼音模糊搜索

## 1. 背景

启动器（`internal/features/launcher`）已具备：全局快捷键唤起面板、扫描开始菜单/桌面 `.lnk`、JSON 文件缓存、按使用记录排序、启动条目。

本轮要补齐用户提出的两组需求：

1. 应用设置页
   - 1.1 快捷键允许删除（删除后**不再回退默认**）。
   - 1.2 增加【触发】行，用按钮手工唤起面板，样式与现有 `SettingsRow` 一致。
   - 1.3 增加索引信息（文件 path / size），并支持【重建索引】。
2. 启动器面板
   - 2.1 增加应用图标与应用描述（**图标本轮不做**，只做描述）。
   - 2.2 增加模糊搜索：标题 / 副标题 / 关键词 / 拼音（全拼 + 首字母）。

已确认的决策：

| 项 | 决策 |
| --- | --- |
| 索引存储 | bbolt（项目 `github.com/etcd-io/bbolt`，Go 模块路径为 `go.etcd.io/bbolt`） |
| 应用图标 | 本轮不做 |
| 别名 | 本轮不做 |
| 拼音 | `github.com/mozillazg/go-pinyin`（纯 Go，无 cgo） |
| 描述来源 | 快捷方式所在的来源与目录（如「开始菜单 · Windows 管理工具」），不解析 exe 版本信息 |

非目标：环境感知、UWP 应用、文件搜索、图标提取、别名、非 Windows 平台实现。

## 2. 数据模型

### 2.1 `Entry`（`internal/features/launcher/index.go`）

```go
type Entry struct {
    Title       string `json:"title"`
    ID          string `json:"id"`          // 去重键；旧缓存为空的条目会被忽略
    Path        string `json:"path"`        // 交给 mygo.Shell 画图标与启动
    Keywords    string `json:"keywords"`    // 标题 + 别名 / exe 名 / 补充词 / 拼音
    Description string `json:"description"` // 可执行文件名；打包应用为空
    Category    string `json:"category"`    // app | system
}
```

- 新增常量：`originStartMenu = "start_menu"`、`originDesktop = "desktop"`。
- `roots()` 改为返回 `[]source{dir, origin}`；新增 `scanSources([]source, *slog.Logger) []Entry` 承载现有的递归、去重与关键字逻辑。
- 保留 `scanRoots(dirs []string, *slog.Logger) []Entry` 作为 `originStartMenu` 的薄包装，避免破坏现有测试。
- 描述不落库为最终文案（多语言安全），渲染时由 `Source` + `Folder` 组合。

### 2.2 存储（新增 `internal/features/launcher/store.go`）

- 文件：`userData/launcher.db`（`mygo.App.Path(mygo.PathUserData)`）；打开前 `os.MkdirAll(dir, 0o755)`。
- `bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second})`。
- Bucket 结构：
  - `entries`：key = `Entry.Path`，value = `Entry` 的 JSON。
  - `usage`：key = `Entry.Path`，value = `Usage` 的 JSON。
  - `meta`：`roots` → `[]rootStamp` 的 JSON；`built_at` → UnixNano 文本。
- API：
  - `openStoreAt(path string) (*store, error)`（测试用）与 `openStore() (*store, error)`。
  - `(*store) Close() error`
  - `(*store) Entries() (entries []Entry, stamps []rootStamp)`
  - `(*store) SaveIndex(entries []Entry, stamps []rootStamp) error`（单事务内清空并重写）
  - `(*store) Usage() map[string]Usage`
  - `(*store) SaveUsage(map[string]Usage) error`
- 删除 `launcher-index.json` / `launcher-usage.json` 相关代码与测试（`indexCache`、`loadIndexCache`、`saveIndexCache`、`userDataFile` 保留后仅供 db 路径使用）。
- `Module` 新增字段 `store *store`；`Init` 中打开失败只记 warn 并降级为「无持久化」；实现 `contract.Closer`：`m.panel.Destroy()`（若已创建）与 `m.store.Close()`。

## 3. 设置页（`internal/features/launcher/view.go`、`module.go`）

设置页仍为「标题 + 副标题 + 一个 `SettingsGroup`」，行如下：

1. 启用开关（现有，不变）。
2. 触发快捷键（现有 `form.HotkeyInput`，`Clearable: true`，新增 `Placeholder: settings_hotkey_unset`）。
3. **触发**（新增）：`Icon: lucide.Play()`，`Title: settings_trigger`，Trailing 为 `mygoui.Button{Label: settings_trigger, Variant: ButtonOutline, Size: theme.ButtonSmall, Icon: lucide.Play(), OnClick: m.toggle}`。
4. **索引**（新增）：`Icon: lucide.Database()`，`Title: settings_index`，`Subtitle` 为 `<db 绝对路径> · <大小>`，Trailing 为 `mygoui.Button{Label: settings_rebuild, Variant: ButtonOutline, Size: theme.ButtonSmall, Icon: lucide.RefreshCw(), OnClick: func() { m.rebuildIndex(true) }}`。
5. **Toast**：按钮调用 `m.rebuildIndex(true)`，`rebuildIndex(notify bool)` 记录本次是否需要提示；`notifyIndexResult` 在帧里检测到结果完成后通过 `overlay.Toast` 弹一次卡片（成功 `ToastSuccess`，失败 `ToastDanger` 并带错误描述）；启动与配置变更触发的自动重建传 `false`，保持静默。`ToastState.View(c)` 安装在设置页末尾。

- 大小由 `os.Stat` 得到，`formatBytes(int64) string`：`< 1 KiB` 用 B，`< 1 MiB` 用 KB，否则 MB（1 位小数）。文件不存在时显示 `—`。
- 禁用时仍显示提示文案（现状保留）。

### 3.1 快捷键「彻底删除」

语义区分：

- 配置中**没有** `features.launcher.hotkeys.toggle` → 使用默认 `Alt+F`。
- 配置中**存在**该键且值为 `""` → 用户显式删除，**不注册**任何快捷键。

改动：

- `Module.accelerator()`：`acc, ok := node.Hotkeys["toggle"]; if !ok { return defaultHotkeyAccel }; return acc`。
- `Module.changeTrigger()`：格式化结果为空时写入 `node.Hotkeys["toggle"] = ""`（确保 map 非 nil），**不再**回填默认值；非空时写入 accelerator。
- `internal/shell/hotkey.go` 的 `rebindHotkeys()`：改为 `acc, ok := node.Hotkeys[h.ID]; if !ok { acc = h.Default }`。

## 4. 面板（`internal/features/launcher/panel_view.go`、`module.go`、`panel.go`）

`navigation.CommandItem` 没有描述字段，无法展示副标题，因此面板改为自绘：

- `Module.command navigation.CommandState` 替换为：
  ```go
  query string
  input ui.Handle
  list  list.ListBoxState
  ```
- `panelView`：
  - 根列保留 `c.Shortcut(0, ui.KeyEscape)`：查询为空时隐藏面板。
  - 首行：搜索图标 + `ui.TextInputBase(c, &m.query)`（`Label` 用 `title`，`Placeholder` 用 `panel_placeholder`，`Bind(&m.input)`），`OnTextKey(m.onTextKey)`。
  - 列表：`list.ListBox`，`Items` 来自 `m.visibleEntries()`，`Description` 为 `m.describe(e)`，`State: &m.list`，`Editor: &m.input`，`KeyboardHighlight: m.input.Focused(c)`，`NoFocusOnClick: true`，`OnAction: m.launchByPath`，`MaxHeight: 360`，`Empty: panel_placeholder`。
- `onTextKey(k ui.TextKey) bool`：
  - `Down`/`Up`（`Mods == 0`）：按当前文本计算的可见条数移动 `m.list.Highlight` 并 clamp。
  - `Enter`（`Mods == 0`）：按当前文本计算可见条目，启动 `Highlight` 位置（越界取第一条）；空列表直接吞掉。
  - `Escape`：查询非空时清空并返回 true；为空返回 false，交给根快捷键隐藏面板。
  - 其他按键返回 false。
  - 用 `k.Text()` 重算可见列表，避免输入事件领先于帧导致命中错误。
- `visibleEntries()`：`rank(filter(m.entries, m.query), m.usage, m.options().MaxResults)`。
- `visibleEntriesFor(text string)` 供 `onTextKey` 使用同一条路径。
- 构建时 clamp `m.list.Highlight`；`showPanel` 中清空 `m.query` 并把 `m.list.Highlight` 归零。
- `launchEntry` 逻辑不变；新增 `launchByPath(path string)` 供 `OnAction` 使用（按 Path 找条目）。

描述文案：`describe(e)` = `locales.T("source_"+e.Source)`；`Folder != ""` 时追加 `" · "` + `strings.ReplaceAll(e.Folder, string(filepath.Separator), " › ")`。

## 5. 拼音模糊搜索（`search.go`、`index.go`）

- 索引阶段为每个条目生成拼音，直接追加进 `Entry.Keywords`（不单独存字段）：
  ```go
  pinyin.Pinyin(title, args) → 逐音节拼接成全拼；取每音节首字母拼成首字母串
  ```
  全拼与首字母都转小写，用空格连接；非中文由 go-pinyin 原样返回（ASCII 标题因此天然支持首字母/缩写匹配）。
- `filter(entries, query)`：
  - 查询按空白切分为 token（小写）；无 token 返回全部。
  - 每个 token 都必须出现在 `strings.ToLower(e.Title + " " + e.Keywords)` 中（AND 语义）。
  - 空查询/空白查询行为不变。
- 匹配覆盖：标题、别名、exe 名、补充关键词、拼音全拼、拼音首字母，全部经 `Keywords` 参与。
- 排序仍由 `rank` 决定（使用次数 → 最近使用 → 标题长度 → 字典序），本轮不引入相关度打分。

## 6. 文案（`internal/features/launcher/locales/{zh-Hans,en}.json`）

新增 key：

| key | zh-Hans | en |
| --- | --- | --- |
| `settings_trigger` | 触发 | Trigger |
| `settings_index` | 索引 | Index |
| `settings_rebuild` | 重建索引 | Rebuild index |
| `toast_index_ok` | 索引重建完成 | Index rebuilt |
| `toast_index_failed` | 索引重建失败 | Index rebuild failed |
| `settings_hotkey_unset` | 未设置 | Not set |
| `source_start_menu` | 开始菜单 | Start menu |
| `source_desktop` | 桌面 | Desktop |

改完运行 `go tool mygo i18n check .`，保证中英条目一致。

## 7. 依赖

- `go get go.etcd.io/bbolt@latest`
- `go get github.com/mozillazg/go-pinyin@latest`

两者均为纯 Go、无 cgo，符合交叉编译要求。用户提到的 `github.com/etcd-io/bbolt` 是其 GitHub 仓库地址，实际模块路径为 `go.etcd.io/bbolt`。

## 8. 测试与验证

- `internal/features/launcher/index_test.go`
  - 现有 `scanRoots` 相关用例保持不变（包装器保证 `Keywords` 格式不变）。
  - 删除 `TestIndexCacheMtimeInvalidation`、`TestLoadIndexCacheMissingOrBroken`。
  - 新增 `TestScanSourcesRecordsOriginAndFolder`（`.lnk` 在子目录时 `Source`/`Folder` 正确）。
  - 新增 `TestPinyinKeywords`（中文标题含全拼与首字母）。
- 新增 `store_test.go`
  - 往返：`SaveIndex` → `Entries`；`SaveUsage` → `Usage`。
  - 不存在的 db：`Entries`/`Usage` 返回空且不 panic。
- `internal/features/launcher/search_test.go`
  - 新增拼音命中：`"jisuanqi"`、`"jsq"` 命中「计算器」。
  - 新增多 token AND：`"visual code"` 命中 `Visual Studio Code`。
  - 新增副标题命中：`folder` 参与匹配。
- 命令：
  ```
  go build ./...
  go vet ./...
  go test ./...
  go tool mygo i18n check .
  ```
  gofmt 由于仓库 CRLF，需复制到 `%TEMP%` 转 LF 后 `gofmt -l` 校验。

## 9. 风险与权衡

- 面板由 `navigation.Command` 改为自绘列表，键盘行为（上下/回车/Esc/IME）需要手测；实现按 `Command` 的既有约定复刻。
- 描述仅反映快捷方式所在目录，不解析 exe 元数据；若后续需要真实应用描述，再引入 `.lnk` 目标解析。
- 首次启动会从 JSON 迁移到 bbolt；旧 JSON 缓存文件不再读取（未随发布，无需兼容）。
- 索引重建通过 `task.Go(m.tasks, "index", buildIndex)` 执行，`buildIndex` 接受 `context.Context`；结果在主线程的面板帧里由 `applyIndexResult` 采用，`store` 关闭后写失败只记 debug。`InstalledApps` 不可取消，退出时 Runner 会短暂等待它返回。

## 10. 面板高度随结果自适应（FitContent 语义）

### 10.1 约束

`mygo.WindowOptions.FitContent` 只对网页（`Content: URL`）生效，注释明确 native UI 不适用。本面板使用 `Content: ui.View(...)`，因此改为自行计算内容高度并调用 `Window.SetSize`。

### 10.2 固定度量（`panel_view.go`）

新增常量（DIP，除特别说明外都随 `theme.TextScale(c)` 缩放）：

| 常量 | 值 | 说明 |
| --- | --- | --- |
| `panelInputHeight` | 40 | 搜索框固定高度，写入 `TextInputBase.Height` |
| `panelRowHeight` | 44 | 结果行固定高度（SM 20 + XS 16 + 行内边距 8），写入 `ListBoxItem.Options.Height` |
| `panelMaxRows` | 8 | 结果超过该行数后列表内部滚动，与 `defaultResultCount` 一致 |

新增两个函数：

```go
// panelHeightForScale 计算面板总高度：上下内边距 + 搜索框，有结果时再加间距与列表。
// rows 为 0 表示没有结果，此时只显示搜索框。scale 是 theme.TextScale(c)。
func panelHeightForScale(scale float32, rows int) int

// panelMaxListHeight 是列表展示 panelMaxRows 行时的最大高度。
func panelMaxListHeight(scale float32) float32
```

- `panelHeightForScale`：`height = 2*theme.Space(4) + panelInputHeight*scale`；仅当 `rows > 0` 时追加 `theme.Space(3) + 2*theme.Space(2) + min(rows, panelMaxRows)*panelRowHeight*scale`。结果向上取整后返回 `int`。
- `panelMaxListHeight`：`2*theme.Space(2) + panelMaxRows*panelRowHeight*scale`。
- `ListBox` 的行高按 `MinHeight(theme.Scale(c, Options.Height))` 计算，列表内边距使用传入的 `Padding` 原值（`theme.Space(2)` = 8），所以上式的度量和实际布局一致。

### 10.3 `panelView`

- 开头记录 `m.textScale = theme.TextScale(c)`。
- 搜索框追加 `.Height(float32(panelInputHeight) * scale)`。
- 每个 `list.ListBoxItem` 设置 `Options: list.ListBoxItemOptions{Height: panelRowHeight}`。
- `list.ListBoxOptions.MaxHeight` 改为 `panelMaxListHeight(scale)`（替换硬编码 360）。
- 计算 `entries` 后调用 `m.setPanelHeight(panelHeightForScale(scale, len(entries)))`，在构建阶段完成窗口高度同步。
- `entries` 为空（默认状态或无匹配）时只保留搜索框：先按 `panelHeightForScale(scale, 0)` 设置高度，再跳过列表渲染。列表不再设置 `Empty` 占位。

### 10.4 窗口（`panel.go`、`module.go`）

- 删除 `module.go` 中的 `panelHeight = 440` 常量，新增字段：
  - `panelHeight int`：面板当前高度（DIP），用于跳过重复 `SetSize`。
  - `textScale float32`：最近一帧的文字缩放，供 `showPanel` 在无 `ui.Context` 时估算高度。
- 新增 `(*Module) setPanelHeight(height int)`（`panel.go`）：`m.panel == nil`、`height <= 0` 或与 `m.panelHeight` 相同时直接返回；否则记录并 `m.panel.SetSize(panelWidth, height)`。
- `createPanel` 初始 `Height: panelHeightForScale(1, 0)`。
- `showPanel`：
  - `scale := m.textScale`，`<= 0` 时取 `1`；
  - `height := panelHeightForScale(scale, 0)`，先 `m.setPanelHeight(height)`；
  - 定位用 `height`：`y = area.Y + (area.Height-height)/2 - area.Height/5`；拿不到工作区时 `Center()` 后上移 `height/5`；
  - **顶部位置固定**：后续结果变化只改高度，不重算 `y`，面板向下生长，输入框不跳动。
- 清除查询后首帧即按默认高度（仅搜索框）显示；输入过程中高度随结果条数增减，超过 `panelMaxRows` 后列表内部滚动。

### 10.5 验证

- 无结果（空查询或无匹配）：面板高度约 72 DIP，只有搜索框，不渲染列表与占位提示。
- 1 条结果 ≈ 144 DIP，每多一条增加 44 DIP，8 条封顶（≈ 452 DIP，`panelMaxListHeight` = 368）。
- 删除查询文本后面板收缩回默认高度；顶部位置保持不变。
- `go build ./...`、`go vet ./...`、`go test ./...`、`go tool mygo i18n check .`，以及 CRLF→LF 副本 `gofmt -l` 校验。

## 11. 设置页暴露索引选项

`indexOptions`（结果条数与排除词）不再是内部常量，改由设置页编辑并写回 `config.json` 的 `features.launcher.options`。

### 11.1 设置页新增两行（`view.go`）

| 行 | 控件 | 文案 key |
| --- | --- | --- |
| 显示结果数量 | `form.NumberField`，宽度 `theme.Scale(c, 120)`；`0` 或负数按默认值 8 处理 | `settings_max_results` |
| 排除关键词 | `form.TagsInput`（逗号或回车分隔） | `settings_exclude` |

两行都插在【触发快捷键】之后、【触发】之前，`OnChange` 均为 `m.changeOptions`。

### 11.2 状态与保存（`module.go`）

- 新增字段：`maxResults form.NumberFieldState`、`excludeTags []string`。
- `Init` 在 `m.trigger` 之后初始化：`m.maxResults.Value = float64(opts.MaxResults)`、`m.excludeTags = append([]string(nil), opts.Exclude...)`。
- 新增 `(*Module) changeOptions()`：
  - 读取 `current := m.options()`，用当前控件值构造 `indexOptions{MaxResults: int(m.maxResults.Value), Exclude: m.excludeTags}.normalized()`；
  - `json.Marshal` 后写入 `m.host.Feature().Options`，`SaveConfig` 失败则回滚 `Options` 并记 error；
  - 仅当 `opts.Exclude` 与 `current.Exclude` 不同（`slices.Equal`）时调用 `m.rebuildIndex(false)`，因为排除在建索引时应用；结果条数只影响查询，无需重建。

### 11.3 生效范围

- `m.options()` 每次从配置解码，`visibleEntries()` 立即采用新的 `MaxResults`。
- 排除词变化触发后台重建索引，完成后 `applyIndex` 刷新内存与面板。

### 11.4 验证

- 设置页出现两行；NumberField 初值为当前配置（默认 8），不设上下限；TagsInput 初值为默认排除词。
- 修改数量后立即影响面板结果条数；修改排除词后索引在后台重建，命中词的应用从结果消失；`config.json` 的 `features.launcher.options` 同步更新。
- 构建、vet、测试、i18n 与 `gofmt` 校验通过。

## 12. 结果分类与 Windows 设置

面板结果分【全部】【应用】【系统】三类，并把常用 Windows 设置页加入索引。

### 12.1 数据模型（`index.go`）

- `Entry` 新增 `Category string`（`json:"category"`），取值为 `categoryApp = "app"` 或 `categorySystem = "system"`。
- `entriesFromApps` 全部标记为 `categoryApp`；旧缓存没有该字段时按应用处理（`categoryOf`）。
- 新增 `builtins` 表（31 项设置 + `mstsc`）：每项 `{本地化 key, target, 中英文补充关键词, settings}`，设置项的 `target` 是 `ms-settings:` URI，程序项的 `target` 是 System32 下的 exe 文件名。
- 设置项覆盖显示、声音、通知、电源、存储、应用、网络、蓝牙、个性化、隐私、更新、时间、鼠标、键盘、关于、账户、登录选项、默认应用、启动应用、可选功能、任务栏、多任务、剪贴板、语言和区域、字体、恢复、激活、疑难解答、开发者选项、辅助功能、打印机和扫描仪；设置首页与 Windows 安全中心已由开始菜单索引，不再重复收录。
- 新增 `builtinEntries() []Entry`：设置项 `ID` 与 `Path` 都是 URI、分类为系统；程序项把 `target` 解析为完整 exe 路径（不存在则跳过），分类为应用，`Description` 为 exe 文件名；标题、别名与拼音统一拼接进 `Keywords`。
- `buildIndex` 在 `runtime.GOOS == "windows"` 时把内置条目排到应用之前并合并，再用 `dedupeByTitle` 按标题去重（内置优先），最后统一按排除词过滤；因此 `mstsc` 会取代同名的 Store「远程桌面连接」应用。

### 12.2 过滤（`search.go`）

- `categoryOf(e)`：空分类视为 `categoryApp`。
- `filterCategory(entries, category)`：`category == ""` 返回全部，否则只保留同分类。
- `visibleEntriesFor` 链式变为 `rank(filter(filterCategory(m.entries, m.category), text), ...)`。

### 12.3 面板（`panel_view.go`、`panel.go`、`module.go`）

- `Module.category string`（空 = 全部），`showPanel` 时重置为空。
- 空查询仍只显示搜索框；有查询时在搜索框下方用 `navigation.SegmentedControl` 渲染【全部】【应用】【系统】，`Selected` 由 `m.category` 推导，`OnChange` 写入分类并重置高亮。
- 当前分类无结果时保留搜索框与分类控件，不渲染列表。
- 高度公式新增可选分类行：`panelHeightForScale(scale, rows, tabs)`，`tabs` 为真时加 `theme.Space(3) + panelTabsHeight*scale`（`panelTabsHeight = 34`）。`createPanel` / `showPanel` 传 `tabs = false`。
- 系统条目行图标用 `lucide.Settings2()`（不走 `FileIcon`），描述固定为 `description_system`。
- `launchEntry`：系统条目走 `mygo.Shell.OpenExternal(e.Path)`，应用条目仍走 `Shell.LaunchApp`；成功后才记使用记录。

### 12.4 文案

`panel_category`、`panel_category_all`、`panel_category_apps`、`panel_category_system`、`description_system`，以及 15 个 `system_settings_*` 标题（中英同步）。

### 12.5 验证

- 输入任意关键词后出现三档分类；切换【系统】只剩 Windows 设置项，切换【应用】只剩应用。
- 选择系统条目后打开对应 `ms-settings:` 页面，并计入使用记录。
- 旧索引缓存（无 `category`）在重建前仍出现在【应用】里，重建后系统项出现。
- 新增单测：`filterCategory`、`builtinEntries`、`entriesFromApps` 的分类标记；构建、vet、测试与 `gofmt` 通过。
