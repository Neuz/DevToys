# 截图（QQ 截图式）

> 状态：方案已定稿，未实现。
>
> 范围：Windows；参考 QQ 截图的交互（冻结屏幕 + 框选 + 窗口识别 + 标注 + 保存/取消/完成）。
>
> 决策记录：默认快捷键 `Ctrl+Alt+A`（用户确认）；首版标注工具为矩形 + 箭头 + 马赛克（用户确认），椭圆 / 画笔 / 文本留第二版。

## 1. 依赖选型

| 依赖 | 用途 | 理由 |
| --- | --- | --- |
| `github.com/kbinani/screenshot` | 全屏 / 任意矩形抓屏 | 纯 syscall（GDI `BitBlt`），与项目 `CGO_ENABLED=0` 约束兼容；支持多显示器虚拟桌面坐标；广泛使用、维护稳定 |
| `github.com/fogleman/gg` | 矩形、箭头的位图绘制 | Go 最常用的 2D 绘制库，纯 Go；第二版文本标注经 `golang.org/x/image` 加载系统字体（微软雅黑）支持中文 |

- 马赛克不引库：对冻结原图按块（8 / 16px）取均值填充，直接操作 `image.RGBA` 像素。
- 剪贴板写图：优先 mygo `Clipboard.Write`（PNG 格式）；若 Windows 后端不映射 `CF_DIB`（Word / QQ 粘贴需要 DIB），则仿照取词模块用 `user32.SetClipboardData` 自写 CF_DIB。实现期验证，二选一。

## 2. 交互流程

```
Ctrl+Alt+A（可改键）
   │ 1. 先抓屏（窗口尚未显示，避免抓到自身）
   │ 2. EnumWindows 收集候选窗口矩形（z-order，过滤不可见 / 工具窗口 / 桌面）
   ▼
覆盖层（每显示器一个全屏窗口：Frameless + Transparent + AlwaysOnTop + SkipTaskbar）
   背景显示冻结截图 + 半透明暗化遮罩
   ├─ 移动鼠标：命中测试 → 高亮窗口矩形（白框）
   ├─ 按下拖拽 >4px：切换为自由矩形选择
   ├─ 点击窗口：选中该窗口矩形
   ├─ Esc / 右键：取消
   ▼ 选区确认
标注态（仅选区保持明亮，四周继续变暗，选区下方浮出工具栏）
   工具：矩形 ▭ | 箭头 ↗ | 马赛克 ▦ | 撤销 ↩ | 重做 ↪
   颜色：红 / 黄 / 蓝 / 绿 / 黑（色块，首版固定 5 色，线宽固定）
   底部动作：保存 | 取消 | 完成（写入剪贴板）
   ▼ 完成
裁剪冻结原图 + 按图层顺序绘制标注 → PNG → 剪贴板（DIB）→ 关闭覆盖层
```

- 马赛克即画即涂：按下拖过的地方实时块化预览。
- 窗口识别：`EnumWindows` 返回值即 z-order，自上而下取第一个命中的可见顶层窗口；复用取词模块的 Win32 调用风格（见 `internal/features/translate/selection_windows.go`）。

## 3. 技术要点

- **每显示器一个覆盖窗口**，规避跨屏 DPI 突变；坐标统一用 DIP 记录，最终渲染按 `mygo.Display.ScaleFactor` 换算回物理像素裁剪原图。显示器信息来自 `mygo.Screen.Displays()`（`Bounds` / `WorkArea` / `ScaleFactor`）。
- **先抓屏后显示**是硬性顺序（否则截到覆盖层自己）；覆盖窗口在功能启用时预创建隐藏，把快捷键到冻结的延迟压到抓屏一步。
- **标注 = 图层模型**：`{type, rect/points, color, thickness}` 有序切片，撤销即 pop、重做即重新入栈；最终渲染时按序合成。
- 保存用系统「另存为」对话框（comdlg32 `GetSaveFileNameW`，Win32 调用方式与取词模块一致）。
- UAC 提权窗口会截成黑屏（GDI 限制），错误不阻断，作为已知限制记录。

## 4. 模块结构与配置

```
internal/features/screenshot/
  module.go        契约实现（Toggleable + GlobalHotkeyer + Closer），默认 Ctrl+Alt+A
  overlay.go       覆盖层窗口、状态机（选择态 / 标注态）、工具栏
  region.go        窗口探测与命中测试（region_windows.go / region_other.go）
  annotate.go      图层模型、撤销 / 重做
  render.go        裁剪 + gg 绘制 + 马赛克 + PNG 编码
  clipboard.go     剪贴板写入（PNG / CF_DIB）
  view.go          设置页（快捷键）
```

配置在 `features.screenshot` 节点（与翻译模块同套路，不新增配置文件）：

```json
{
  "features": {
    "screenshot": {
      "enabled": true,
      "hotkeys": { "trigger": "Ctrl+Alt+A" }
    }
  }
}
```

「保存」动作每次弹出系统「另存为」对话框，不记忆上次目录；设置页只有快捷键与功能开关。

## 5. 首版明确不做

- 椭圆、画笔、文本标注（第二版）。
- 放大镜、实时屏幕（非冻结预览）。
- 滚动截屏、长截图、GIF / 录屏、OCR。
- macOS / Linux。

## 6. 里程碑

| 里程碑 | 内容 |
| --- | --- |
| T1 | 抓屏 + 每显示器覆盖层 + 矩形拖拽选区 |
| T2 | 窗口识别、鼠标高亮与点击选中 |
| T3 | 矩形 / 箭头 / 马赛克标注 + 撤销 / 重做 + 工具栏 |
| T4 | 完成（剪贴板）/ 保存（另存为）/ 取消 |
| T5 | 设置页、快捷键、中英文文案、单测与验收 |
