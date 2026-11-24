# 启动器（launcher）

全局快速启动面板：快捷键唤起置顶浮层，输入关键字查找并启动开始菜单应用或 Windows 系统设置页。

- 代码：[internal/features/launcher/](../../internal/features/launcher/)
- 入口：侧栏「启动器」进入设置页；全局快捷键直接唤起浮层面板

## 入口与快捷键

模块实现 `contract.Module`、`contract.Toggleable`、`contract.GlobalHotkeyer` 与 `contract.Closer`，见 [module.go](../../internal/features/launcher/module.go)。

| 项目 | 值 |
| --- | --- |
| 模块 ID | `launcher` |
| 分组 | 空字符串（侧栏不显示分组标题） |
| 图标 | `lucide.Search()` |
| 热键 ID | `toggle` |
| 默认快捷键 | `Alt+F` |

快捷键取值规则：

- `features.launcher.hotkeys.toggle` **不存在** → 使用默认值 `Alt+F`；
- 键存在但值为空串 → 用户显式删除，不注册任何快捷键，也不回退默认值；
- 设置页清空录制后保存空串，面板此后只能进入启动器页点【触发】打开；
- 保存失败会回滚内存中的值并记录错误，成功后调用 `Host.HotkeysChanged()` 让外壳立即重绑。

启用开关位于启动器设置页，写入 `features.launcher.enabled`。禁用后外壳跳过它的全局快捷键；设置页保留开关以便重新启用。

## 索引

索引由 [index.go](../../internal/features/launcher/index.go) 构建，来源有两类：

1. **开始菜单应用**：`mygo.Shell.InstalledApps()` 返回的桌面应用与打包应用，跳过没有路径的项，按 `ID`（缺失时用路径）去重。
2. **内置条目**（仅 Windows）：
   - 31 个常用 `ms-settings:` 设置页；
   - `mstsc.exe`（远程桌面连接）等拿不到开始菜单快捷方式的系统程序；exe 不存在时跳过。
   
   内置条目的标题按当前语言读取，关键词同时写中英文，切换语言后仍可搜索。设置页归入「系统」分类，`mstsc` 归入「应用」。

组装规则：

- 内置条目排在应用之前，随后按标题去重（不区分大小写），因此同名时优先保留内置条目；
- 排除词作用于标题，命中任一关键词（不区分大小写）即丢弃；
- 每个条目的关键词包含：标题、别名、exe 名、拼音全拼与首字母。桌面应用别名是可执行文件名（去掉扩展名），打包应用别名是去掉 `!App` 的包标识。

默认排除词：`uninstall`、`卸载`、`readme`、`help`、`帮助`、`说明`、`文档`。

## 搜索与排序

搜索由 [search.go](../../internal/features/launcher/search.go) 实现：

- 查询按空白分词并转小写，每个词都必须作为子串出现在「标题 + 关键词」中（AND 语义）；
- 空查询不返回结果，面板只显示搜索框；
- 分类过滤：全部 / 应用（`app`）/ 系统（`system`）；
- 排序：使用次数降序 → 最近使用时间降序 → 标题长度升序 → 标题字典序；没有任何使用记录时按标题字典序；
- 结果截断到 `max_results`（默认 8）。

使用记录以条目路径为 key，只有在启动成功后才累加，见 [launch.go](../../internal/features/launcher/launch.go)。

## 面板

面板生命周期由 [panel.go](../../internal/features/launcher/panel.go) 管理，内容由 [panel_view.go](../../internal/features/launcher/panel_view.go) 渲染。

- **窗口**：首次唤起时创建，宽 680、无边框、置顶、不占任务栏、不可缩放；隐藏不销毁，之后复用。
- **定位**：显示在鼠标所在显示器的工作区中央，并向上偏移约工作区高度的 1/5。
- **高度**：搜索框固定 36 DIP，分类控件 34 DIP，每行 44 DIP；最多展示 8 行，超出后列表内部滚动；高度只在输入变化时重算，避免重复 `SetSize`。
- **内容**：空查询只有搜索框；输入后出现【全部】【应用】【系统】分类与结果；当前分类无结果时保留搜索框与分类。
- **键盘**：方向键移动高亮，回车启动，`Esc` 在查询非空时先清空查询，空查询时隐藏面板；列表由搜索框驱动，不作为第二个 Tab 停留点。
- **隐藏**：关闭事件被拦截，只隐藏窗口；失焦后延迟 100 ms 隐藏，若期间重新获得焦点则保持显示。
- **图标**：应用图标通过 `mygo.Shell.FileIcon` 按显示缩放取 PNG，解码后按路径缓存；取不到时用名称首字头像；系统设置条目固定使用设置图标。

## 持久化

[store.go](../../internal/features/launcher/store.go) 使用 bbolt，数据库位于 `mygo.App.Path(mygo.PathUserData)` 下的 `launcher.db`：

- `entries` 桶：索引条目，以路径为 key，每次重建整桶替换；
- `usage` 桶：使用记录，以路径为 key，值为次数与最后使用时间。

启动时先同步读取缓存，保证首帧可用；随后后台重新枚举应用并替换。早期缓存中没有 `ID` 的条目会被忽略，等后台重建补全。数据库打不开时降级为不持久化，功能仍可使用。

## 设置页与配置

设置页由 [view.go](../../internal/features/launcher/view.go) 渲染：

| 行 | 行为 |
| --- | --- |
| 启用启动器 | 写入 `enabled`，让外壳立即重绑快捷键 |
| 触发快捷键 | `form.HotkeyInput` 录制；清空即删除快捷键 |
| 显示结果数量 | `form.NumberField`，写入 `options.max_results` |
| 触发 | 直接打开面板 |
| 索引 | 【重建索引】按钮，完成后弹 Toast（成功或失败） |
| 排除关键词 | `form.TagsInput`，修改后重建索引 |

配置结构：

```json
{
  "features": {
    "launcher": {
      "enabled": true,
      "hotkeys": { "toggle": "Alt+F" },
      "options": {
        "max_results": 8,
        "exclude": ["uninstall", "卸载", "readme", "help", "帮助", "说明", "文档"]
      }
    }
  }
}
```

`max_results <= 0` 时回退默认 8；`exclude` 为 `null` 时回退默认列表；缺失或解析失败时整份 options 使用默认值。修改排除词会触发后台重建，修改结果数量只影响查询，无需重建。

## 生命周期

- `Init`：打开数据库、解析当前快捷键、读取 options 与使用记录、同步加载缓存、启动一次静默后台重建。
- `Close`：销毁面板窗口、关闭数据库；由外壳在退出时调用。
- 索引任务 key 为 `index`，新任务会取消旧任务；只有手动点击【重建索引】的那次会弹 Toast。

相关限制见 [开发进度](../progress.md#已知问题)。
