//go:build !windows

package translate

// readNative 在非 Windows 平台上不可用。
func readNative() (Selection, error) { return Selection{}, ErrUnsupported }
