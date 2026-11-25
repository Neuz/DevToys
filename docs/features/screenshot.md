# 截图（QQ 截图式）

> 状态：已实现（含完整标注工具集与选区/图形调整）。交互流程：唤起选区 → 标注（形状 / 箭头 / 画笔 / 马赛克 / 文本）→ 调整（选区缩放 / 平移、标记选中移动）→ 输出（Enter 完成 / Ctrl+S 保存 / Esc 取消）。
>
> 范围：Windows；参考 QQ 截图的交互（冻结屏幕 + 框选 + 窗口识别 + 标注 + 保存/取消/完成）。
>
> 决策记录：默认快捷键 `Ctrl+Alt+A`（用户确认）；工具栏统一用 mygoui 组件（`navigation.Toolbar` + `ToggleButton` + `form.Slider` + `color.ColorSwatchPicker`）；标注架构为 `[]Shape` 矢量列表，仅完成 / 保存时光栅化。

## 1. 依赖选型

| 依赖 | 用途 | 理由 |
| --- | --- | --- |
| `github.com/kbinani/screenshot` | 全屏 / 任意矩形抓屏 | 纯 syscall（GDI `BitBlt`），与项目 `CGO_ENABLED=0` 约束兼容；支持多显示器虚拟桌面坐标；广泛使用、维护稳定 |
| `golang.org/x/image` | 文本光栅化（`opentype` 解析系统字体 + `font.Drawer`） | 官方扩展库；文本标注需要任意字号的中文字体（微软雅黑），`basicfont` 只有 13px ASCII |

- 标注渲染**架构上分两层**：交互态用 `[]Shape` 矢量列表（`Painter` 直接画，不落位图）；只有「完成 / 保存」时才用标准库 `image/draw` 把 Shape 光栅化合并到底图，不引 2D 绘制库。
- 马赛克不引库：对冻结原图按块（16px×scale）取均值填充，直接操作 `image.RGBA` 像素。
- 剪贴板写图：直接用 `mygo.Clipboard.WriteImage(pngData)`（mygo 内部完成 PNG → 系统位图与主线程分发），不自写 CF_DIB。
- 保存对话框：直接用 `mygo.Dialog.Save(mygo.SaveDialogOptions{...})`（阻塞式、可在任意 goroutine 调用），不自写 comdlg32。

## 2. 交互流程（用户定稿）

```
1. 唤起与选择区域
Ctrl+Alt+A（可改键）
   │ 1. 先抓屏（窗口尚未显示，避免抓到自身）
   │ 2. EnumWindows 收集候选窗口矩形（z-order，过滤不可见 / 工具窗口 / 桌面）
   ▼
覆盖层（每显示器一个全屏窗口：Frameless + AlwaysOnTop + SkipTaskbar）
   背景显示冻结截图 + 半透明暗化蒙版
   ├─ 移动鼠标：命中测试 → 高亮窗口矩形（白框，左上角显示尺寸 1920*1080）
   ├─ 按下拖拽 >4px：自由矩形选择
   ├─ 点击窗口：选中该窗口矩形
   ▼ 选区确定
2. 标注（选区明亮，下方浮出真实组件工具栏）
   主工具栏（图标按钮，单选）：矩形 | 圆形 | 划线 | 箭头 | 画笔 | 马赛克 | 文本
                              + 撤销 ↩ | 重做 ↪ | 保存 | 完成 | 取消
   子工具栏（按当前工具切换，单行、与主栏同高）：
     形状 / 箭头 / 画笔 / 马赛克   线条粗细（Slider 1–12 DIP）+ 颜色（ColorSwatchPicker 6 色）
     马赛克另附模式切换按钮        涂抹（Brush 图标）/ 框选（SquareDashedX 图标，默认）
     文本                          字号（Slider 1–48）+ 字色
   鼠标操作：左键按下取起点，按住拖动，松开确认终点；画笔 / 涂抹沿路径采集点
   文本：点击锚定位置 → 就地编辑（EditableText 风格，Enter 提交 / 空文本丢弃）
2.5 调整（工具栏取消所有工具后，即 toolNone 状态）
   选区缩放：8 向手柄（四角 + 四边中点）拖拽，钳制最小边长与屏幕边界
   选区平移：选区内按下拖动（未命中 Shape 时）
   标记移动：点击 / 拖动已固化 Shape（最上层优先命中），包围盒钳制在选区内
3. 输出
   完成（Enter 或点击按钮）：光栅化全部 Shape 合成 → PNG → 系统剪贴板，可直接 Ctrl+V
   保存（Ctrl+S 或点击按钮）：隐藏覆盖层 → 系统另存为对话框 → 写 PNG；对话框关闭后退出截图
   取消（Esc 或右键）：文本输入中退出输入 → 绘制中丢弃临时 Shape → 否则退出截图
```

