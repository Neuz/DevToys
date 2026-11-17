# DevToys

用 Go 写的桌面开发工具箱骨架：原生窗口（[mygo](https://github.com/egoist/mygo)）+ 组件库（[mygoui](https://github.com/Cyenoch/mygoui)），左侧 Sidebar 导航，自绘标题栏，常驻托盘，主题与界面语言都可在运行时切换。

这不是一个成品工具集，而是一套"功能会持续增加"的工程骨架：新增一个工具页只需要新建一个目录并在装配处注册，外壳不用改。

## 设计目标

- **外壳稳定，功能可插拔**：`shell` 负责窗口、导航、标题栏、设置窗口、托盘；`app` 负责把外观与语言作用到运行时；`features/*` 只画自己的页面。
- **单向依赖**：`features` 不 import `shell`，`shell` 不 import `features`，两边只认 `contract`；装配在 `app` 完成。
- **界面状态只有一个来源**：当前页与侧栏折叠状态归外壳持有，侧栏宽度是 `sidebar.go` 里的常量；主题、语言与全局快捷键以 `config` 为准，由设置窗口直接读写，外壳负责应用，功能只读。
- **并发结果统一回主线程**：所有 UI 状态只在主线程读写，后台 goroutine 通过 `task.Runner` 的结果回到帧里。

## 分层

```
cmd/devtoys           入口：单实例锁，调用 app.Run
   │
 app                  装配层：配置、日志、模块列表、外壳、托盘
   │
 shell                窗口外壳：页面结构 / Sidebar / 标题栏 / 设置窗口 / 托盘 / 生命周期
   │
 contract             外壳 ↔ 功能的唯一契约（接口）
   │
 features/*           功能页：welcome
   │
 task · config · logging · fsx   基础设施
```

依赖方向只能自上而下。`app` 是唯一同时认识 `shell` 和 `features` 的地方。

## 目录

```
DevToys/
├── cmd/devtoys/main.go        入口：RequestSingleInstanceLock + app.Run
├── internal/
│   ├── app/app.go             装配：读配置、建日志、注册模块与托盘，进入 mygo 事件循环
│   ├── shell/
│   │   ├── shell.go           组装：窗口、页面结构、模块切换、生命周期、host
│   │   ├── sidebar.go         侧栏：导航条目、折叠状态、底部插槽、拖动区域
│   │   ├── titlebar.go        自绘标题栏：图标 + 标题 + 副标题，右侧插槽
│   │   ├── settings.go        设置窗口：独立弹出窗口，通用（外观 + 语言）与快捷键两个选项卡
│   │   ├── hotkey.go          快捷键：accelerator 转换、全局注册、应用内响应与显示/隐藏主窗口
│   │   └── tray.go            托盘图标：右键菜单（显示/隐藏、检查更新、退出）
│   ├── contract/contract.go   Module / Live / Host
│   ├── features/
│   │   └── welcome/           欢迎页（示例模块，带侧栏图标）
│   │       ├── module.go      模块声明：ID、分组、标题、图标、状态、Init
│   │       ├── view.go        页面内容（只渲染）
│   │       ├── actions.go     界面触发的动作（起后台任务等）
│   │       └── locales/       该模块自己的 en.json、zh-Hans.json 与 T(key) 入口
│   ├── task/task.go           后台任务 runner，结果经 post 回到主线程
│   ├── config/                config.json 的读写与规范化
│   ├── logging/               slog 日志器（文本输出，dev 或 debug 时同时写 stderr）
│   └── fsx/atomic.go          原子写文件（临时文件 + fsync + rename）
├── locales/                   外壳与公共文案：en.json、zh-Hans.json、catalog_gen.go（生成）
├── mygo.json                  mygo 应用清单：入口、i18n 目录、更新源
└── go.mod                     Go 1.27，mygo / mygoui / mygoicons
```

## 启动流程

1. `cmd/devtoys` 取单实例锁，调用 `app.Run()`。
2. `app` 从 `os.UserConfigDir()/DevToys` 读 `config.json`：文件缺失时用默认值；JSON 损坏时把原文件备份为 `config.json.bad` 并回退默认值，保证应用仍能启动。
3. 初始化 `slog` 日志器。
4. 组装模块列表与 `shell.Options`（模块、窗口尺寸、`*config.Config`、托盘）。
5. `shell.Run(opts)` 先按配置应用主题（保证首帧正确），再注册回调（`WhenReady`、第二实例、全部窗口关闭、退出前清理），然后 `mygo.App.Run()` 进入事件循环。
6. `WhenReady` 里应用语言、创建主窗口与托盘，给每个模块建一个 `task.Runner`，调用 `Init`，注册全局快捷键，最后激活当前页。
7. 退出时 `OnBeforeQuit` → `shell.close()`：关闭所有 runner（取消任务并等待 goroutine），注销全局快捷键，销毁托盘。

## 契约（internal/contract）

| 接口 | 用途 | 实现方 |
| --- | --- | --- |
| `Module` | 主窗口功能页：`ID`、`Group`、`Title`、`Icon`、`Init(Host)`、`View(*ui.Context)` | `features/welcome` |
| `Live` | 可选：页面激活/离开钩子 `OnActivate` / `OnDeactivate` | 需要时由模块实现 |
| `Host` | 外壳给功能的运行时设施：`Tasks() *task.Runner`、`Log() *slog.Logger` | `shell.host` |

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

2. 在 `app.Run` 的模块列表里加一行：`ping.New()`。

3. 在 `internal/features/ping/locales/` 下放 `en.json`、`zh-Hans.json`，并照着欢迎页的 `locales.go` 写 `//go:embed`、`i18n.NewCatalog`、`i18n.Register` 和 `T(key)`；目录已按模块隔离，key 不需要加模块前缀，直接用 `locales.T("title")` 读取。`go tool mygo generate` 只管理根 `locales/`，不处理各 feature 的目录。

侧栏条目、标题栏标题和页面切换都由外壳自动完成，启动时进入模块列表的第一个页面。长时间运行的工作请走 `task.Go`，不要直接在 `View` 里阻塞；同一个 key 的新任务会取消并顶替旧任务。

## 窗口与导航

- 页面结构由 `shell.go` 的 `view` 自行组装，不用 `layout.Scaffold`：
  - 顶层 `ui.Row`：左侧是整列 `mygoui.Sidebar`（分组 + 图标 + 标题，自己管理宽度、背景与折叠动画，贯通窗口高度）；
  - 右侧是 `ui.Column`：上面是标题栏（模块图标 + 标题 + 分组副标题，右侧插槽留给操作按钮），下面是 `Grow(1)` 的内容区；内容区用 `mygoui.ScrollArea` 承载当前模块的 `View`，超出高度时整体滚动，滚动位置由外壳跨帧保存；
  - 分组名来自模块的 `Group()`，同组条目按注册顺序排列，分组顺序取首次出现的位置。
- 没有原生标题栏（`TitleBarStyle: TitleBarHidden`）：页面铺满整个窗口，系统窗口按钮悬浮在页面之上。右侧标题栏按 `c.TitleBar()` 报告的结束侧 insets 留出控件空间，整条可拖动窗口；高度取「44 DIP、原生按钮带、按钮行高」三者最大值，文字缩放变大时按钮也不会被裁切。侧栏本身也标记为拖动区域，所以侧栏顶部的空白处同样可以移动窗口。
- 侧栏展开宽度 180、折叠后 48，两个值都是 `sidebar.go` 里的常量，不可拖动调整，也不写入配置。宽度由 `mygoui.Sidebar` 自己持有并做折叠动画，右侧内容区作为 flex 兄弟节点跟着一起动，不会留下空白。
- 侧栏底部插槽（`mygoui.SidebarOptions.Footer`）放常驻的设置入口；侧栏展开时是整行按钮，折叠后收成齿轮图标。
- 设置按钮打开**独立的设置窗口**（`shell/settings.go`）：它是主窗口的子窗口，随主窗口显示/隐藏，但不遮挡、不阻塞主窗口；顶部是「通用 / 快捷键」两个选项卡。通用页用 `mygoui.SettingsGroup` 放外观（横向单选组）与语言（下拉选择）；快捷键页分「全局快捷键」与「应用内快捷键」两组，用 `form.HotkeyInput` 分别录制显示/隐藏窗口与折叠/展开侧边栏的组合。改动即时应用并写回 `config.json`。
- 标题栏右侧的插槽（`titleBarOptions.Trailing`）提供给调用方放操作按钮，目前留空。
- 没有搜索框、没有工具搜索，导航就是侧栏列表。

## 配置、日志、主题、语言

**配置文件** `os.UserConfigDir()/DevToys/config.json`：

| 字段 | 说明 |
| --- | --- |
| `log_level` | `debug` / `info` / `warn` / `error`，默认 `info` |
| `theme` | `system` / `light` / `dark`，默认跟随系统 |
| `locale` | `system` / `en` / `zh-Hans`，默认跟随系统 |
| `hotkey.global.window_toggle` | 全局显示/隐藏窗口的 accelerator（如 `Ctrl+Shift+Space`），默认空，不注册 |
| `hotkey.app.sidebar_toggle` | 应用内折叠/展开侧边栏的 accelerator，默认空，不响应 |

写入走 `fsx.WriteFile`（临时文件 + fsync + rename），损坏或缺失时回退默认值；未知的 `theme`/`locale` 在读取时归一化为跟随系统。旧的扁平键 `hotkey_global` / `hotkey_sidebar` 已废弃，不再读取，升级后需在设置窗口重新录制。

**日志**：`os.UserConfigDir()/DevToys/devtoys.log`。级别取 `config.log_level`，统一用 `slog.TextHandler` 输出文本；开发模式（`go tool mygo dev`）或 `debug` 级别时同时写 stderr。日志目录不可写时降级为只写 stderr，不会阻止应用启动。本期不做轮转。

**主题**：`shell` 的 `applyTheme` 把 `config.ThemeMode` 映射到 `mygo.Theme.SetSource`（启动时在 `App.Run` 之前调用一次），mygoui 组件在每帧用 `theme.Apply(c)` 读取当前外观。主题只在设置窗口里修改：设置页直接改 `shell.Options.Config` 并保存，随后由外壳重绘。

**语言**：每个消息目录各自是一个 `i18n.Catalog` 并调用 `i18n.Register`，注册的语言会合并。基准语言（source）是 `zh-Hans`，`en` 作为翻译：当前语言缺词时回退到基准语言。根 `locales/` 放外壳与公共文案，由 `go tool mygo generate` 维护（改完 `zh-Hans.json` / `en.json` 执行一次，生成 `catalog_gen.go`，key 变成 `locales.SettingsTitle()` 这样的函数；`@` 描述取自基准语言 `zh-Hans.json`）；`internal/features/<name>/locales/` 归各模块自己，JSON 与 `T(key)` 入口手写，`mygo generate` 不会覆盖或校验。`T` 用 `Catalog.Format(i18n.Locale(), key, nil)` 取词，无参数消息由 i18n 缓存。运行时用 `mygo.App.SetLocale` 切换，`shell` 的 `applyLocale` 把配置里的 `system` 映射成 mygo 的空字符串（跟随系统）：外壳在首帧调用一次，之后设置窗口改动同样走这里，并由外壳统一重绘主窗口与设置窗口。

**全局快捷键**：`shell` 用 `mygo.GlobalShortcut.Register` 绑定 `config.hotkey.global.window_toggle`，按 `form.HotkeyInput` 录制的组合重新注册；被占用时记一条 `warn` 并保持未注册。触发后显示并聚焦主窗口，或把主窗口收起（有托盘时等同隐藏）。

**应用内快捷键**：`shell` 把 `config.hotkey.app.sidebar_toggle` 解析成 `form.Hotkey`，在主窗口根视图用 `c.OnWindowShortcut` 声明；只在窗口获得焦点时生效，按下后翻转外壳的侧栏折叠状态并重绘。

## 托盘

- 托盘图标复用 `resources.AppIconPNG()` 的 `icon.png`，与窗口图标一致。
- `shell` 用 `mygo.NewTray` 创建托盘图标，只有右键菜单：显示/隐藏主窗口（右侧显示已设置的全局快捷键）、检查更新、退出；单击与双击不做任何事。本期没有 Flyout 面板，需要启动器时再加。
- 有托盘时关闭主窗口只是隐藏（macOS 上同时把激活策略切成 Accessory），只有 `Quit` 才真正退出并销毁托盘。

## 构建与验证

```bash
# 开发运行（热重载）
go tool mygo dev

# 产出安装包/可执行文件
go tool mygo build

# 修改 locales/*.json 后在项目根目录重新生成 catalog_gen.go
go tool mygo generate

# 格式、静态检查、测试
gofmt -l cmd internal locales resources
go vet ./...
go test ./...
go tool mygo i18n check .
```

`internal/shell` 目前没有测试文件；需要回归时可以用 `ui.NewTester` 直接渲染外壳，不依赖真实窗口。

## 约定

- 业务错误一律显式返回并带上下文（`fmt.Errorf("...: %w", err)`），不在业务逻辑里 panic。
- UI 状态只在主线程读写；后台 goroutine 只通过 `task.Runner` 交付结果，`Close` 负责取消并等待。
- 功能包只依赖 `contract` 与基础设施包，反过来 `shell` 不认识任何具体功能。
- 新增文案先加到 `locales/zh-Hans.json`（基准语言，带 `@` 描述），再补 `locales/en.json`，最后重新生成。
- 本期未做：命令行模式（`cmd` 里预留位置）、工具搜索、日志轮转、配置迁移。
