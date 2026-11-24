package translate

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type fakeProvider struct {
	id    string
	delay time.Duration
	err   error
	text  string
}

func (p *fakeProvider) ID() string { return p.id }

func (p *fakeProvider) Name() string { return p.id }

func (p *fakeProvider) Translate(ctx context.Context, req Request) (Result, error) {
	if p.delay > 0 {
		select {
		case <-time.After(p.delay):
		case <-ctx.Done():
			return Result{}, ctx.Err()
		}
	}
	if p.err != nil {
		return Result{}, p.err
	}
	text := p.text
	if text == "" {
		text = p.id + ":" + req.Text
	}
	return Result{Provider: p.id, Text: text}, nil
}

func TestServiceTranslateAllProviders(t *testing.T) {
	svc := &Service{
		providers: []Provider{
			&fakeProvider{id: "a"},
			&fakeProvider{id: "b"},
			&fakeProvider{id: "c"},
		},
		base: context.Background(),
	}
	results := make(chan string, 3)
	gen := svc.Translate("hello", "auto", "auto", func(got uint64, p Provider, res Result, err error) {
		if err != nil {
			t.Errorf("%s: %v", p.ID(), err)
		}
		results <- res.Text
	})
	if gen == 0 {
		t.Fatal("gen should be non-zero")
	}
	seen := make(map[string]bool)
	for i := 0; i < 3; i++ {
		select {
		case text := <-results:
			seen[text] = true
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for results")
		}
	}
	for _, id := range []string{"a", "b", "c"} {
		if !seen[id+":hello"] {
			t.Fatalf("missing result %q, got %v", id, seen)
		}
	}
}

func TestServiceFailureIsolation(t *testing.T) {
	svc := &Service{
		providers: []Provider{
			&fakeProvider{id: "bad", err: errors.New("boom")},
			&fakeProvider{id: "good"},
		},
		base: context.Background(),
	}
	type outcome struct {
		id  string
		err error
	}
	done := make(chan outcome, 2)
	svc.Translate("hi", "auto", "auto", func(_ uint64, p Provider, _ Result, err error) {
		done <- outcome{id: p.ID(), err: err}
	})
	got := make(map[string]error)
	for i := 0; i < 2; i++ {
		select {
		case out := <-done:
			got[out.id] = out.err
		case <-time.After(time.Second):
			t.Fatal("timed out")
		}
	}
	if got["bad"] == nil || got["good"] != nil {
		t.Fatalf("outcomes = %v", got)
	}
}

func TestServiceCancelDropsResults(t *testing.T) {
	svc := &Service{
		providers: []Provider{&fakeProvider{id: "slow", delay: 80 * time.Millisecond}},
		base:      context.Background(),
	}
	var calls atomic.Int32
	svc.Translate("hi", "auto", "auto", func(uint64, Provider, Result, error) { calls.Add(1) })
	svc.Cancel()
	time.Sleep(200 * time.Millisecond)
	if n := calls.Load(); n != 0 {
		t.Fatalf("emitted %d results after Cancel", n)
	}
}

func TestServiceEmptyInput(t *testing.T) {
	svc := &Service{providers: []Provider{&fakeProvider{id: "a"}}, base: context.Background()}
	var calls atomic.Int32
	svc.Translate("   ", "auto", "auto", func(uint64, Provider, Result, error) { calls.Add(1) })
	time.Sleep(30 * time.Millisecond)
	if n := calls.Load(); n != 0 {
		t.Fatalf("emitted %d results for empty input", n)
	}
}

func TestServiceNoProviders(t *testing.T) {
	svc := &Service{base: context.Background()}
	var calls atomic.Int32
	svc.Translate("hi", "auto", "auto", func(uint64, Provider, Result, error) { calls.Add(1) })
	time.Sleep(30 * time.Millisecond)
	if n := calls.Load(); n != 0 {
		t.Fatalf("emitted %d results without providers", n)
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"chinese", "你好，世界", "zh-Hans"},
		{"japanese", "こんにちは世界", "ja"},
		{"korean", "안녕하세요", "ko"},
		{"russian", "Привет, мир", "ru"},
		{"english", "Hello, world", "en"},
		{"symbols", "123 !?", "en"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detectLanguage(tt.text); got != tt.want {
				t.Fatalf("detectLanguage(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestResolveLanguagesFrom(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		source     string
		target     string
		wantSource string
		wantTarget string
	}{
		{"explicit source", "hello", "ja", "zh-Hans", "ja", "zh-Hans"},
		{"explicit source ignored detection", "こんにちは", "en", "auto", "en", "zh-Hans"},
		{"auto source", "hello", "auto", "ja", "en", "ja"},
		{"same source and target swaps", "hello", "en", "en", "en", "zh-Hans"},
		{"same target swaps to chinese", "hello", "auto", "en", "en", "zh-Hans"},
		{"empty source detects", "你好", "", "auto", "zh-Hans", "en"},
		{"empty target", "hello", "auto", "", "en", "zh-Hans"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, target := resolveLanguagesFrom(tt.text, tt.source, tt.target)
			if source != tt.wantSource || target != tt.wantTarget {
				t.Fatalf("resolveLanguagesFrom(%q, %q, %q) = (%q, %q), want (%q, %q)",
					tt.text, tt.source, tt.target, source, target, tt.wantSource, tt.wantTarget)
			}
		})
	}
}
