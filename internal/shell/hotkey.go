package shell

import (
	"github.com/Neuz/DevToys/internal/contract"
	"github.com/egoist/mygo"
)

// rebindHotkeys 注销并重新注册外壳与所有启用模块的全局快捷键。
// 快捷键或功能的启用状态变化后都要调用它。
func (s *Shell) rebindHotkeys() {
	s.unregisterHotkeys()
	s.hotkeyGlobal = s.registerHotkey("shell.window_toggle", s.opts.Config.Hotkey.Global.WindowToggle, s.toggleMain)
	for _, m := range s.modules {
		hk, ok := m.(contract.GlobalHotkeyer)
		if !ok || !moduleEnabled(m) {
			continue
		}
		for _, h := range hk.Hotkeys() {
			// 配置里没有这个键才算「未自定义」，回退默认；键存在但值为空
			// 表示用户显式删除，此时不注册任何快捷键。
			acc, ok := s.opts.Config.FeatureNode(m.ID()).Hotkeys[h.ID]
			if !ok {
				acc = h.Default
			}
			s.registerHotkey(m.ID()+"."+h.ID, acc, h.Run)
		}
	}
	s.refreshTrayMenu()
}

// registerHotkey 注册一个全局快捷键，返回实际生效的 accelerator。
// 重复或注册失败时返回空串，只记一条 warn，不影响其他绑定。
func (s *Shell) registerHotkey(id, acc string, run func()) string {
	if acc == "" {
		return ""
	}
	for _, got := range s.hotkeys {
		if got == acc {
			s.log.Warn("shell: 全局快捷键重复", "id", id, "hotkey", acc)
			return ""
		}
	}
	if err := mygo.GlobalShortcut.Register(acc, run); err != nil {
		s.log.Warn("shell: 注册全局快捷键", "id", id, "hotkey", acc, "error", err)
		return ""
	}
	s.hotkeys = append(s.hotkeys, acc)
	return acc
}

// unregisterHotkeys 注销全部全局快捷键；没有注册时什么也不做。
func (s *Shell) unregisterHotkeys() {
	for _, acc := range s.hotkeys {
		mygo.GlobalShortcut.Unregister(acc)
	}
	s.hotkeys = nil
}

// moduleEnabled 报告模块是否启用；没有实现 contract.Toggleable 的模块始终启用。
func moduleEnabled(m contract.Module) bool {
	t, ok := m.(contract.Toggleable)
	return !ok || t.Enabled()
}

// toggleMain 由全局快捷键调用：显示并聚焦主窗口，或把它收起来。
func (s *Shell) toggleMain() {
	mygo.RunOnMain(func() {
		if s.win == nil {
			return
		}
		if s.win.IsVisible() {
			s.win.Hide()
			mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
			return
		}
		s.showMain(s.current)
	})
}
