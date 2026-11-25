//go:build !windows

package screenshot

// listWindows 在非 Windows 平台不支持窗口识别。
func listWindows() ([]windowRect, error) {
	return nil, ErrUnsupported
}
