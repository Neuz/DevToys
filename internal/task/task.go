// Package task 在后台 goroutine 中执行任务，并把结果送回主线程。
//
// 生命周期：每个 Runner 属于一个窗口或面板，NewRunner 收到的 post 负责把
// 函数放到主线程执行（通常是 mygo.Window.Update）。任务的结果只在主线程
// 写入，因此界面可以在帧里安全地读取。Close 会取消所有任务并等待它们的
// goroutine 退出；调用方必须在主线程调用它，且不应在等待期间阻塞 post。
package task

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
)

// Runner 拥有若干按 key 区分的任务，同一个 key 的新任务会顶替旧任务。
type Runner struct {
	post func(func())

	mu     sync.Mutex
	jobs   map[string]*job
	next   uint64
	closed bool
	wg     sync.WaitGroup
}

type job struct {
	gen    uint64
	cancel context.CancelFunc
}

// NewRunner 返回一个 Runner；nil 的 post 表示直接执行（仅用于测试）。
func NewRunner(post func(func())) *Runner {
	if post == nil {
		post = func(fn func()) { fn() }
	}
	return &Runner{post: post, jobs: make(map[string]*job)}
}

// Go 用 fn 顶替 key 上的旧任务：旧任务被取消，其结果被丢弃。
//
// fn 返回后，结果经 post 回到主线程；若任务已被顶替、取消或 Runner 已关闭，
// 结果不会保留在 Result 上。
func Go[T any](r *Runner, key string, fn func(context.Context) (T, error)) *Result[T] {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Result[T]{runner: r, key: key, running: true}
	if r == nil {
		cancel()
		s.running = false
		return s
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		cancel()
		s.mu.Lock()
		s.running = false
		s.done = true
		s.mu.Unlock()
		return s
	}
	if old := r.jobs[key]; old != nil {
		old.cancel()
	}
	r.next++
	s.gen = r.next
	r.jobs[key] = &job{gen: s.gen, cancel: cancel}
	r.wg.Add(1)
	r.mu.Unlock()

	go func() {
		defer r.wg.Done()
		v, err := call(ctx, fn)
		r.post(func() { s.finish(v, err) })
	}()
	return s
}

// call 执行 fn，并把 panic 转成错误：fn 由功能模块提供，一个 bug 不应该带走
// 整个应用，也不应该让 Result 永远停在 running。
func call[T any](ctx context.Context, fn func(context.Context) (T, error)) (v T, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("task: panic: %v\n%s", p, debug.Stack())
		}
	}()
	return fn(ctx)
}

// Result 是一个后台任务的结果。
type Result[T any] struct {
	runner *Runner
	key    string
	gen    uint64

	mu      sync.Mutex
	running bool
	done    bool
	value   T
	err     error
	ok      bool
}

func (s *Result[T]) finish(v T, err error) {
	s.mu.Lock()
	if s.done || s.gen == 0 {
		s.mu.Unlock()
		return
	}
	gen := s.gen
	s.mu.Unlock()

	current := false
	if r := s.runner; r != nil {
		r.mu.Lock()
		if j := r.jobs[s.key]; j != nil && j.gen == gen && !r.closed {
			current = true
			delete(r.jobs, s.key)
		}
		r.mu.Unlock()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done || s.gen != gen {
		return
	}
	s.done = true
	s.running = false
	if current {
		s.value, s.err, s.ok = v, err, err == nil
	}
}

// Running 报告任务是否仍在运行。
func (s *Result[T]) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Value 返回任务保留的结果；ok 为 false 表示还没有结果，或结果已被丢弃。
func (s *Result[T]) Value() (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.value, s.ok
}

// Err 返回任务保留的错误；结果被丢弃时为 nil。
func (s *Result[T]) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Cancel 取消任务；它之后返回的结果会被丢弃。
func (s *Result[T]) Cancel() {
	s.mu.Lock()
	if s.done {
		s.mu.Unlock()
		return
	}
	gen := s.gen
	s.mu.Unlock()

	if r := s.runner; r != nil {
		r.mu.Lock()
		if j := r.jobs[s.key]; j != nil && j.gen == gen {
			j.cancel()
			delete(r.jobs, s.key)
		}
		r.mu.Unlock()
	}

	s.mu.Lock()
	if s.gen == gen && !s.done {
		s.running = false
		s.done = true
	}
	s.mu.Unlock()
}

// Running 报告 key 上是否还有任务在运行。
func (r *Runner) Running(key string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.jobs[key] != nil
}

// Close 取消所有任务并等待它们的 goroutine 退出。重复调用没有额外效果。
func (r *Runner) Close() {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	for key, j := range r.jobs {
		j.cancel()
		delete(r.jobs, key)
	}
	r.mu.Unlock()
	r.wg.Wait()
}
