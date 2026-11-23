// Package hotkey 在界面录制的组合键与 mygo 的 accelerator 文本之间转换。
package hotkey

import (
	"runtime"
	"strconv"
	"strings"

	"github.com/Cyenoch/mygoui/form"
	"github.com/egoist/mygo/ui"
)

// Format 把录制的快捷键写成 accelerator 文本，如 Ctrl+Shift+Space；
// 没有录制时返回空串。
func Format(h form.Hotkey) string {
	if h.Mods == 0 {
		return ""
	}
	keys := h.Keys()
	if len(keys) == 0 {
		return ""
	}
	parts := make([]string, 0, len(keys))
	parts = append(parts, keys[:len(keys)-1]...)
	key := keys[len(keys)-1]
	if name, ok := acceleratorKeys[key]; ok {
		key = name
	}
	return strings.Join(append(parts, key), "+")
}

// Parse 读回 accelerator 文本；无法识别时返回零值。
func Parse(acc string) form.Hotkey {
	parts := strings.Split(acc, "+")
	if len(parts) < 2 {
		return form.Hotkey{}
	}
	var h form.Hotkey
	for _, word := range parts[:len(parts)-1] {
		h.Mods |= modifier(word)
	}
	h.Key = key(parts[len(parts)-1])
	if h.Mods == 0 || h.Key == ui.KeyUnknown {
		return form.Hotkey{}
	}
	return h
}

// acceleratorKeys 把按键在界面上的写法换成 accelerator 认识的名字。
var acceleratorKeys = map[string]string{
	"Esc": "Escape",
	"←":   "Left",
	"→":   "Right",
	"↑":   "Up",
	"↓":   "Down",
}

// modifier 读出一个修饰词，与 accelerator 的词表一致。
func modifier(word string) ui.Modifiers {
	switch strings.ToLower(word) {
	case "command", "cmd", "super", "meta", "win":
		return ui.Super
	case "control", "ctrl":
		return ui.Ctrl
	case "cmdorctrl":
		if runtime.GOOS == "darwin" {
			return ui.Super
		}
		return ui.Ctrl
	case "alt", "option":
		return ui.Alt
	case "shift":
		return ui.Shift
	}
	return 0
}

// key 读出一个按键名，与 accelerator 的词表一致。
func key(name string) ui.Key {
	if len(name) == 1 {
		c := name[0]
		switch {
		case c >= 'a' && c <= 'z':
			return ui.KeyA + ui.Key(c-'a')
		case c >= 'A' && c <= 'Z':
			return ui.KeyA + ui.Key(c-'A')
		case c >= '0' && c <= '9':
			return ui.Key0 + ui.Key(c-'0')
		}
	}
	lower := strings.ToLower(name)
	if len(lower) > 1 && lower[0] == 'f' {
		if n, err := strconv.Atoi(lower[1:]); err == nil && n >= 1 && n <= 12 {
			return ui.KeyF1 + ui.Key(n-1)
		}
	}
	return keyCodes[lower]
}

// keyCodes 是命名按键与 accelerator 词表的对照。
var keyCodes = map[string]ui.Key{
	"escape": ui.KeyEscape, "esc": ui.KeyEscape,
	"enter": ui.KeyEnter, "return": ui.KeyEnter,
	"space": ui.KeySpace, "tab": ui.KeyTab,
	"backspace": ui.KeyBackspace,
	"delete":    ui.KeyDelete, "del": ui.KeyDelete,
	"insert": ui.KeyInsert,
	"home":   ui.KeyHome, "end": ui.KeyEnd,
	"pageup": ui.KeyPageUp, "pagedown": ui.KeyPageDown,
	"left": ui.KeyLeft, "right": ui.KeyRight, "up": ui.KeyUp, "down": ui.KeyDown,
	"-": ui.KeyMinus, "=": ui.KeyEqual, ",": ui.KeyComma, ".": ui.KeyPeriod,
	"/": ui.KeySlash, ";": ui.KeySemicolon, "'": ui.KeyQuote,
	"[": ui.KeyBracketLeft, "]": ui.KeyBracketRight, "\\": ui.KeyBackslash,
	"`": ui.KeyBackquote,
}
