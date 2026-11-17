package welcome

import (
	"context"
	"time"

	"github.com/Neuz/DevToys/internal/task"
)

// demoKey 是演示任务在 Runner 上的 key。
const demoKey = "demo"

// runDemo 启动演示任务。同一 key 的新任务会取消旧任务；结果经 Runner 的 post
// 回到主线程，View 在下一帧读到。
func (m *Module) runDemo() {
	start := time.Now()
	m.log.Info("demo: task started", "key", demoKey)
	m.demo = task.Go(m.tasks, demoKey,
		func(ctx context.Context) (string, error) {
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				m.log.Debug("demo: task cancelled", "key", demoKey)
				return "", ctx.Err()
			}
			elapsed := time.Since(start).Round(100 * time.Millisecond)
			m.log.Info("demo: task finished", "key", demoKey, "elapsed", elapsed)
			return elapsed.String(), nil
		})
}
