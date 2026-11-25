# 架构设计

面向开发者的结构说明：分层依赖、契约、线程模型与扩展方式。用户视角的能力描述见 [外壳](shell.md) 与各功能文档。

## 分层与依赖

```
cmd/devtoys   入口：单实例锁，调用 app.Run
   │
 app          装配层：配置、日志、网络代理、模块列表、外壳、托盘
   │
 shell        窗口外壳：页面结构 / Sidebar / 标题栏 / 设置窗口 / 托盘 / 快捷键 / 生命周期
   │
 contract     外壳 ↔ 功能的唯一契约（接口）
   │
 features/*   功能页：welcome · launcher · translate · screenshot · qrcode
   │
 task · config · logging · hotkey · fsx · network   基础设施
```

依赖只能自上而下。`app` 是唯一同时认识 `shell` 与具体 `features` 的地方；`features` 不 import `shell`，`shell` 不 import 任何具体功能。

## 目录结构

```
DevToys/
├── cmd/devtoys/main.go              入口：RequestSingleInstanceLock + app.Run
├── internal/
│   ├── app/app.go                   装配：配置、日志、代理安装、模块列表、外壳、托盘
│   ├── shell/                       外壳：shell / sidebar / titlebar / settings / hotkey / tray
│   ├── contract/contract.go         Module / Live / Toggleable / GlobalHotkeyer / Closer / Host
│   ├── features/
│   │   ├── welcome/                 示例页：module / view / actions + locales
│   │   ├── launcher/                启动器：module / index / search / store / panel / launch
│   │   ├── translate/               划词翻译：module / panel / selection* / service / provider / google / bilibili
│   │   ├── screenshot/              截图：module / flow / capture* / overlay / input / adjust /
│   │   │                            shape / toolbar / paint / render / geometry / font_*
│   │   └── qrcode/                  二维码：module / view / export
│   ├── task/task.go                 后台任务 runner，结果经 post 回到主线程
│   ├── config/                      Config 模型、默认值、规范化、功能节点、原子读写
│   ├── hotkey/hotkey.go             快捷键文本与 accelerator 互转
│   ├── logging/logging.go           slog 日志器（文本输出）
│   ├── network/                     全局代理：Init / Apply / Proxy / Client / 校验与匹配
│   └── fsx/atomic.go                原子写文件（临时文件 + fsync + rename）
├── locales/                         外壳与公共文案（catalog_gen.go 由 mygo 生成）
├── resources/                       应用图标与嵌入资源
├── docs/                            外壳、架构与各功能文档
├── mygo.json                        mygo 应用清单：入口、i18n 目录、更新源
└── go.mod                           Go 1.27，mygo / mygoui / mygoicons
```

## 启动与退出

1. `cmd/devtoys` 取单实例锁，调用 `app.Run()`；启动失败时 `log.Fatal`。
2. `app.Run` 从用户配置目录读 `config.json`：缺失用默认值；JSON 损坏时备份为 `config.json.bad` 并回退默认值。
3. 初始化 `slog` 日志器；随后安装全局网络代理（必须早于任何 HTTP 请求，失败回退系统代理并 warn）。
4. 组装模块列表与 `shell.Options`（模块、窗口尺寸、配置、托盘），`shell.Run` 进入事件循环。
5. `WhenReady`：应用语言 → 创建主窗口 → 为每个模块建独立 `task.Runner` 并 `Init(host)` → 注册主题/语言回调 → 创建托盘 → 注册全局快捷键 → 激活默认页。
6. 退出时依次调用实现了 `Closer` 的模块、关闭所有 runner（取消并等待）、注销快捷键、销毁托盘。

## 契约

实现位置：[internal/contract/contract.go](../internal/contract/contract.go)

| 接口 / 类型 | 用途 | 当前实现方 |
| --- | --- | --- |
| `Module` | 主窗口功能页：`ID`、`Group`、`Title`、`Icon`、`Init(Host)`、`View(*ui.Context)` | 全部功能 |
| `Live` | 可选：页面激活/离开钩子 | 需要时实现 |
| `Toggleable` | 可选：`Enabled()`，禁用后外壳跳过它的全局快捷键 | launcher、translate、screenshot |
| `GlobalHotkeyer` | 可选：`Hotkeys()` 声明 ID、默认 accelerator、描述与回调 | launcher、translate、screenshot |
| `Closer` | 可选：`Close()`，退出时释放窗口等资源 | launcher、translate、screenshot |
| `Host` | 外壳给功能的运行时设施：`Tasks`、`Log`、`Window`、`Feature`、`SaveConfig`、`SetEnabled`、`HotkeysChanged` | `shell.host` |

