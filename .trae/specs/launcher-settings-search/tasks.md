# 任务清单：启动器设置页、bbolt 索引与拼音搜索

> 依赖与顺序：T1 先落依赖 → T2/T3 存储与索引 → T4 搜索 → T5 设置页 → T6 面板 → T7 文案 → T8 验证。

## T1 引入依赖

- [x] `go get go.etcd.io/bbolt@latest`
- [x] `go get github.com/mozillazg/go-pinyin@latest`
- [x] 确认 `go.mod` 只新增这两项（及必要的间接依赖），不升级 mygo / mygoui / mygoicons。

验收：`go build ./...` 通过。

## T2 快捷键「彻底删除」

涉及：`internal/features/launcher/module.go`、`internal/shell/hotkey.go`

- [x] `Module.accelerator()` 改为按 key 是否存在取值：缺 key → `defaultHotkeyAccel`；存在但为空 → `""`。
- [x] `Module.changeTrigger()` 清空时写入 `Hotkeys["toggle"] = ""`，不再回填默认值，也不重置 `m.trigger`。
- [x] `rebindHotkeys()` 使用 `acc, ok := node.Hotkeys[h.ID]`；`!ok` 才回退 `h.Default`。

验收：删除快捷键后重启，仍无绑定；手动把 config 里的 `toggle` 移除后恢复 `Alt+F`。

## T3 bbolt 存储

涉及：新增 `internal/features/launcher/store.go`、`store_test.go`；改 `index.go`、`search.go`、`module.go`

- [x] 新增 `store` 类型与 bucket：`entries` / `usage` / `meta`（`roots`、`built_at`）。
- [x] `openStoreAt`（测试）与 `openStore`（用户数据目录 + `MkdirAll`），`bolt.Open` 超时 1s、权限 0600。
- [x] `Entries` / `SaveIndex` / `Usage` / `SaveUsage` / `Close`。
- [x] `Module` 增加 `store` 字段，`Init` 中打开；实现 `Close()`（销毁面板 + 关闭 db）。
- [x] `buildIndex` 改为写 bbolt；`loadCachedEntries` 改为读 bbolt 并校验 `roots` 时间戳。
- [x] `loadUsage` / `saveUsage` 改为走 bbolt。
- [x] 删除 JSON 缓存相关代码与用例。

验收：`go test ./internal/features/launcher` 通过；`%APPDATA%/.../launcher.db` 生成，无 `launcher-*.json`。

## T4 索引条目扩展与拼音

涉及：`index.go`、新增/改 `index_test.go`

- [x] `Entry` 增加 `Source`、`Folder`、`Pinyin`。
- [x] `roots()` 返回 `[]source`；新增 `scanSources`；`scanRoots` 保留为包装器。
- [x] `scanDir` 记录来源与相对目录；`Keywords` 保持「标题 + 相对目录」。
- [x] 新增 `pinyinText(title string) string`（全拼 + 首字母，小写）。
- [x] 新增 `source` 相关常量。

验收：子目录快捷方式的 `Source`/`Folder` 正确；中文标题的 `Pinyin` 同时含全拼与首字母。

## T5 搜索（拼音模糊 + 多词）

涉及：`search.go`、`search_test.go`

- [x] `filter` 按空白切 token，AND 匹配 `Title + Keywords + Pinyin`（小写）。
- [x] 保持空查询返回全部、大小写不敏感。
- [x] 新增拼音全拼/首字母、多 token、副标题命中用例。

验收：输入 `jisuanqi`、`jsq` 均能命中「计算器」；`visual code` 命中 `Visual Studio Code`。

## T6 设置页新增两行

涉及：`view.go`、`module.go`、`panel.go`

- [x] 触发快捷键行补 `Placeholder: settings_hotkey_unset`。
- [x] 新增【触发】行，按钮调用 `m.toggle`。
- [x] 新增【索引】行，Subtitle 显示 `launcher.db` 路径与大小，按钮调用 `m.rebuildIndex`。
- [x] 新增 `indexSummary()`、`formatBytes(int64)`。
- [x] 面板状态改为 `query` / `input` / `list`，移除 `navigation.CommandState` 依赖。

验收：设置页三行显示正常，按钮可用；禁用态提示仍在。

## T7 面板自绘列表（标题 + 描述）

涉及：`view.go`、`panel.go`

- [x] `panelView` 用 `TextInputBase` + `list.ListBox` 替换 `navigation.Command`。
- [x] `onTextKey` 处理 Down/Up/Enter/Escape，用 `k.Text()` 重算可见列表。
- [x] `describe(e)` 输出「来源 · 目录」，目录分隔符替换为 ` › `。
- [x] `showPanel` 清空查询并重置高亮；构建时 clamp 高亮。
- [x] `launchByPath` 供 `OnAction` 使用。

验收：面板每行显示标题与描述；上下键/回车/Esc/点击行为与旧版一致。

## T8 文案与文档

涉及：`internal/features/launcher/locales/{zh-Hans,en}.json`、`README.md`

- [x] 新增 7 个 key（见 spec 第 6 节）。
- [x] `go tool mygo i18n check .` 通过（启动器 15 条、根目录仍 18 条）。
- [x] README 补启动器章节：快捷键删除语义、索引位置与重建、拼音搜索。

验收：i18n check 无差异；README 与实现一致。

## T9 全量验证

