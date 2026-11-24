# 架构设计

## 分层与依赖

```
cmd/devtoys   入口：单实例锁，调用 app.Run
   │
 app          装配层：配置、日志、模块列表、外壳、托盘
   │
 shell        窗口外壳：页面结构 / Sidebar / 标题栏 / 设置窗口 / 托盘 / 快捷键 / 生命周期
   │
 contract     外壳 ↔ 功能的唯一契约（接口）
   │
 features/*   功能页：welcome（示例）· launcher
   │
 task · config · logging · hotkey · fsx   基础设施
```

依赖只能自上而下。`app` 是唯一同时认识 `shell` 与具体 `features` 的地方；`features` 不 import `shell`，`shell` 不 import 任何具体功能。

## 目录结构

```
DevToys/
├── cmd/devtoys/main.go              入口：RequestSingleInstanceLock + app.Run
├── internal/
│   ├── app/app.go                   装配：配置、日志、模块列表、外壳、托盘
│   ├── shell/
│   │   ├── shell.go                 窗口、页面结构、模块切换、生命周期、host
│   │   ├── sidebar.go               侧栏导航、折叠、底部设置入口
│   │   ├── titlebar.go              自绘标题栏与玻璃材质
│   │   ├── settings.go              设置窗口（通用 / 快捷键）
│   │   ├── hotkey.go                全局快捷键注册与重绑
│   │   └── tray.go                  托盘图标与右键菜单
│   ├── contract/contract.go         Module / Live / Toggleable / GlobalHotkeyer / Closer / Host
│   ├── features/
│   │   ├── welcome/                 示例页：module / view / actions + locales
│   │   └── launcher/                全局启动器：module / index / search / store / panel / launch
│   ├── task/task.go                 后台任务 runner，结果经 post 回到主线程
│   ├── config/
│   │   ├── config.go                Config 模型、默认值、规范化、功能节点
│   │   └── load.go                  config.json 的读取、损坏回退、原子保存
│   ├── hotkey/hotkey.go             快捷键文本与 accelerator 互转
│   ├── logging/logging.go           slog 日志器（文本输出）
│   └── fsx/atomic.go                原子写文件（临时文件 + fsync + rename）
├── locales/                         外壳与公共文案：en.json、zh-Hans.json、catalog_gen.go（生成）
├── resources/                       应用图标与嵌入资源
├── docs/                            需求、架构、进度与功能文档
├── mygo.json                        mygo 应用清单：入口、i18n 目录、更新源
└── go.mod                           Go 1.27，mygo / mygoui / mygoicons
```

## 启动与退出

1. `cmd/devtoys` 取单实例锁，调用 `app.Run()`；启动失败时 `log.Fatal`。
2. `app.Run` 从 `os.UserConfigDir()/DevToys` 读 `config.json`：文件缺失时用默认值；JSON 损坏时把原文件备份为 `config.json.bad` 并回退默认值，保证应用仍能启动。
3. 初始化 `slog` 日志器，注册退出时关闭日志文件。
4. 读取应用图标，组装模块列表与 `shell.Options`（模块、窗口尺寸、`*config.Config`、托盘）。
5. `shell.Run` 先按配置应用主题（保证首帧正确），再注册回调（`WhenReady`、第二实例、全部窗口关闭、退出前清理）、装配 `mygo` 的 updater 插件，最后 `mygo.App.Run()` 进入事件循环。
6. `WhenReady`：应用语言；创建主窗口（`StateKey: "main"`，隐藏原生标题栏）；为每个模块建一个 `task.Runner` 并调用 `Init(host)`；注册主题与语言变化回调；有托盘时创建托盘；注册全局快捷键；激活默认页。
7. 退出时 `OnBeforeQuit` → `Shell.close()`：依次调用实现了 `contract.Closer` 的模块、关闭所有 runner（取消任务并等待 goroutine）、注销全局快捷键、销毁托盘。

第二实例启动时只触发 `OnSecondInstance`，把已有窗口显示到前台，不创建第二套界面。

## 契约

实现位置：[internal/contract/contract.go](../internal/contract/contract.go)