`Host.Feature()` 返回该模块自己的配置节点（`enabled`、`hotkeys`、`options`），功能只应改写自己的节点并通过 `SaveConfig()` 落盘。

## 线程模型与后台任务

- 所有 UI 状态只在主线程读写。
- `task.Go(runner, key, fn)` 在 goroutine 中执行，结果经 post 回到主线程帧里；同一 key 的新任务取消旧任务；`fn` 的 panic 被转成错误；`Runner.Close` 取消全部任务并等待退出。
- **约束：后台任务一律优先使用 `task.Go`，不要在功能模块里手写 `go func()`。** 扇出多任务（如翻译多服务并行）可以用 goroutine，但必须由带超时的 `context` + generation 管理生命周期。允许的例外只有两类，须在注释说明：平台 API 要求独立线程（如 COM 调用）、标准库定时器回调。
- 全局快捷键回调可能不在主线程，操作窗口前用 `mygo.RunOnMain` 切回。

参考实现：launcher（索引重建）、translate（取词/重试走 `task.Go`，扇出走 context）、screenshot（截图与保存走 `task.Go`，取消窗口用 `task.Go` 绕回主线程帧外销毁）。

## 截图的坐标与渲染约定

- 截图源（物理像素）与 UI（DIP）是两套坐标，换算集中在 `geometry.go`；多显示器以虚拟桌面整体为画布。
- 标注维护 `[]Shape` 矢量列表：按下创建、拖动更新预览、松开固化；预览帧只做矢量绘制，只有完成/保存时才 `image/draw` 光栅化合并输出。
- 覆盖层自身必须从截图中排除：保存前隐藏覆盖层再弹对话框；窗口点选用 DWM 可见边界并过滤不可见窗口。

## HTTP 客户端约束

代理通过进程级 transport 生效，功能页发起请求时：

1. 只使用 `network.Client()`，不要自带 Transport。
2. 不要改写共享 transport 的 `Proxy` 等字段，不要绕过 `network.Proxy` 自行读环境变量。
3. 回环地址在任何模式下直连；长任务用 `context` 收紧超时。

## 国际化

- 根 `locales/` 放外壳与公共文案，由 `go tool mygo generate` 生成函数入口；基准语言是 `zh-Hans`，`en` 为翻译，缺词回退基准。
- `internal/features/<name>/locales/` 归各模块自己，JSON 与 `T(key)` 手写，目录按模块隔离，key 不加前缀。

## 新增一个工具页

1. 在 `internal/features/` 下建包，实现 `Module`（必要时加 `Toggleable` / `GlobalHotkeyer` / `Closer`）：

```go
package ping

type Module struct {
    tasks *task.Runner
    log   *slog.Logger
}

func (m *Module) ID() string    { return "ping" }
func (m *Module) Group() string { return "" } // 空分组，侧栏不显示分组标题
func (m *Module) Title() string { return locales.T("title") }
func (m *Module) Icon() *ui.SVG { return lucide.Activity() }

func (m *Module) Init(h contract.Host) { m.tasks, m.log = h.Tasks(), h.Log() }

func (m *Module) View(c *ui.Context) {
    theme.Apply(c)
    // 后台任务用 task.Go(m.tasks, "ping", fn)，结果在帧里通过 Result 读取。
}
```

2. 在 [app.go](../internal/app/app.go) 的 `setupModules()` 里加一行 `ping.New()`。
3. 在 `internal/features/ping/locales/` 下放 `en.json`、`zh-Hans.json`，照现有模块写 embed 与 `T(key)`。

侧栏条目、标题与页面切换由外壳自动完成。

## 构建与校验

```bash
go tool mygo dev      # 开发运行（热重载）
go tool mygo build    # 构建可执行文件与安装包
go tool mygo generate # 修改根 locales/*.json 后重新生成 catalog_gen.go
```

校验（与 CI 一致）：

```bash
gofmt -l cmd internal locales resources
go vet ./...
go test ./...
go tool mygo i18n check .
```

发布流水线见 [.github/workflows/release.yml](../.github/workflows/release.yml)：推送 `v*` 标签后先跑格式、vet、测试与 i18n 检查，再创建草稿 Release 并构建上传 Windows amd64 / arm64 安装包。
