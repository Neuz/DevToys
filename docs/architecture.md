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
│   │   ├── settings.go              设置窗口（通用 / 快捷键 / 网络）
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
| `Host` | 外壳给功能的运行时设施：`Tasks`、`Log`、`Window`、`Feature`、`SaveConfig`、`SetEnabled`、`HotkeysChanged` | `shell.host` |

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

功能页需要发起 HTTP 请求时统一使用 `network.Client()`，不要自带 Transport；约束见[网络代理 § HTTP 客户端约束](#http-客户端约束)。

## 窗口与导航

- 页面结构由 [shell.go](../internal/shell/shell.go) 的 `view` 自行组装：
  - 顶层 `ui.Row`：左侧整列 `mygoui.Sidebar`（分组 + 图标 + 标题，自己管理宽度、背景与折叠动画，贯通窗口高度）；
  - 右侧 `ui.Column`：上层是绝对定位的自绘标题栏，下层是 `Grow(1)` 的 `mygoui.ScrollArea`；内容区顶部预留标题栏高度，超出高度时整体滚动，滚动位置由外壳跨帧保存。
- 分组名来自模块的 `Group()`，同组条目按注册顺序排列，分组顺序取首次出现的位置（`lo.Uniq` / `lo.GroupBy`）。
- 无原生标题栏：系统窗口按钮悬浮在页面上，标题栏按 `c.TitleBar()` 的结束侧 insets 留出空间，整条可拖动窗口；高度取「44 DIP、原生按钮带、按钮行高」三者最大值。
- 侧栏展开 180、折叠 48，均为 [sidebar.go](../internal/shell/sidebar.go) 中的常量，不写入配置。侧栏本身是拖动区域。
- 设置按钮打开独立的模态子窗口（[settings.go](../internal/shell/settings.go)）：顶部「通用 / 快捷键 / 网络」三个选项卡；通用页包含外观、语言、开机启动与版本/检查更新；快捷键页包含外壳的显示/隐藏窗口快捷键；网络页见下节。

## 网络代理

> 状态：已实现。默认使用系统代理。

### 模式与边界

| 模式 | 配置值 | 行为 |
| --- | --- | --- |
| 不使用代理 | `direct` | 所有请求直连，忽略系统代理与环境变量 |
| 系统 | `system` | 读取当前系统代理（含绕过列表、PAC/WPAD），应用内请求全部走该代理 |
| 手动 | `manual` | 使用填写的代理地址与例外列表，支持 HTTP、HTTPS、SOCKS5 |

「全局」指 **DevToys 进程内通过 Go `net/http` 发出的请求**：包括 mygo updater 的检查与下载、后续功能页请求，以及直接使用 `http.DefaultClient` 的代码。从启动器拉起的外部程序不受影响；当前没有 WebView；自带独立 Transport 的第三方库无法覆盖。

### 配置

```go
type ProxyMode string

const (
    ProxyDirect ProxyMode = "direct"
    ProxySystem ProxyMode = "system"
    ProxyManual ProxyMode = "manual"
)

type NetworkConfig struct {
    Proxy ProxyConfig `json:"proxy"`
}

type ProxyConfig struct {
    Mode       ProxyMode `json:"mode"`
    URL        string    `json:"url,omitempty"`        // 仅 manual
    Exceptions []string  `json:"exceptions,omitempty"` // 仅 manual，已规范化
}
```

```json
{
  "network": {
    "proxy": {
      "mode": "manual",
      "url": "socks5://127.0.0.1:1080",
      "exceptions": ["localhost", "*.corp.example.com", "10.0.0.0/8"]
    }
  }
}
```

规则：

- 默认 `mode = system`，与现状一致；缺省或未知值在 `Normalize()` 中归一化为 `system`。
- 手动 URL 允许省略 scheme（按 `http://` 处理），必须包含主机与端口；scheme 仅接受 `http`、`https`、`socks5`（`socks5h` 归一化为 `socks5`）；不允许 path（结尾 `/` 除外）、query、fragment；允许 `user:pass@` 凭据。
- 例外列表在设置页以逗号分隔输入，保存时去掉空白、转小写、去重后写入数组。
- 启动时 `manual` 的 URL 为空或非法：记录 warn，本次运行回退为 `system`，配置保持原样；设置页仍显示原配置，修正后可重新保存。

### 系统代理

`system` 分支的**决策**不自行读取注册表，直接使用 mygo 的 `netproxy`，与官方推荐写法一致：

```go
client := &http.Client{Transport: netproxy.Transport(), Timeout: time.Minute}
```

`netproxy.Proxy` 的决策顺序是：回环直连 → 环境变量（`HTTP_PROXY` / `HTTPS_PROXY` / `ALL_PROXY` / `NO_PROXY`）→ 系统设置（Windows 为 WinHTTP，含 PAC/WPAD 与绕过列表）。系统答案缓存 5 秒，运行中修改系统代理可在数秒内生效。**环境变量优先于系统设置**，这是 netproxy 的既定行为，本方案沿用。

设置页为了展示系统代理地址与例外，会在 Windows 上以只读方式读取当前用户的「Internet 选项」（`AutoConfigURL` / `AutoDetect` / `ProxyServer` / `ProxyOverride`）；这只是展示快照，不参与代理决策，也不修改任何系统设置。

### 全局安装

为了让 `direct` / `manual` 也能影响 updater，启动早期把一个稳定的包装函数安装到共享 transport；其 `system` 分支原样调用 `netproxy.Proxy`：

```go
func Init(cfg config.ProxyConfig) error {
    if err := Apply(cfg); err != nil { // 校验并写入原子状态
        return err
    }
    netproxy.Transport().Proxy = Proxy
    if t, ok := http.DefaultTransport.(*http.Transport); ok {
        t.Proxy = Proxy
    }
    return nil
}
```

安装时机不可交换：mygo updater 在**首次请求时克隆** `netproxy.Transport()`，所以安装必须发生在任何请求之前——`app.Run` 中日志初始化之后、读取资源与 `shell.Run` 之前。安装后包装函数本身不再替换，切换模式只更新内部原子状态，updater 已持有的 client 也会跟随。

这是对 mygo 内部共享对象的依赖。若上游提供 `netproxy.SetProxy` 或 updater 的 client 注入，可改为显式注入并移除对共享 transport 的替换。

### HTTP 客户端约束

代理通过进程级 transport 生效，所有内部 HTTP 请求遵守以下约束：

1. **统一入口**：新代码只使用 `network.Client()`，不要构造 `&http.Transport{}` 或自带 Transport 的 `&http.Client{}`。
2. **不要改写共享 transport**：`netproxy.Transport()` 的 `Proxy` 字段归 `internal/network` 所有；业务代码不要设置 `Proxy`、`TLSClientConfig`、`DialContext` 等字段。
3. **不要绕过决策函数**：不要直接调用 `http.ProxyFromEnvironment`，也不要自行读取 `NO_PROXY` / 系统代理后决定直连；这些逻辑只在 `network.Proxy` 中实现。
4. **超时**：`network.Client()` 默认 `Timeout: time.Minute`；长任务用 `context` 收紧，短请求也建议带超时，避免占用连接。
5. **客户端可随时创建**：`Client()` 每次返回新的 `*http.Client`，共享同一个 transport，可并发使用；不要缓存后修改它。
6. **运行时切换**：设置页只调用 `network.Apply`，不重建 transport；已有 client 在下一次请求按新状态选代理，无需重启。
7. **回环直连**：`localhost`、`*.localhost`、`127.0.0.0/8`、`::1` 在任何模式下都直连，业务代码无需特殊处理。
8. **第三方库**：自带 client/transport 的库无法被覆盖；接入前确认可以传入 `network.Client()` 或共享 transport。
9. **测试辅助**：`CheckReachable` 只供设置页检测代理地址，不要用它探测业务地址。

```go
func Client() *http.Client {
    return &http.Client{Transport: netproxy.Transport(), Timeout: time.Minute}
}
```

### 手动模式与例外

- 解析顺序：回环地址直连 → 命中等例列表直连 → 其余走填写的代理。
- 例外匹配规则（不区分大小写）：
  - 裸域名 `example.com`：匹配该主机及其子域；
  - `.example.com` / `*.example.com`：匹配子域与该主机；
  - IP：精确匹配；
  - CIDR（如 `10.0.0.0/8`）：按网段匹配；
  - `*`：全部直连。
- 例外只作用于手动模式；系统模式使用操作系统自己的绕过列表，直连模式没有例外概念。
- 回环地址（`localhost`、`*.localhost`、`127.0.0.0/8`、`::1`）在任何模式下都直连。
- 手动模式不解析 PAC，需要 PAC 时使用系统模式。

### 设置页

设置窗口增加第三个选项卡「网络」，内容是一组代理设置（不显示分组标题与说明）：

| 行 | 控件 | 说明 |
| --- | --- | --- |
| 网络代理 | 横向单选组 | 不使用代理 / 系统 / 手动，图标 `lucide.Network()` |
| 代理地址 | 输入框 / 只读文字 | 手动模式为输入框，系统模式在右侧只读显示；图标 `lucide.Globe()` |
| 代理例外 | 输入框 / 只读文字 | 手动模式为逗号分隔输入框，系统模式在右侧只读显示；空时显示「无」 |
| 操作 | 【测试并保存】+ 状态 | 仅手动模式；测试通过才保存 |

行为：

- 系统模式只展示系统代理设置，不显示操作行；展示内容来自只读快照，不参与 `netproxy` 的决策。
- 代理地址与例外是当前窗口的草稿；点击【测试并保存】时先校验 URL 与例外，不合法则显示错误，不改变当前生效配置。
- 校验通过后检测代理地址是否可达：
  1. 解析出 `host:port`，用 `net.Dialer` 做一次 TCP 连接，超时 3 秒；
  2. 可达：应用并保存 `manual`，提示「地址可达（耗时），已保存并生效」；
  3. 不可达：切换为 `system` 并保存，提示「未配置代理，使用系统代理」；已填写的地址与例外保留，便于之后切回手动修改。
  
  检测不发送 HTTP 请求、不依赖固定外部 API，也不校验代理协议、认证或出口网络。
- 地址输入变化时清空上一次结果，避免把旧地址的结果显示在新地址上。
- 检测在后台 goroutine 中执行，【测试并保存】在检测期间禁用；每次操作都有序号，窗口关闭、取消或被新操作取代时丢弃结果。
- `direct` / `system` 切换后立即生效并保存；切到 `manual` 只显示输入框，不改变当前生效配置，成功执行【测试并保存】后才应用并持久化。
- 网络代理行的副标题显示当前真正生效的模式（手动地址脱敏）；回退到系统代理时直接显示「未配置代理，使用系统代理」。操作行的副标题显示检测中、保存成功或校验错误。成功文案明确是「地址可达」，避免被理解为代理完全可用。

新增根文案 key（`SettingsNetwork`、`SettingsProxyMode`、`SettingsProxyUrl`、`SettingsProxyUrlPlaceholder`、`SettingsProxyExceptions`、`SettingsProxySave`、`SettingsProxySavedReachable`、`SettingsProxyFallback`、`SettingsProxyInvalid`、`SettingsProxyTesting`、`SettingsProxySystem*`、`SettingsProxyCurrent*` 等），执行 `go tool mygo generate` 并补 `en` 翻译。

### 请求覆盖

| 请求来源 | 是否覆盖 | 原因 |
| --- | --- | --- |
| mygo updater（检查更新、下载安装包） | 是 | 首次请求克隆 `netproxy.Transport()`，安装包装函数后生效 |
| 使用 `network.Client()` 的功能页 | 是 | Transport 来自同一共享对象 |
| 直接使用 `http.DefaultClient` 的代码 | 是 | 同时替换了 `http.DefaultTransport.Proxy` |
| 自带独立 Transport/Client 的第三方库 | 否 | 无法在不修改库的情况下拦截 |
| 从启动器启动的外部应用 | 否 | 走各自进程的系统代理设置 |

### 日志与安全

- 切换成功：`Info("network: proxy updated", "mode", mode, "proxy", redacted)`；启动回退：`Warn("network: 代理配置无效，回退到系统代理", "error", err)`。
- 日志中的代理 URL 只保留 scheme、host、port，隐藏 userinfo 中的密码；错误信息不回显完整密码。
- 含凭据的 URL 以明文保存在 `config.json`，依赖当前用户目录的权限保护。

### 测试

- `internal/network`：三种模式的选择、回环直连、例外匹配（域名 / 子域 / IP / CIDR / `*`）、URL 校验（scheme、端口、path）、失败不改变旧状态、并发切换（`-race`）、脱敏函数。
- `internal/config`：默认 `system`、未知 mode 归一化、`exceptions` 序列化与默认值。
- 手工验证：默认系统模式行为不变；直连在设置系统代理或 `HTTP_PROXY` 后仍直连；手动 HTTP / SOCKS5 能完成更新检查；例外列表命中时直连；非法地址不写入配置；切换无需重启。
- 可达性检测可用本地监听端口与关闭端口做单元测试，不依赖外网；代理的真实可用性（协议、认证、出口）手工验证。

### 风险

| 风险 | 影响 | 缓解 |
| --- | --- | --- |
| 修改 mygo 共享 transport 的 `Proxy` 属于内部约定 | mygo 升级后可能失效 | 启动早期一次性安装；行为测试；推动上游提供 `netproxy.SetProxy` |
| `system` 模式沿用环境变量优先 | 进程设置 `HTTP_PROXY` 时不会严格只读系统设置 | 文档说明；需要严格语义时再做平台实现 |
| 手动模式不解析 PAC | 企业 PAC 场景不适用 | 使用系统模式 |
| 凭据明文保存 | 配置泄露即代理泄露 | 日志脱敏；后续可接 Windows Credential Manager |
| TCP 可达不等于代理可用 | 代理进程在监听，但协议、认证或上游出口有问题时仍显示可达 | 文案明确为「地址可达」；真实可用性由更新检查等实际请求验证 |

### 实现位置

| 位置 | 内容 |
| --- | --- |
| [config.go](../internal/config/config.go) | `ProxyMode`、`NetworkConfig`、`ProxyConfig`、默认 `system`、`Normalize` |
| [network.go](../internal/network/network.go) | `Init` / `Apply` / `Proxy` / `Client` / `Current` / `CheckReachable` / `Redact` |
| [validate.go](../internal/network/validate.go) | 代理 URL、例外列表的解析与匹配 |
| [systemproxy_windows.go](../internal/network/systemproxy_windows.go) | 设置页只读系统代理快照（Windows） |
| [app.go](../internal/app/app.go) | 启动早期安装代理；失败回退 `system` 并 warn |
| [settings.go](../internal/shell/settings.go) | 「网络」选项卡与【测试并保存】 |
| [locales/](../locales) | 网络相关文案（`go tool mygo generate` 生成） |

### 验收

- 默认 `system`，现有更新检查行为不变。
- 三种模式运行时可切换，无需重启；`direct` 确实绕过系统代理与环境变量。
- 手动模式支持 `http://`、`https://`、`socks5://` 与可选凭据，例外列表按上述规则生效。
- 【测试并保存】只做 URL 校验与 TCP 可达性检测，不访问固定 API；通过后才应用并持久化 `manual`。
- 检测失败时回退并保存为 `system`，界面提示「未配置代理，使用系统代理」；校验失败只显示错误，不改动当前配置。
- 更新检查与新功能页请求遵循当前模式；新增请求都经 `network.Client()`（或共享 transport），未改写 `netproxy.Transport()` 的代理字段。
- 日志不出现代理密码；格式、vet、测试与 i18n 检查全部通过。

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