| 接口 / 类型 | 用途 | 实现方 |
| --- | --- | --- |
| `Module` | 主窗口功能页：`ID`、`Group`、`Title`、`Icon`、`Init(Host)`、`View(*ui.Context)` | `features/welcome`、`features/launcher` |
| `Live` | 可选：页面激活/离开钩子 `OnActivate` / `OnDeactivate` | 需要时由模块实现 |
| `Toggleable` | 可选：`Enabled()`，禁用后外壳跳过它的全局快捷键 | `features/launcher` |
| `GlobalHotkeyer` | 可选：`Hotkeys()` 声明 ID、默认 accelerator、描述与回调 | `features/launcher` |
| `Hotkey` | 快捷键声明：`ID`、`Default`、`Description`、`Run` | 由 `GlobalHotkeyer` 返回 |
| `Closer` | 可选：`Close()`，退出时释放窗口、数据库等资源 | `features/launcher` |
| `Host` | 外壳给功能的运行时设施：`Tasks`、`Log`、`Feature`、`SaveConfig`、`SetEnabled`、`HotkeysChanged` | `shell.host` |

`Host.Feature()` 返回该模块自己的 `config.Feature` 节点（`enabled`、`hotkeys`、`options`），功能只应改写自己的节点，并通过 `SaveConfig()` 落盘。

## 关键数据流

### 配置

- 模型与规范化：[config.go](../internal/config/config.go)。未知主题、语言、日志级别在读取时归一化为默认值。
- 读写：[load.go](../internal/config/load.go)。写入走 `fsx.WriteFile`（临时文件 + fsync + rename）。
- 设置窗口直接读改写 `shell.Options.Config`，每次修改立即 `Save()`；功能通过 `Host.Feature` / `Host.SaveConfig` 管理自己的节点。
- `Feature.UnmarshalJSON` 让省略 `enabled` 的节点默认启用，避免手写配置意外关掉功能。

### 主题与语言

- 主题：`applyTheme` 把 `config.ThemeMode` 映射到 `mygo.Theme.SetSource`；启动时在 `App.Run` 之前调用一次。mygoui 组件在每帧用 `theme.Apply(c)` 读取当前外观。
- 语言：`applyLocale` 把配置里的 `system` 映射为 mygo 的空字符串（跟随系统）。设置窗口切换后由外壳统一重绘主窗口与设置窗口，并刷新托盘菜单与设置窗口标题。

### 全局快捷键

[shell/hotkey.go](../internal/shell/hotkey.go) 的 `rebindHotkeys` 先注销旧绑定，再重建：

1. 外壳自己的 `hotkey.global.window_toggle`；
2. 每个实现了 `GlobalHotkeyer` 且处于启用状态的模块声明的全部快捷键。

模块快捷键的取值规则：

- 配置里**没有**该热键 ID → 使用模块声明的 `Default`；
- 键存在但值为空串 → 用户显式删除，不注册；
- 重复或注册失败 → 记一条 warn，返回空 accelerator，不影响其他绑定。

全局快捷键回调可能不在主线程，因此外壳与启动器都用 `mygo.RunOnMain` 切回主线程再操作窗口。

### 后台任务与线程模型

[task.go](../internal/task/task.go) 提供按 key 区分的后台任务：

- 所有 UI 状态只在主线程读写。
- `task.Go(runner, key, fn)` 在 goroutine 中执行；结果经 `Runner` 创建时传入的 `post`（主窗口是 `win.Update`）回到主线程。
- 同一 key 的新任务会取消旧任务，旧任务的结果被丢弃。
- `fn` 的 panic 被转成错误，避免一个功能 bug 带走整个应用。
- `Runner.Close` 取消全部任务并等待 goroutine 退出；每个模块拥有独立 runner，随外壳退出一起关闭。
- 功能在 `View` 的帧里读取 `Result` 的 `Running` / `Value` / `Err`，不要在后台 goroutine 中直接改 UI 状态。

### 启动器索引

启动器在 `Init` 中先同步读取 bbolt 缓存，随后用 `task.Go` 在后台重建索引，构建完成时把新索引写回缓存；结果在下一帧（设置页或面板）通过 `applyIndexResult` 采用。搜索与排序在主线程完成，数据规模是开始菜单应用量级。细节见 [启动器](features/launcher.md)。

## 新增一个工具页

1. 在 `internal/features/` 下建包，例如 `ping`：

