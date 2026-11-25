package screenshot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Neuz/DevToys/internal/features/screenshot/locales"
	"github.com/Neuz/DevToys/internal/fsx"
	"github.com/Neuz/DevToys/internal/task"
	"github.com/egoist/mygo"
)

// 流程参数。
const (
	// captureTimeout 是抓屏 + 窗口枚举的整体超时。
	captureTimeout = 5 * time.Second
	// minDragPixels 是拖拽超过多少物理像素才算框选。
	minDragPixels = 4
)

// toggle 是快捷键入口：先抓屏，再创建覆盖层。
func (m *Module) toggle() {
	if m.capturing {
		return
	}
	if len(m.overlays) > 0 {
		// 已在截图态，再次触发取消。
		m.cancel()
		return
	}
	m.capturing = true
	if m.tasks == nil {
		m.capturing = false
		return
	}
	task.Go(m.tasks, "capture", func(ctx context.Context) (struct{}, error) {
		ctx, cancel := context.WithTimeout(ctx, captureTimeout)
		defer cancel()
		err := m.startCapture(ctx)
		if ctx.Err() != nil {
			return struct{}{}, ctx.Err()
		}
		mygo.RunOnMain(func() {
			m.capturing = false
			if err != nil {
				m.log.Error("screenshot: 启动截图失败", "error", err)
			}
		})
		return struct{}{}, nil
	})
}

// startCapture 在 Runner goroutine 里抓屏、枚举窗口、建覆盖层。
// 抓屏必须发生在覆盖窗口显示之前，否则会截到覆盖层自己。
func (m *Module) startCapture(ctx context.Context) error {
	shots, err := captureDisplays()
	if err != nil {
		return err
	}
	windows, _ := listWindows()
	done := make(chan struct{})
	mygo.RunOnMain(func() {
		m.openOverlay(shots, mygo.Screen.Displays())
		for _, ov := range m.overlays {
			ov.windows = windows
		}
		close(done)
	})
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// cancel 关闭全部覆盖层（选择态 Esc/右键/再次触发快捷键）。
// 触发点可能在窗口自身的绘制回调里，不能当场销毁窗口：
// 经 task.Go 绕到 Runner goroutine，再用 RunOnMain 排回主线程帧外执行。
func (m *Module) cancel() {
	if m.tasks == nil {
		m.closeOverlays()
		return
	}
	task.Go(m.tasks, "cancel", func(ctx context.Context) (struct{}, error) {
		mygo.RunOnMain(m.closeOverlays)
		return struct{}{}, nil
	})
}

// finish 由覆盖层在标注态点「完成」或按 Enter 调用：
// 光栅化全部 Shape 合成 → PNG → 剪贴板。
func (m *Module) finish(ov *overlay) {
	region := ov.region
	shapes := ov.shapes.committed()
	shot := ov.shot
	if m.tasks == nil {
		return
	}
	task.Go(m.tasks, "finish", func(ctx context.Context) (struct{}, error) {
		png, err := rasterize(shot, region, shapes)
		if err == nil {
			// mygo.Clipboard.WriteImage 内部自己分发到主线程，
			// PNG 进出会转成系统位图格式。
			err = mygo.Clipboard.WriteImage(png)
		}
		if ctx.Err() != nil {
			return struct{}{}, ctx.Err()
		}
		mygo.RunOnMain(func() {
			if err != nil {
				m.log.Error("screenshot: 写入剪贴板失败", "error", err)
			}
			m.closeOverlays()
		})
		return struct{}{}, nil
	})
}

// saveBy 由覆盖层在标注态点「保存」或按 Ctrl+S 调用：渲染后弹另存为对话框。
// 对话框会被 AlwaysOnTop 覆盖层挡住，弹出前先隐藏覆盖层；
// 对话框关闭后（保存 / 失败 / 取消）退出截图。
func (m *Module) saveBy(ov *overlay) {
	region := ov.region
	shapes := ov.shapes.committed()
	shot := ov.shot
	if m.tasks == nil {
		return
	}
	task.Go(m.tasks, "save", func(ctx context.Context) (struct{}, error) {
		hidden := make(chan struct{})
		mygo.RunOnMain(func() {
			for _, o := range m.overlays {
				if o.window != nil {
					o.window.Hide()
				}
			}
			close(hidden)
		})
		<-hidden

		png, err := rasterize(shot, region, shapes)
		if err == nil {
			err = saveAsDialog(png)
		}
		if ctx.Err() != nil {
			return struct{}{}, ctx.Err()
		}
		mygo.RunOnMain(func() {
			if err != nil && err != errSaveCanceled {
				m.log.Error("screenshot: 保存失败", "error", err)
			}
			// 无论保存、失败还是取消，都退出截图（取消时覆盖层已被隐藏）。
			m.closeOverlays()
		})
		return struct{}{}, nil
	})
}

// errSaveCanceled 表示用户在另存为对话框点了取消。
var errSaveCanceled = fmt.Errorf("取消保存")

// saveAsDialog 弹系统另存为对话框并写 PNG 文件。
// mygo.Dialog.Save 阻塞到对话框关闭，可在任意 goroutine 调用；
// 返回空路径表示用户取消。
func saveAsDialog(png []byte) error {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	name := fmt.Sprintf("screenshot_%s.png", time.Now().Format("20060102_150405"))
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Title:       locales.T("save_dialog"),
		DefaultPath: filepath.Join(home, name),
		Filters:     []mygo.FileFilter{{Name: locales.T("png_filter"), Extensions: []string{"png"}}},
	})
	if err != nil {
		return err
	}
	if path == "" {
		return errSaveCanceled
	}
	if filepath.Ext(path) == "" {
		path += ".png"
	}
	return fsx.WriteFile(path, png, 0o644)
}
