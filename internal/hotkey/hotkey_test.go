package hotkey

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestFormatParseRoundTrip(t *testing.T) {
	for _, acc := range []string{"Ctrl+Alt+Space", "Ctrl+Shift+A", "Alt+F1", "Ctrl+Escape"} {
		h := Parse(acc)
		if h.Key == ui.KeyUnknown {
			t.Fatalf("Parse(%q) 失败", acc)
		}
		if got := Format(h); got != acc {
			t.Fatalf("Format(Parse(%q)) = %q", acc, got)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	for _, acc := range []string{"", "F1", "Ctrl+NoSuchKey"} {
		if h := Parse(acc); h.Key != ui.KeyUnknown {
			t.Fatalf("Parse(%q) = %+v, want 零值", acc, h)
		}
	}
}
