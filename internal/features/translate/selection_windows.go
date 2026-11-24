//go:build windows

package translate

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	ole32            = syscall.NewLazyDLL("ole32.dll")
	oleaut32         = syscall.NewLazyDLL("oleaut32.dll")
	getForeground    = user32.NewProc("GetForegroundWindow")
	getWindowPID     = user32.NewProc("GetWindowThreadProcessId")
	getCursorPos     = user32.NewProc("GetCursorPos")
	coInitializeEx   = ole32.NewProc("CoInitializeEx")
	coUninitialize   = ole32.NewProc("CoUninitialize")
	coCreateInstance = ole32.NewProc("CoCreateInstance")
	sysStringLen     = oleaut32.NewProc("SysStringLen")
	sysFreeString    = oleaut32.NewProc("SysFreeString")
)

type guid struct {
	Data1        uint32
	Data2, Data3 uint16
	Data4        [8]byte
}

var (
	clsidAutomation  = guid{0xff48dba4, 0x60ef, 0x4201, [8]byte{0xaa, 0x87, 0x54, 0x10, 0x3e, 0xef, 0x59, 0x4e}}
	iidAutomation    = guid{0x30cbe57d, 0xd9d0, 0x452a, [8]byte{0xab, 0x13, 0x7a, 0xc5, 0xac, 0x48, 0x25, 0xee}}
	clsidAutomation8 = guid{0xe22ad333, 0xb25f, 0x460c, [8]byte{0x83, 0xd0, 0x05, 0x81, 0x10, 0x73, 0x95, 0xc9}}
	iidAutomation2   = guid{0x34723aff, 0x0c9d, 0x49d0, [8]byte{0x98, 0x96, 0x7a, 0xb5, 0x2d, 0xf8, 0xcd, 0x8a}}
	iidTextPattern   = guid{0x32eba289, 0x3583, 0x42c9, [8]byte{0x9c, 0x59, 0x3b, 0x6d, 0x9a, 0x1e, 0x9b, 0x6a}}
)

type winPoint struct{ X, Y int32 }

// comObject 保留系统返回的原生指针，避免把 COM 地址当作 Go 整数解引用。
// 不同接口只访问其实际拥有的方法索引；最大接口 IUIAutomationElement 有 85 项。
type comObject struct{ VTable *[85]uintptr }

// comCall 按 Windows SDK 的接口虚表调用 COM 方法；参数中的指针需在调用期间保持存活。
//
//go:uintptrescapes
func comCall(object *comObject, method int, args ...uintptr) uintptr {
	address := object.VTable[method]
	arguments := append([]uintptr{uintptr(unsafe.Pointer(object))}, args...)
	result, _, _ := syscall.SyscallN(address, arguments...)
	return result
}

// releaseCOM 释放一个已取得引用的 COM 对象。
func releaseCOM(object *comObject) {
	if object != nil {
		comCall(object, 2)
	}
}

func failed(result uintptr) bool { return int32(result) < 0 }

// foreground 返回前台窗口及其进程，以免对应用自身触发反馈循环。
func foreground() (uintptr, uint32) {
	window, _, _ := getForeground.Call()
	var pid uint32
	getWindowPID.Call(window, uintptr(unsafe.Pointer(&pid)))
	return window, pid
}

