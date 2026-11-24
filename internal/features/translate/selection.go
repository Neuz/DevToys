package translate

import (
	"context"
	"errors"
	"strings"
)

var (
	// ErrNoSelection 表示前台应用当前没有可读取的选中文本。
	ErrNoSelection = errors.New("未读取到选中文字")
	// ErrUnsupported 表示当前系统没有可用的跨应用取词实现。
	ErrUnsupported = errors.New("当前系统暂不支持跨应用取词")
)

// Point 是屏幕坐标（物理像素）。
type Point struct{ X, Y int }

// Selection 是一次取词结果。
type Selection struct {
	Text  string
	Point Point
}

// readSlot 是容量 1 的并发槽：UI Automation 不适合多线程同时读取。
var readSlot = make(chan struct{}, 1)

// Read 读取前台应用的选区。原生读取可能阻塞，因此放在独立 goroutine；
// ctx 取消时立即返回，后台读取结束后仍会释放并发槽。
func Read(ctx context.Context) (Selection, error) {
	select {
	case readSlot <- struct{}{}:
	case <-ctx.Done():
		return Selection{}, ctx.Err()
	}
	done := make(chan struct{})
	var selection Selection
	var err error
	go func() {
		defer func() {
			<-readSlot
			close(done)
		}()
		selection, err = readNative()
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return Selection{}, ctx.Err()
	}
	selection.Text = strings.TrimSpace(selection.Text)
	if err != nil {
		return Selection{}, err
	}
	if selection.Text == "" {
		return Selection{}, ErrNoSelection
	}
	return selection, nil
}
