# DevToys

用 Go 写的桌面开发工具箱骨架：原生窗口（[mygo](https://github.com/egoist/mygo)）+ 组件库（[mygoui](https://github.com/Cyenoch/mygoui)）。左侧 Sidebar 导航、自绘标题栏、常驻托盘，主题与界面语言都可在运行时切换。

## 文档

- [需求与范围](docs/requirements.md)：功能清单、优先级、明确的非目标
- [架构设计](docs/architecture.md)：模块划分、数据流、关键接口
- [开发进度](docs/progress.md)：已完成、进行中、待办、已知问题
- [启动器](docs/features/launcher.md)：全局快速启动功能说明
- [划词翻译](docs/features/translate.md)：取词、浮窗与翻译渠道的方案设计（未实现）
- [网络代理](docs/architecture.md#网络代理)：外壳「网络」分类与全局代理

## 当前功能

- **外壳**：自绘标题栏、可折叠侧栏、独立设置窗口、托盘、全局快捷键、检查更新。
- **外观与语言**：`system` / `light` / `dark`，`system` / `en` / `zh-Hans`，切换即时生效并写入配置。
- **启动器**：默认 `Alt+F` 唤起搜索面板，索引开始菜单应用与 Windows 系统设置，支持拼音与使用频率排序。
- **可扩展**：每个功能拥有独立的配置节点、文案目录与后台任务 runner。

## 快速开始

环境：Go 1.27+；当前开发与发布目标为 Windows。

```bash
go tool mygo dev     # 开发运行（热重载）
go tool mygo build   # 构建可执行文件与安装包
```

校验（与 CI 一致）：

```bash
gofmt -l cmd internal locales resources
go vet ./...
go test ./...
go tool mygo i18n check .
```

## 目录速览

```
cmd/devtoys/        入口：单实例锁 + app.Run
internal/app/       装配：配置、日志、模块、外壳、托盘
internal/shell/     外壳：窗口、侧栏、标题栏、设置窗口、托盘、快捷键注册
internal/contract/  外壳 ↔ 功能的唯一契约
internal/features/  功能页：welcome（示例）、launcher、qrcode
internal/task/      后台任务，结果回到主线程
internal/config/    config.json 读写与规范化
internal/logging/   slog 日志器
internal/hotkey/    快捷键文本与 accelerator 互转
internal/fsx/       原子写文件
locales/            外壳与公共文案（catalog_gen.go 由 mygo 生成）
resources/          应用图标与嵌入资源
docs/               需求、架构、进度与功能文档
```

## 约定

- 依赖单向：`cmd → app → shell → contract ← features`；`features` 不 import `shell`，`shell` 不认识具体功能，装配只在 `app`。
- UI 状态只在主线程读写；后台 goroutine 通过 `task.Runner` 把结果交回帧里，退出时统一取消并等待。
- 业务错误显式返回并带上下文（`fmt.Errorf("...: %w", err)`），不在业务逻辑里 panic。
- 新增文案先写 `locales/zh-Hans.json`（基准语言，带 `@` 描述），再补 `en.json`，最后执行 `go tool mygo generate`。

## 许可证

[MIT](LICENSE)