- 窗口识别：`EnumWindows` 返回值即 z-order，自上而下取第一个命中的可见顶层窗口；复用取词模块的 Win32 调用风格（见 `internal/features/translate/selection_windows.go`）。
- 窗口级快捷键：`Enter` 完成、`Ctrl+S` 保存、`Esc` 取消（`OnWindowShortcut`，与焦点无关）。

## 3. 技术要点

- **坐标约定**：`overlay.bounds` 统一用虚拟桌面物理像素（与 kbinani 截图、`GetWindowRect`、指针换算同系）；mygo 窗口与 `Displays().Bounds` 是 per-display 的 DIP（Windows 后端），窗口尺寸/位置按 `scale` 换算。截图与显示器按下标可能错位，按物理 = round(DIP × scale) 尺寸匹配。明亮区域用 `Painter.ImageRegion`（源像素矩形 → DIP 目标矩形）绘制，`Image + FillBox` 是整图拉伸、不能用于裁剪。
- **先抓屏后显示**是硬性顺序（否则截到覆盖层自己）。
- **标注 = `[]Shape` 有序切片**（`shape.go`）：`Shape{ID, Type, Start, End, Color, Width, Text, FontSize, Scale, Points, Blocks, Block}`；交互严格三段式——MouseDown `begin` 创建临时 Shape 推入列表尾部、MouseMove `update`/`appendPoint`/`smearTo` 触发重绘（预览态白细边框）、MouseUp `commit` 固化（位移过小 / 单点画笔 / 空块 / 空文本丢弃）。撤销 / 重做操作 Shape 栈。渲染顺序（Z-Order）：底图 → 暗色遮罩 → 选区明亮 → 遍历 `[]Shape` → 选中高亮框 / 调整手柄 → 工具栏（真实组件）与提示。
- **选区与图形调整**（`adjust.go`，toolNone 状态）：8 向手柄命中半径 6 DIP，对边固定、最小边长 `minRegionPixels=20` 物理像素、钳制屏幕边界；Shape 移动与选区平移都从按下时快照算位移（无累积误差），`translate` 平移 Start/End/Points，马赛克块按块尺寸量化（`roundDiv`）。命中框 `shapeBounds` 按类型计算，细线类外扩 max(线宽/2, 6·scale) 容差。
- **马赛克两种模式**（`boxBlocks` / `smearBlocks`）：框选 = 拖拽中画矩形样式预览（粗细固定 2），松开把矩形内所有块生成；涂抹 = 沿路径按步长补点、笔刷半径 `max(thick/4, 0.5)` 块（滑杆 1–12 DIP → 0.5–3 块，圆判支持小数），块列表去重后随 Shape 保存。
- 光栅化（`render.go`，仅完成 / 保存时执行）：矩形边框四条 `draw.Draw` 实心带；椭圆沿参数方程逐点填充圆盘；线 / 画笔 / 箭头杆 = 折线逐点实心圆盘；箭头 = 圆盘杆 + 重心坐标实心三角头；马赛克块均值填充；文本 = `font.Drawer` + 系统字体（微软雅黑 → 宋体 → 黑体 → Arial 回退，`opentype.ParseCollection` 兼容 ttc/ttf，无内嵌字体）。
- 光栅化前必须把底图拷到原点为 (0,0) 的新画布（`SubImage` 保留原 `Bounds().Min`，直接编码会带偏移）。
- **工具栏 = 真实 mygoui 组件**（`toolbar.go`）：绝对定位 frame（全屏 `PassThrough`）内嵌 `navigation.Toolbar`（子栏底色 `t.Input.Over(t.Card)`，避免白色色块在 Card 白底上隐形），工具用 `ToggleButton`——**每个按钮必须独立选中变量**（共享变量会被闭包读到末次赋值，导致切换失效）；属性区用 `form.Slider`（`.Width(sliderWidth).Shrink(0)` 抵消 FillWidth 防挤压）+ `color.ColorSwatchPicker`；子栏单行统一行高（`subBarHeight=28` + `ButtonSmall`）；工具栏与文本输入框的实际矩形由包装元素绘制回调捕获（`toolbarRectBox`/`textBoxRect`），画布把落在其上的按压让给组件。文本就地编辑用 `ui.TextInputBase`（EditableText 风格：Background + Accent 描边；**不接受 Key**，构建期即初始化输入状态会 panic；捕获矩形用 `DrawOver` 不用 `Draw`）。覆盖层预览的文本直接 `Painter.Text`，光栅化才用真实字体。
- **窗口识别**（`capture_windows.go`，纯 syscall 无 CGO）：`EnumWindows` 即 z-order；过滤不可见、最小化（`IsIconic`）、被 DWM 遮蔽（`DWMWA_CLOAKED`，虚拟桌面/UWP）、工具窗口、零尺寸；边界优先 `DWMWA_EXTENDED_FRAME_BOUNDS`（GetWindowRect 含 Win10/11 左右下各约 7px 不可见调整边框），失败回退。
- 保存前先 `Hide()` 覆盖层（AlwaysOnTop 会挡住对话框），对话框关闭后（保存 / 失败 / 取消）退出截图；文件写入用 `fsx.WriteFile`。
- UAC 提权窗口会截成黑屏（GDI 限制），错误不阻断，作为已知限制记录。

