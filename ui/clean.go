package ui

import (
	"os"
	"os/exec"

	"github.com/egoist/mygo/ui"

	"lumemole/app"
	"lumemole/engine"
)

// CleanPage renders the disk-cleanup screen.
// Phase 3a: dry-run preview powered by `mo clean --dry-run`, with section
// grouping, per-entry checkboxes, and a read-only review. Actual deletion is
// delegated to the terminal (`mo clean`) until the safety chain is proven.
func CleanPage(c *ui.Context, a *app.App) {
	t := c.Theme()
	st := cleanState(a)

	if st.Provider == nil || !st.Provider.Available() {
		placeholder(c, "⚠️", "未找到 Mole CLI",
			"清理功能依赖 mole CLI，请先通过 Homebrew 安装 mole:\n  brew install mole")
		return
	}

	// First visit: show a friendly start screen, not an auto-run.
	if st.Report == nil && !st.Running && st.Err == "" {
		cleanLanding(c, a, st, *t)
		return
	}

	if st.Running {
		ui.Column(c).Fill().Center().Gap(16).Children(func() {
			ui.Spinner(c).Size(28, 28)
			ui.Text(c, st.Progress).FontSize(14).TextColor(t.TextMuted)
			ui.Text(c, "扫描通常需要 30–60 秒").FontSize(11).TextColor(t.TextMuted)
		})
		return
	}

	if st.Err != "" {
		ui.Column(c).Fill().Center().Gap(14).Children(func() {
			ui.Text(c, "⚠️").FontSize(40)
			ui.Text(c, "扫描失败").FontSize(18).Bold()
			ui.Text(c, st.Err).FontSize(12).TextColor(t.TextMuted)
			if ui.Button(c, "重试").Clicked() {
				st.Err = ""
				st.Run(func() { a.Win.Invalidate() }, nil)
			}
		})
		return
	}

	// Report view.
	cleanReport(c, a, st, *t)
}

func cleanState(a *app.App) *engine.CleanState {
	if a.Clean == nil {
		a.Clean = engine.NewCleanState()
	}
	return a.Clean
}

// cleanLanding is the initial "hero" screen before a scan runs.
func cleanLanding(c *ui.Context, a *app.App, st *engine.CleanState, t ui.Theme) {
	ui.Column(c).Fill().Center().Gap(20).Padding(40).Children(func() {
		ui.Text(c, "🌍").FontSize(72)
		ui.Text(c, "磁盘清理").FontSize(24).Bold()
		ui.Text(c, "扫描系统、应用和浏览器的缓存、日志与残留文件").FontSize(13).TextColor(t.TextMuted)

		if ui.PrimaryButton(c, "开始扫描").Width(160).Height(38).Clicked() {
			st.Run(func() { a.Win.Invalidate() }, nil)
		}

		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "🔒").FontSize(12)
			ui.Text(c, "预览模式：只扫描不删除，全部确认后才执行").FontSize(12).TextColor(t.TextMuted)
		})
	})
}

// cleanReport shows the parsed dry-run result.
func cleanReport(c *ui.Context, a *app.App, st *engine.CleanState, t ui.Theme) {
	r := st.Report

	ui.Column(c).Fill().Padding(16, 20).Gap(12).Children(func() {
		// Summary header.
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Gap(4).Children(func() {
				ui.Row(c).AlignItems(ui.Baseline).Gap(8).Children(func() {
					ui.Text(c, engine.FmtBytes(r.TotalSize)).FontSize(32).Bold()
					ui.Textf(c, "可清理 · %d 项", r.TotalItems).FontSize(14).TextColor(t.TextMuted)
				})
				if r.Partial {
					ui.Text(c, "部分项目大小未知").FontSize(11).TextColor(ui.Hex("#e0c53c"))
				}
			})
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				if ui.Button(c, "重新扫描").Clicked() {
					st.Report = nil
					st.Run(func() { a.Win.Invalidate() }, nil)
				}
				ui.PrimaryButton(c, "在终端执行清理").Height(34).OnClick(func() {
					openMoleInTerminal()
				})
			})
		})

		ui.Divider(c).BorderColor(t.Border)

		// Section list + entries.
		ui.Row(c).Fill().Gap(14).Children(func() {
			sectionList(c, a, st, t)
			entryPane(c, a, st, t)
		})
	})
}

