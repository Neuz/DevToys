package qrcode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cyenoch/mygoicons/lucide"
	"github.com/Cyenoch/mygoui"
	"github.com/Cyenoch/mygoui/color"
	"github.com/Cyenoch/mygoui/content"
	"github.com/Cyenoch/mygoui/form"
	"github.com/Cyenoch/mygoui/list"
	"github.com/Cyenoch/mygoui/theme"
	"github.com/Neuz/DevToys/internal/features/qrcode/locales"
	"github.com/Neuz/DevToys/internal/fsx"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// 尺寸只影响导出：预览固定 qrPreviewSize，默认导出 qrDefaultSize。
const (
	qrDefaultSize = 480
	qrPreviewSize = 160
	qrMinSize     = 10
	qrMaxSize     = 4096
	qrSizeStep    = 10
	// controlWidth 让尺寸、纠错级别和两个颜色选择器右对齐且同宽。
	controlWidth = 200
	// copyFeedback 是“已复制”提示停留的时长。
	copyFeedback = 1500 * time.Millisecond
)

// 纠错级别的稳定键，存进 form.SelectState 后映射到 content.QRLevel。
const (
	levelLow      = "low"
	levelMedium   = "medium"
	levelQuartile = "quartile"
	levelHigh     = "high"
)

// View
func (m *Module) View(c *ui.Context) {
	theme.Apply(c)
	ui.Column(c).Fill().Padding(theme.Space(2), theme.Space(8)).Gap(theme.Space(5)).Children(func() {
		ui.Column(c).Gap(theme.Space(1)).Children(func() {
			mygoui.Heading(c, int(theme.TypeH1), locales.T("title"))
			mygoui.Typography(c, locales.T("subtitle"), mygoui.TypographyOptions{
				Type: theme.TypeBody,
			})
		})
		ui.Row(c).Gap(theme.Space(4)).AlignItems(ui.Center).Children(func() {
			// 预览固定 160px，导出时才使用“尺寸”里的值。
			_, _ = content.QRCode(c, content.QRCodeOptions{
				Value:      m.qrValue,
				Size:       qrPreviewSize,
				Level:      m.levelValue(),
				Foreground: m.foreground.Value,
				Background: m.background.Value,
			})
			ui.Column(c).Gap(theme.Space(2)).Children(func() {
				mygoui.Button(c, mygoui.ButtonOptions{
					Label:   locales.T("save"),
					Icon:    lucide.Save(),
					Variant: mygoui.ButtonOutline,
					OnClick: m.saveImage,
				})
				label, icon := locales.T("copy"), lucide.Copy()
				if m.copied {
					label, icon = locales.T("copied"), lucide.Check()
				}
				mygoui.Button(c, mygoui.ButtonOptions{
					Label:   label,
					Icon:    icon,
					Variant: mygoui.ButtonOutline,
					OnClick: m.copyImage,
				})
			})
		})
		mygoui.SettingsGroup(c, mygoui.SettingsGroupOptions{
			Rows: []mygoui.SettingsRow{
				{
					Icon:  lucide.LetterText(),
					Title: locales.T("content"),
					Trailing: func() {
						form.Input(c, &m.qrValue, form.InputOptions{
							AriaLabel: locales.T("content"),
						}).Width(theme.Scale(c, 350))
					},
				},
				{
					Icon:     lucide.Maximize(),
					Title:    locales.T("size"),
					Subtitle: locales.T("size_hint"),
					Trailing: func() {
						min, max := float64(qrMinSize), float64(qrMaxSize)
						form.NumberField(c, &m.size, form.NumberFieldOptions{
							AriaLabel: locales.T("size"),
							Min:       &min,
							Max:       &max,
							Step:      qrSizeStep,
						}).Width(theme.Scale(c, controlWidth))
					},
				},
				{
					Icon:  lucide.ShieldCheck(),
					Title: locales.T("level"),
					Trailing: func() {
						ui.Row(c).Width(theme.Scale(c, controlWidth)).Children(func() {
							form.Select(c, form.SelectOptions{
								Items:     levelItems(),
								State:     &m.level,
								Flyout:    &m.flyouts,
								FullWidth: true,
							})
						})
					},
					Control: &m.level.Trigger,
				},
				{
					Icon:  lucide.Palette(),
					Title: locales.T("colors"),
					Rows: []mygoui.SettingsRow{
						{
							Icon:  lucide.Palette(),
							Title: locales.T("foreground"),
							Trailing: func() {
								ui.Row(c).Width(theme.Scale(c, controlWidth)).Children(func() {
									color.ColorPicker(c, color.ColorPickerOptions{
										Label:     locales.T("foreground"),
										State:     m.foreground,
										Open:      &m.fgOpen,
										Flyout:    &m.flyouts,
										HideAlpha: true,
									})
								})
							},
						},
						{
							Icon:  lucide.Palette(),
							Title: locales.T("background"),
							Trailing: func() {
								ui.Row(c).Width(theme.Scale(c, controlWidth)).Children(func() {
									color.ColorPicker(c, color.ColorPickerOptions{
										Label:     locales.T("background"),
										State:     m.background,
										Open:      &m.bgOpen,
										Flyout:    &m.flyouts,
										HideAlpha: true,
									})
								})
							},
						},
					},
				},
			},
		})
	})
}

