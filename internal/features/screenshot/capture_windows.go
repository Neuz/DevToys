//go:build windows

package screenshot

import (
	"fmt"
	"image"
	"syscall"
	"unsafe"
)

var (
	user32             = syscall.NewLazyDLL("user32.dll")
	dwmapi             = syscall.NewLazyDLL("dwmapi.dll")
	procEnumWindows    = user32.NewProc("EnumWindows")
	procIsWindowVis    = user32.NewProc("IsWindowVisible")
	procGetWindowRect  = user32.NewProc("GetWindowRect")
	procIsIconic       = user32.NewProc("IsIconic")
	procGetWindowTextW = user32.NewProc("GetWindowTextW")
	procGetWindowLongW = user32.NewProc("GetWindowLongW")
	procGetClassNameW  = user32.NewProc("GetClassNameW")
	procDwmGetWindow   = dwmapi.NewProc("DwmGetWindowAttribute")
)

// 顶层窗口扩展样式；DWM 窗口属性。
const (
	gwlExStyle              = -20
	wsExToolWindow          = 0x00000080
	dwmwExtendedFrameBounds = 9
	dwmwCloaked             = 13
)

// listWindows 枚举可见顶层窗口，按 z-order 返回候选矩形。
// 过滤不可见、最小化、被 DWM 遮蔽（cloaked，如虚拟桌面/UWP 切换中的窗口）、
// 工具窗口、零尺寸与整屏（桌面）窗口。
// COM/Win32 阻塞调用按架构约束的特例在独立线程执行，
// 由调用方放在 task.Go 的 goroutine 里。
func listWindows() ([]windowRect, error) {
	var windows []windowRect
	enumWindows(func(hwnd uintptr) bool {
		if !isWindowVisible(hwnd) {
			return true
		}
		if isIconic(hwnd) {
			return true
		}
		if isCloaked(hwnd) {
			return true
		}
		ex := getWindowLong(hwnd, gwlExStyle)
		if ex&wsExToolWindow != 0 {
			return true
		}
		title := windowTitle(hwnd)
		if title == "" && !classInteresting(hwnd) {
			return true
		}
		r, ok := windowBounds(hwnd)
		if !ok || r.Dx() <= 8 || r.Dy() <= 8 {
			return true
		}
		windows = append(windows, windowRect{bounds: r})
		return true
	})
	if len(windows) == 0 {
		return nil, fmt.Errorf("没有可识别的窗口")
	}
	return windows, nil
}

// isIconic 报告窗口是否最小化。
func isIconic(hwnd uintptr) bool {
	r, _, _ := procIsIconic.Call(hwnd)
	return r != 0
}

// isCloaked 报告窗口是否被 DWM 遮蔽（虚拟桌面、UWP 宿主等场景下
// IsWindowVisible 仍为真但实际不可见）。查询失败按未遮蔽处理。
func isCloaked(hwnd uintptr) bool {
	var cloaked uint32
	r, _, _ := procDwmGetWindow.Call(hwnd, dwmwCloaked,
		uintptr(unsafe.Pointer(&cloaked)), unsafe.Sizeof(cloaked))
	return r == 0 && cloaked != 0
}

func enumWindows(fn func(hwnd uintptr) bool) {
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		if fn(hwnd) {
			return 1
		}
		return 0
	})
	procEnumWindows.Call(cb, 0)
}

func isWindowVisible(hwnd uintptr) bool {
	visible, _, _ := procIsWindowVis.Call(hwnd)
	return visible != 0
}

// getWindowLong 读窗口长整型属性；GetWindowLongW 失败时返回 0，
// 对样式过滤场景 0 值是安全的（不会误判为 ToolWindow）。
func getWindowLong(hwnd uintptr, index int) int32 {
	value, _, _ := procGetWindowLongW.Call(hwnd, uintptr(index))
	return int32(value)
}

// windowBounds 返回窗口的可见矩形。GetWindowRect 含 Win10/11 不可见
// 调整边框（左右下各约 8px），优先用 DWM 扩展框架边界贴合视觉尺寸，
// 失败（如 UWP 核心窗口）时回退 GetWindowRect。
func windowBounds(hwnd uintptr) (image.Rectangle, bool) {
	type nativeRect struct{ Left, Top, Right, Bottom int32 }
	var ext nativeRect
	if r1, _, _ := procDwmGetWindow.Call(hwnd, dwmwExtendedFrameBounds,
		uintptr(unsafe.Pointer(&ext)), unsafe.Sizeof(ext)); r1 == 0 {
		if er := toImageRect(ext); !er.Empty() {
			return er, true
		}
	}
	var rect nativeRect
	r1, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	if r1 == 0 {
		return image.Rectangle{}, false
	}
	return toImageRect(rect), true
}

func toImageRect(r struct{ Left, Top, Right, Bottom int32 }) image.Rectangle {
	return image.Rect(int(r.Left), int(r.Top), int(r.Right), int(r.Bottom))
}

func windowTitle(hwnd uintptr) string {
	buf := make([]uint16, 256)
	n, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

// classInteresting 保留无标题但值得选中的窗口类（桌面外壳、浏览器等）。
func classInteresting(hwnd uintptr) bool {
	buf := make([]uint16, 128)
	procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	switch syscall.UTF16ToString(buf) {
	case "CabinetWClass", "Chrome_WidgetWin_1", "MozillaWindowClass":
		return true
	}
	return false
}