## 4. 模块结构与配置

```
internal/features/screenshot/
  module.go             契约实现（Toggleable + GlobalHotkeyer + Closer）+ 设置页 View + 配置解析
  flow.go               触发流程：抓屏 → 覆盖层 → 完成 / 保存 / 取消（全部走 task.Go）
  capture.go            kbinani/screenshot 逐显示器抓屏 + windowRect 命中（capture_windows.go 枚举 / capture_other.go 兜底）
  overlay.go            overlay 状态结构体与生命周期（开窗 / 关窗 / 每帧入口）
  input.go              交互状态机：指针事件记录、选择态 / 标注态分派、马赛克涂抹、文本提交
  adjust.go             选区 8 向缩放 / 平移、Shape 命中与移动
  toolbar.go            真实 mygoui 组件工具栏（主工具栏 + 子属性栏 + 就地文本编辑）
  shape.go              Shape 模型与有序列表（三段式交互、撤销 / 重做栈、平移、马赛克块生成）
  tools.go              工具 / 色板 / 线宽与字号范围常量
  geometry.go           DIP ↔ 物理像素换算与纯数学 helper（clamp / normRect / roundDiv…）
  paint.go              整帧绘制编排（Z-Order）+ Shape 矢量预览（不落位图）
  render.go             光栅化：image/draw 合成 Shape 到底图 + PNG 编码
  font_windows.go       文本光栅化字体加载（Windows 系统字体；font_other.go 兜底）
  locales/              中英文文案
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

## 5. 明确不做（首版）

- Shape 控制点改形（选中移动已实现；缩放单个 Shape 的控制点未做）。
- 放大镜、实时屏幕（非冻结预览）。
- 滚动截屏、长截图、GIF / 录屏、OCR。
- macOS / Linux（文本标注依赖 Windows 系统字体加载）。

## 6. 实现备注

- 后台任务全部走 `task.Go`（抓屏、渲染、剪贴板、另存为）；`cancel` 经 Runner 绕回主线程帧外再销毁覆盖窗口，避免在绘制回调里 `Destroy`。
- 覆盖层交互：全屏 `ui.Box` 自绘画布，指针事件走 `HandleInput`（画布需要每一个指针事件，含未按压的移动；`Pressed()` 只在构建期查询才生效，绘制期查询收不到按压路由），Enter / Ctrl+S / Esc 走 `OnWindowShortcut`（窗口级，与焦点无关）。
- 箭头覆盖层预览用两段折线近似 V 形头（Painter 无多边形填充），光栅化阶段画实心三角头。
- 已知限制：UAC 提权窗口截成黑屏（GDI 限制）；失败仅记日志（`slog`），首版不做 Toast。
