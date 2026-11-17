package task

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// newTestRunner 返回一个把 post 投递到 channel 的 Runner，测试里手动执行，
// 从而精确控制“主线程”何时看到结果。
func newTestRunner() (*Runner, chan func()) {
	posts := make(chan func(), 8)
	return NewRunner(func(fn func()) { posts <- fn }), posts
}

func runPost(t *testing.T, posts chan func()) {
	t.Helper()
	select {
	case fn := <-posts:
		fn()
	case <-time.After(2 * time.Second):
		t.Fatal("task did not post a result")
	}
}

func TestGoDeliversResult(t *testing.T) {
	r, posts := newTestRunner()
	defer r.Close()

	res := Go(r, "k", func(context.Context) (int, error) { return 42, nil })
	if !res.Running() {
		t.Fatal("Running() = false before the result is posted")
	}

	runPost(t, posts)
	if res.Running() {
		t.Fatal("Running() = true after the result is posted")
	}
	if v, ok := res.Value(); !ok || v != 42 {
		t.Fatalf("Value() = %d, %t; want 42, true", v, ok)
	}
	if err := res.Err(); err != nil {
		t.Fatalf("Err() = %v", err)
	}
}

func TestSupersededResultIsDropped(t *testing.T) {
	r, posts := newTestRunner()
	defer r.Close()

	first := Go(r, "k", func(ctx context.Context) (int, error) {
		<-ctx.Done()
		return 1, ctx.Err()
	})
	second := Go(r, "k", func(context.Context) (int, error) { return 2, nil })

	runPost(t, posts)
	runPost(t, posts)

	if v, ok := second.Value(); !ok || v != 2 {
		t.Fatalf("second Value() = %d, %t; want 2, true", v, ok)
	}
	if _, ok := first.Value(); ok {
		t.Fatal("superseded task kept its result")
	}
}

func TestCancelDropsResult(t *testing.T) {
	r, posts := newTestRunner()
	defer r.Close()

	res := Go(r, "k", func(ctx context.Context) (int, error) {
		<-ctx.Done()
		return 1, ctx.Err()
	})
	res.Cancel()
	runPost(t, posts)

	if res.Running() {
		t.Fatal("Running() = true after Cancel")
	}
	if _, ok := res.Value(); ok {
		t.Fatal("canceled task kept its result")
	}
}

func TestGoReturnsError(t *testing.T) {
	r, posts := newTestRunner()
	defer r.Close()

	want := errors.New("boom")
	res := Go(r, "k", func(context.Context) (int, error) { return 0, want })
	runPost(t, posts)

	if !errors.Is(res.Err(), want) {
		t.Fatalf("Err() = %v, want %v", res.Err(), want)
	}
	if _, ok := res.Value(); ok {
		t.Fatal("failed task reported a value")
	}
}

func TestGoRecoversPanic(t *testing.T) {
	r, posts := newTestRunner()
	defer r.Close()

	res := Go(r, "k", func(context.Context) (int, error) { panic("boom") })
	runPost(t, posts)

	if res.Running() {
		t.Fatal("Running() = true after a panic")
	}
	if err := res.Err(); err == nil || !strings.Contains(err.Error(), "panic") {
		t.Fatalf("Err() = %v, want a panic error", err)
	}
	if _, ok := res.Value(); ok {
		t.Fatal("panicked task reported a value")
	}
}