// sectionList is the left column of cleanable categories.
func sectionList(c *ui.Context, a *app.App, st *engine.CleanState, t ui.Theme) {
	r := st.Report
	ui.Column(c).Width(280).Fill().Background(t.Surface).Radius(12).Children(func() {
		ui.List(c, cleanSectionState, len(r.Sections), func(row int) {
			sec := &r.Sections[row]
			selected := st.SelectedSection == row
			el := ui.Row(c).FillWidth().Gap(10).Padding(10, 12).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, sec.Name).FontSize(13).Bold()
					ui.Textf(c, "%s · %d 项", engine.FmtBytes(sec.TotalSize), len(sec.Entries)).
						FontSize(11).TextColor(t.TextMuted)
				})
			})
			if selected {
				el.Background(t.SurfaceHover)
			}
			el.OnClick(func() {
				st.SelectedSection = row
			})
		}).Grow(1)
	})
}

var cleanSectionState = &ui.ListState{}
var cleanEntryState = &ui.ListState{}

// entryPane shows the right-hand entries for the selected section.
func entryPane(c *ui.Context, a *app.App, st *engine.CleanState, t ui.Theme) {
	r := st.Report
	if st.SelectedSection >= len(r.Sections) {
		st.SelectedSection = 0
	}
	sec := &r.Sections[st.SelectedSection]

	ui.Column(c).Grow(1).Fill().Background(t.Surface).Radius(12).Children(func() {
		// Header with select-all.
		ui.Row(c).FillWidth().Padding(10, 14).AlignItems(ui.Center).Children(func() {
			ui.Textf(c, "%s · %d 项", sec.Name, len(sec.Entries)).FontSize(14).Bold().Grow(1)
			ui.Row(c).Gap(8).Children(func() {
				if ui.Button(c, "全选").Width(70).Clicked() {
					r.SelectSection(sec.Name, true)
				}
				if ui.Button(c, "全不选").Width(80).Clicked() {
					r.SelectSection(sec.Name, false)
				}
			})
		})
		ui.Divider(c).BorderColor(t.Border)

		ui.List(c, cleanEntryState, len(sec.Entries), func(row int) {
			e := &sec.Entries[row]
			ui.Row(c).FillWidth().Gap(10).Padding(8, 14).AlignItems(ui.Center).Children(func() {
				ui.Checkbox(c, &e.Selected, "")
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, shortenPath(e.Path)).FontSize(12).SingleLine()
					ui.Row(c).Gap(6).Children(func() {
						size := e.SizeStr
						if size == "" && e.Size >= 0 {
							size = engine.FmtBytes(e.Size)
						}
						if e.Size < 0 {
							size = "大小未知"
						}
						ui.Text(c, size).FontSize(10).TextColor(t.TextMuted)
						if e.Items > 0 {
							ui.Textf(c, "%d 项", e.Items).FontSize(10).TextColor(t.TextMuted)
						}
						if e.Note != "" {
							ui.Text(c, e.Note).FontSize(10).TextColor(t.TextMuted)
						}
					})
				})
			})
		}).Grow(1)
	})
}

// shortenPath compresses the home directory for display.
func shortenPath(p string) string {
	home, _ := os.UserHomeDir()
	if home != "" && len(p) > len(home) && p[:len(home)] == home {
		return "~" + p[len(home):]
	}
	return p
}

// openMoleInTerminal opens Terminal.app running `mo clean`.
func openMoleInTerminal() {
	script := `tell application "Terminal"
	activate
	do script "mo clean"
end tell`
	_ = exec.Command("osascript", "-e", script).Run()
}