// levelItems 是纠错级别的四个选项，键与 content.QRLevel 的映射见 levelValue。
func levelItems() []list.ListBoxItem {
	return []list.ListBoxItem{
		{Key: levelLow, Label: locales.T("level_low")},
		{Key: levelMedium, Label: locales.T("level_medium")},
		{Key: levelQuartile, Label: locales.T("level_quartile")},
		{Key: levelHigh, Label: locales.T("level_high")},
	}
}

func (m *Module) levelValue() content.QRLevel {
	switch m.level.List.SelectedKey {
	case levelLow:
		return content.QRLow
	case levelQuartile:
		return content.QRQuartile
	case levelHigh:
		return content.QRHigh
	default:
		return content.QRMedium
	}
}

// saveImage 弹系统保存框，把按“尺寸”等参数生成的 PNG 写到用户选择的位置。
func (m *Module) saveImage() {
	value, size := m.qrValue, int(m.size.Value)
	level, fg, bg := m.levelValue(), m.foreground.Value, m.background.Value
	parent := m.flyouts.Window
	dialogTitle, filterName := locales.T("save_dialog"), locales.T("png_filter")
	failTitle := locales.T("save_failed")

	go func() {
		name := fmt.Sprintf("qrcode_%d.png", size)
		defaultPath := name
		if home, err := os.UserHomeDir(); err == nil {
			defaultPath = filepath.Join(home, name)
		}
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:      parent,
			Title:       dialogTitle,
			DefaultPath: defaultPath,
			Filters:     []mygo.FileFilter{{Name: filterName, Extensions: []string{"png"}}},
		})
		if err != nil {
			m.showError(failTitle, err)
			return
		}
		if path == "" {
			return
		}
		if !strings.EqualFold(filepath.Ext(path), ".png") {
			path += ".png"
		}
		data, err := encodePNG(value, level, size, fg, bg)
		if err == nil {
			err = fsx.WriteFile(path, data, 0o644)
		}
		if err != nil {
			m.showError(failTitle, err)
		}
	}()
}

// copyImage 把按当前参数生成的 PNG 放进剪贴板。
func (m *Module) copyImage() {
	value, size := m.qrValue, int(m.size.Value)
	level, fg, bg := m.levelValue(), m.foreground.Value, m.background.Value
	failTitle := locales.T("copy_failed")

	go func() {
		data, err := encodePNG(value, level, size, fg, bg)
		if err == nil {
			err = mygo.Clipboard.WriteImage(data)
		}
		if err != nil {
			m.showError(failTitle, err)
			return
		}
		mygo.RunOnMain(m.markCopied)
	}()
}

// markCopied 让复制按钮短暂显示“已复制”，随后恢复。
func (m *Module) markCopied() {
	m.copySeq++
	seq := m.copySeq
	m.copied = true
	if m.copyTimer != nil {
		m.copyTimer.Stop()
	}
	m.copyTimer = time.AfterFunc(copyFeedback, func() {
		mygo.RunOnMain(func() {
			if m.copySeq != seq {
				return
			}
			m.copied = false
			m.invalidate()
		})
	})
	m.invalidate()
}

func (m *Module) invalidate() {
	if w := m.flyouts.Window; w != nil {
		w.Invalidate()
	}
}

// showError 把导出失败的原因告诉用户，同时记一条 warn。
func (m *Module) showError(title string, err error) {
	m.log.Warn("qrcode: exporting image", "error", err)
	mygo.Dialog.Error(title, err.Error())
}