```go
package ping

import (
	"log/slog"

	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Cyenoch/mygoui/theme"
	"github.com/Neuz/DevToys/internal/contract"
	"github.com/Neuz/DevToys/internal/features/ping/locales"
	"github.com/Neuz/DevToys/internal/task"
	"github.com/egoist/mygo/ui"
)

type Module struct {
	tasks *task.Runner
	log   *slog.Logger
}

func New() *Module { return &Module{} }

func (m *Module) ID() string    { return "ping" }
func (m *Module) Group() string { return "" } // 空分组，侧栏不显示分组标题
func (m *Module) Title() string { return locales.T("title") }
func (m *Module) Icon() *ui.SVG { return lucide.Activity() } // 返回 nil 时外壳用通用图标

func (m *Module) Init(h contract.Host) {
	m.tasks = h.Tasks()
	m.log = h.Log()
}

func (m *Module) View(c *ui.Context) {
	theme.Apply(c)
	// 后台任务用 task.Go(m.tasks, "ping", fn)，结果在帧里通过 Result 读取。
}
```

2. 在 [app.go](../internal/app/app.go) 的 `setupModules()` 里加一行：`ping.New()`。

3. 在 `internal/features/ping/locales/` 下放 `en.json`、`zh-Hans.json`，并照着欢迎页的 `locales.go` 写 `//go:embed`、`i18n.NewCatalog`、`i18n.Register` 和 `T(key)`。目录已按模块隔离，key 不需要加模块前缀。

侧栏条目、标题栏标题和页面切换都由外壳自动完成，启动时进入模块列表的第一个页面。长时间运行的工作请走 `task.Go`，不要直接在 `View` 里阻塞。

## 窗口与导航

- 页面结构由 [shell.go](../internal/shell/shell.go) 的 `view` 自行组装：
  - 顶层 `ui.Row`：左侧整列 `mygoui.Sidebar`（分组 + 图标 + 标题，自己管理宽度、背景与折叠动画，贯通窗口高度）；
  - 右侧 `ui.Column`：上层是绝对定位的自绘标题栏，下层是 `Grow(1)` 的 `mygoui.ScrollArea`；内容区顶部预留标题栏高度，超出高度时整体滚动，滚动位置由外壳跨帧保存。
- 分组名来自模块的 `Group()`，同组条目按注册顺序排列，分组顺序取首次出现的位置（`lo.Uniq` / `lo.GroupBy`）。
- 无原生标题栏：系统窗口按钮悬浮在页面上，标题栏按 `c.TitleBar()` 的结束侧 insets 留出空间，整条可拖动窗口；高度取「44 DIP、原生按钮带、按钮行高」三者最大值。
- 侧栏展开 180、折叠 48，均为 [sidebar.go](../internal/shell/sidebar.go) 中的常量，不写入配置。侧栏本身是拖动区域。
- 设置按钮打开独立的模态子窗口（[settings.go](../internal/shell/settings.go)）：顶部「通用 / 快捷键」两个选项卡；通用页包含外观、语言、开机启动与版本/检查更新；快捷键页包含外壳的显示/隐藏窗口快捷键。

## 国际化

- 根 [locales/](../locales) 放外壳与公共文案，由 `go tool mygo generate` 维护：改完 `zh-Hans.json` / `en.json` 后生成 `catalog_gen.go`，key 变成 `locales.SettingsTitle()` 这样的函数；`@` 描述取自基准语言 `zh-Hans.json`。
- `internal/features/<name>/locales/` 归各模块自己，JSON 与 `T(key)` 入口手写，`mygo generate` 不会覆盖或校验。
- 基准语言（source）是 `zh-Hans`，`en` 作为翻译；当前语言缺词时回退到基准语言。
- `T` 用 `Catalog.Format(i18n.Locale(), key, nil)` 取词，无参数消息由 i18n 缓存。

## 构建、发布与验证

```bash
go tool mygo dev      # 开发运行（热重载）
go tool mygo build    # 构建可执行文件与安装包
go tool mygo generate # 修改根 locales/*.json 后重新生成 catalog_gen.go
```

校验命令：

```bash
gofmt -l cmd internal locales resources
go vet ./...
go test ./...
go tool mygo i18n check .
```

发布流水线见 [.github/workflows/release.yml](../.github/workflows/release.yml)：推送 `v*` 标签后先跑格式、vet、测试与 i18n 检查，再创建草稿 Release，并用 `go tool mygo build -platform "windows/amd64,windows/arm64" -upload` 构建上传 Windows 安装包。
