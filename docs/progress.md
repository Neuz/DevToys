# 开发进度

> 更新于 2026-11-24，对应版本 `0.0.1`（见 [mygo.json](../mygo.json)）。

## 已完成

### 工程骨架

- [x] 分层与依赖约束：`cmd → app → shell → contract ← features`，功能与外壳互不 import。
- [x] `contract` 契约：`Module` / `Live` / `Toggleable` / `GlobalHotkeyer` / `Closer` / `Host`。
- [x] 编译期模块注册，新增工具页只需新建包并在 `setupModules()` 注册。
- [x] 单实例锁与第二实例唤起主窗口。
- [x] 后台任务 runner：按 key 顶替旧任务、结果回主线程、panic 转错误、退出时取消并等待。

### 外壳与系统集成

- [x] 主窗口、自绘标题栏（玻璃材质、结束侧 insets 让位、随文字缩放）。
- [x] 侧栏导航：分组、展开/折叠、底部设置入口、拖动窗口。
- [x] 设置窗口：通用（外观 / 语言 / 开机启动 / 版本与检查更新）、快捷键与网络三个选项卡。
- [x] 网络代理：`direct` / `system` / `manual` 三种模式、代理例外、合并的【测试并保存】（TCP 可达才保存，失败回退系统代理），启动时安装到进程内的 HTTP transport。
- [x] 主题与语言运行时切换并持久化。
- [x] 托盘：显示/隐藏、检查更新、退出；有托盘时关闭窗口仅隐藏。
- [x] 全局快捷键注册、重绑、重复与失败处理。
- [x] 配置读写：默认值、非法值归一化、损坏备份回退、原子写入。
- [x] 日志：slog 文本输出、开发/debug 双写 stderr、不可写时降级。
- [x] 根文案目录生成（`go tool mygo generate`）与功能级文案隔离。

### 功能页

- [x] `welcome`：演示模块、后台任务与日志用法（示例，不算产品功能）。
- [x] `launcher`：快捷键唤起、应用与系统设置索引、多词/拼音搜索、使用频率排序、bbolt 持久化、分类面板、设置页与 Toast。
- [x] 启动器索引与搜索的单元测试（`index_test.go`、`search_test.go`、`store_test.go`）。

### 交付

- [x] `.github/workflows/release.yml`：标签触发格式、vet、测试、i18n 检查，构建并上传 Windows 安装包。

## 进行中

当前没有已启动但未完成的功能开发。下一次迭代的候选见「待办」。

## 待办

### 近期

- [ ] 为 `internal/shell` 补回归测试：用 `ui.NewTester` 渲染外壳，覆盖导航切换、主题/语言刷新、快捷键注册与重绑。
- [ ] 启动器非 Windows 平台的内置条目（或明确保持 Windows-only 并在界面说明）。
- [ ] 启动器索引重建的进度/耗时反馈（目前只有完成后的 Toast）。
- [x] 划词翻译首版：Windows UI Automation 取词 + 启动器风格浮窗 + Google / 哔哩哔哩多服务，配置放在 `features.translate`；默认只启用哔哩哔哩，不做自动划词与 OCR。面板支持拖动、固定、结果滚动、单卡片折叠与源/目标语言互换。已通过 `go build ./...`、`go vet ./internal/...`、`go test ./...`、`go tool mygo i18n check .`。
- [ ] 新增实用工具页（在现有契约下逐个添加）。

### 后续候选（当前为非目标）

- [ ] 配置迁移：把旧的 `hotkey_global` / `hotkey_sidebar` 一次性转换为新结构（当前直接忽略）。
- [ ] 日志轮转与保留策略（当前持续追加）。
- [ ] 工具搜索 / 命令面板。

## 已知问题

| 位置 | 问题 | 影响 | 计划 |
| --- | --- | --- | --- |
| `internal/shell` | 没有测试文件 | 外壳回归依赖手动验证 | 用 `ui.NewTester` 补测试 |
| [launcher/index.go](../internal/features/launcher/index.go) | `mygo.Shell.InstalledApps()` 不接受 `context` | 退出时 `Runner.Close` 可能等待枚举返回；`post` 是异步的，不会与主线程互锁 | 等上游支持后传入 ctx |
| [launcher/index.go](../internal/features/launcher/index.go) | 内置设置页与 `mstsc` 仅在 Windows 加入 | 其他平台只能搜索开始菜单应用 | 保持 Windows-only；需要时再实现 |
| [launcher/index.go](../internal/features/launcher/index.go) | 按标题去重（不区分大小写） | 同名应用只保留一个，内置条目优先 | 有意的设计取舍 |
| [launcher/index.go](../internal/features/launcher/index.go) | 旧缓存中没有 `ID` 的条目会被忽略 | 升级后首帧结果可能偏少，后台重建后自动补全 | 无需处理 |
| [launcher/module.go](../internal/features/launcher/module.go) | 索引结果只在设置页或面板的帧里采用 | 重建完成时若两个界面都未打开，结果延后到下次进入生效 | 如需即时生效可改为在主线程回调中更新 |
| [launcher/panel_view.go](../internal/features/launcher/panel_view.go) | 图标按路径缓存且不失效 | 应用图标变化后需重启才能刷新 | 可增加失效或按修改时间重建 |
| [config/config.go](../internal/config/config.go) | 旧扁平快捷键键不再读取 | 升级后需在设置窗口重新录制 | 列为非目标；如需兼容再做一次性迁移 |