// readNative 通过 UI Automation 读取焦点元素及其少量祖先的选区；
// 不模拟按键、不读写剪贴板。
func readNative() (Selection, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	window, pid := foreground()
	if window == 0 || pid == uint32(os.Getpid()) {
		return Selection{}, ErrNoSelection
	}
	hr, _, _ := coInitializeEx.Call(0, 0) // COINIT_MULTITHREADED
	if failed(hr) {
		return Selection{}, fmt.Errorf("初始化 Windows 取词失败（0x%08X）", uint32(hr))
	}
	defer coUninitialize.Call()
	var automation *comObject
	hr, _, _ = coCreateInstance.Call(uintptr(unsafe.Pointer(&clsidAutomation8)), 0, 1,
		uintptr(unsafe.Pointer(&iidAutomation2)), uintptr(unsafe.Pointer(&automation)))
	if !failed(hr) && automation != nil {
		// IUIAutomation2 的连接与事务超时，避免第三方窗口阻塞取词。
		comCall(automation, 61, 700)
		comCall(automation, 63, 700)
	} else {
		hr, _, _ = coCreateInstance.Call(uintptr(unsafe.Pointer(&clsidAutomation)), 0, 1,
			uintptr(unsafe.Pointer(&iidAutomation)), uintptr(unsafe.Pointer(&automation)))
	}
	if failed(hr) || automation == nil {
		return Selection{}, fmt.Errorf("Windows UI Automation 不可用（0x%08X）", uint32(hr))
	}
	defer releaseCOM(automation)
	var element *comObject
	if failed(comCall(automation, 8, uintptr(unsafe.Pointer(&element)))) || element == nil {
		return Selection{}, ErrNoSelection
	}
	var walker *comObject
	comCall(automation, 14, uintptr(unsafe.Pointer(&walker)))
	defer releaseCOM(walker)
	var text string
	// 文档容器可能持有选区而其子元素持有键盘焦点，只向上查找少量祖先。
	for depth := 0; depth < 6 && element != nil; depth++ {
		var elementPID int32
		comCall(element, 20, uintptr(unsafe.Pointer(&elementPID)))
		if elementPID == int32(os.Getpid()) {
			releaseCOM(element)
			return Selection{}, ErrNoSelection
		}
		text = selectedText(element)
		if strings.TrimSpace(text) != "" {
			releaseCOM(element)
			element = nil
			break
		}
		var parent *comObject
		if walker != nil {
			comCall(walker, 3, uintptr(unsafe.Pointer(element)), uintptr(unsafe.Pointer(&parent)))
		}
		releaseCOM(element)
		element = parent
	}
	releaseCOM(element)
	if current, _ := foreground(); current != window {
		return Selection{}, ErrNoSelection
	}
	if strings.TrimSpace(text) == "" {
		return Selection{}, ErrNoSelection
	}
	var p winPoint
	getCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return Selection{Text: text, Point: Point{int(p.X), int(p.Y)}}, nil
}

// selectedText 通过 TextPattern.GetSelection 读取文本。
func selectedText(element *comObject) string {
	var isPassword int32
	if !failed(comCall(element, 35, uintptr(unsafe.Pointer(&isPassword)))) && isPassword != 0 {
		return ""
	}
	var pattern *comObject
	// GetCurrentPatternAs 可直接请求正确的 IUIAutomationTextPattern 接口。
	if failed(comCall(element, 14, 10014, uintptr(unsafe.Pointer(&iidTextPattern)), uintptr(unsafe.Pointer(&pattern)))) || pattern == nil {
		return ""
	}
	defer releaseCOM(pattern)
	var ranges *comObject
	if failed(comCall(pattern, 5, uintptr(unsafe.Pointer(&ranges)))) || ranges == nil {
		return ""
	}
	defer releaseCOM(ranges)
	var count int32
	if failed(comCall(ranges, 3, uintptr(unsafe.Pointer(&count)))) {
		return ""
	}
	if count <= 0 {
		return ""
	}
	parts := make([]string, 0, min(count, 8))
	for i := int32(0); i < count && i < 8; i++ {
		var textRange *comObject
		if failed(comCall(ranges, 4, uintptr(i), uintptr(unsafe.Pointer(&textRange)))) || textRange == nil {
			continue
		}
		var bstr *uint16
		hr := comCall(textRange, 12, 100000, uintptr(unsafe.Pointer(&bstr)))
		releaseCOM(textRange)
		if bstr == nil {
			continue
		}
		if !failed(hr) {
			length, _, _ := sysStringLen.Call(uintptr(unsafe.Pointer(bstr)))
			if length > 0 && length <= 100000 {
				parts = append(parts, string(utf16.Decode(unsafe.Slice(bstr, length))))
			}
		}
		sysFreeString.Call(uintptr(unsafe.Pointer(bstr)))
	}
	return strings.Join(parts, "\n")
}