- [x] `go build ./...`
- [x] `go vet ./...`
- [x] `go test ./...`
- [x] `go tool mygo i18n check .`
- [x] CRLF→LF 副本 `gofmt -l` 无输出
- [x] 手测（`go tool mygo dev`）：Alt+F 唤起、拼音搜索、回车启动、Esc 隐藏、删除快捷键后不再响应、重建索引入口

## T10 图标、别名与 InstalledApps 索引

- [x] 索引改用 `mygo.Shell.InstalledApps()`，覆盖桌面应用与打包应用，按 ID 去重。
- [x] `Entry` 改为 `Title / ID / Path / Keywords / Description / Pinyin`，删除目录扫描与根目录时间戳。
- [x] 别名：桌面应用取可执行文件名（去扩展名），打包应用取 `!App` 前的包标识，参与搜索。
- [x] 描述：桌面应用显示可执行文件名，打包应用回退到 `description_app` 文案。
- [x] 图标：`mygo.Shell.FileIcon` 按显示缩放取 PNG，`ui.DecodeBitmap` 解码后按 Path 缓存；无图标时用 `ui.Avatar`。
- [x] 启动改走 `mygo.Shell.LaunchApp`，删除 `launch_*.go`。
- [x] 旧缓存（无 ID 的 `.lnk` 条目）读取时忽略，等后台刷新。
- [x] 单测、README、locales 同步更新；build/vet/test/i18n/gofmt 全部通过。
- [x] 面板打开时不展示备选项：空查询不展示任何条目，输入后才出现结果。

## T11 面板高度随结果自适应

涉及：`view.go`、`panel.go`、`module.go`

- [x] `module.go` 删除 `panelHeight = 440`，新增字段 `panelHeight int`、`textScale float32`。
- [x] `view.go` 新增常量 `panelInputHeight = 40`、`panelRowHeight = 44`、`panelMaxRows = 8`。
- [x] `view.go` 新增 `panelHeightForScale(scale, rows)` 与 `panelMaxListHeight(scale)`（公式见 spec 10.2）。
- [x] `panelView`：记录 `m.textScale`；搜索框 `.Height(panelInputHeight*scale)`；行 `Options.Height = panelRowHeight`；`MaxHeight` 改用 `panelMaxListHeight(scale)`；构建末调用 `m.setPanelHeight(...)`。
- [x] `panel.go` 新增 `setPanelHeight`（同高度跳过），`createPanel` 初始高度用空状态值，`showPanel` 先设高度再按该高度定位。
- [x] 验证：空查询约 136 DIP；每条结果 +44 DIP（1 条约 144、8 条约 452）；8 条封顶后内部滚动；清空查询后收缩且顶部位置不变（按公式与布局度量核对，GUI 手测由用户确认）。
- [x] `go build ./...`、`go vet ./...`、`go test ./...`、`go tool mygo i18n check .`、CRLF→LF 副本 `gofmt -l` 全部通过。

## T12 无结果时不渲染列表

涉及：`view.go`、`panel.go`、`locales/{zh-Hans,en}.json`

- [x] `panelHeightForScale` 简化为：`rows == 0` 时只算内边距 + 搜索框；`rows > 0` 才加间距与列表。
- [x] `panelView`：`entries` 为空（空查询或无匹配）时只保留搜索框并跳过列表渲染。
- [x] 移除 `ListBoxOptions.Empty`，删除 `panel_empty` 文案（中英）。
- [x] `createPanel` 与 `showPanel` 的初始/默认高度使用 `panelHeightForScale(scale, 0)`。
- [x] 验证：无结果时高度约 72 DIP；build/vet/test/i18n/gofmt 通过。

## T13 设置页暴露索引选项

涉及：`module.go`、`view.go`、`locales/{zh-Hans,en}.json`、`README.md`

- [x] `Module` 新增 `maxResults form.NumberFieldState`、`excludeTags []string`，`Init` 用 `m.options()` 初始化。
- [x] 设置页新增【显示结果数量】（`form.NumberField`，不设上下限，`0` 用默认值）与【排除关键词】（`form.TagsInput`）。
- [x] 新增 `changeOptions()`：把值编码进 `features.launcher.options` 并保存，失败回滚。
- [x] 排除词变化（`slices.Equal` 比较）后调用 `rebuildIndex()`；结果条数变化即时生效。
- [x] 新增 4 个文案 key，README 补充设置项与 `features.launcher.options` 说明。
- [x] 验证：build/vet/test/i18n/gofmt 通过。

## T14 结果分类与 Windows 设置

涉及：`index.go`、`search.go`、`view.go`、`module.go`、`panel.go`、locales、测试

- [x] `Entry` 新增 `Category`；`entriesFromApps` 标记为 `app`。
- [x] 新增 `windowsSettings`（15 项 `ms-settings:`）与 `systemEntries()`；`buildIndex` 在 Windows 上并入。
- [x] `search.go` 新增 `categoryOf` / `filterCategory`，`visibleEntriesFor` 先按分类过滤。
- [x] 面板在有查询时渲染【全部】【应用】【系统】分段控件，空查询仍只有搜索框；分类无结果时保留切换。
- [x] `panelHeightForScale` 增加 `tabs` 参数（`panelTabsHeight = 34`），`createPanel`/`showPanel` 传 false。
- [x] 系统条目用 `lucide.Settings2()` 作图标、`description_system` 作描述，启动走 `mygo.Shell.OpenExternal`。
- [x] 新增 20 个文案 key（中英同步）与 3 个单测；README、spec 更新。
- [x] 验证：build/vet/test/i18n/gofmt 通过。
