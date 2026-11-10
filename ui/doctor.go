package ui

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lumemole/app"
	"lumemole/engine"
)

// DoctorPage is the 诊断 tab: issues worth attention plus grouped
// system info, with a copy-report button for support requests.
func DoctorPage(c *ui.Context, a *app.App) {
	t := c.Theme()
	st := doctorState(a)

	if st.Report == nil && !st.Running {
		st.Run(func() { a.Win.Invalidate() })
	}
	if st.Running && st.Report == nil {
		ui.Column(c).Fill().Center().Gap(14).Children(func() {
			ui.Spinner(c).Size(28, 28)
			ui.Text(c, "正在诊断…").TextColor(t.TextMuted)
		})
		return
	}
	if st.Report == nil {
		placeholder(c, "🩺", "系统诊断", "无法生成报告")
		return
	}

	r := st.Report
	ui.Column(c).Fill().Padding(16, 20).Gap(12).Children(func() {
		// Header.
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Gap(4).Children(func() {
				ui.Text(c, "系统诊断").FontSize(22).Bold()
				ui.Text(c, "查看 Mac 与 LumeMole 的运行状态，反馈问题时可附上这份报告").
					FontSize(12).TextColor(t.TextMuted)
			})
			ui.Row(c).Gap(10).Children(func() {
				if ui.Button(c, "复制报告").Clicked() {
					mygo.Clipboard.WriteText(r.Text())
					c.Toast("已复制到剪贴板")
				}
				if ui.Button(c, "重新扫描").Clicked() {
					st.Run(func() { a.Win.Invalidate() })
				}
			})
		})

		ui.Divider(c).BorderColor(t.Border)

		ui.List(c, doctorListState, doctorRowCount(r), func(row int) {
			doctorRow(c, a, r, row, *t)
		}).Grow(1)
	})
}

func doctorState(a *app.App) *engine.DoctorState {
	if a.Doctor == nil {
		fda := a.Permissions != nil && a.Permissions.FullDiskAccess
		a.Doctor = &engine.DoctorState{FDA: fda}
	}
	return a.Doctor
}

var doctorListState = &ui.ListState{}

// doctorRowCount = issue rows + (section header + kv rows) per section.
func doctorRowCount(r *engine.DoctorReport) int {
	n := 0
	if len(r.Issues) > 0 {
		n += 1 + len(r.Issues) // "待处理项" header + issues
	}
	for _, s := range r.Sections {
		n += 1 + len(s.Rows)
	}
	return n
}

// doctorRow renders row i across the issues block and the sections.
func doctorRow(c *ui.Context, a *app.App, r *engine.DoctorReport, i int, t ui.Theme) {
	idx := 0
	if len(r.Issues) > 0 {
		if i == 0 {
			doctorSectionHeader(c, "待处理项", t)
			return
		}
		i--
		if i < len(r.Issues) {
			doctorIssueRow(c, a, r.Issues[i], t)
			return
		}
		i -= len(r.Issues)
		_ = idx
	}
	for _, s := range r.Sections {
		if i == 0 {
			doctorSectionHeader(c, s.Title, t)
			return
		}
		i--
		if i < len(s.Rows) {
			kv := s.Rows[i]
			ui.Row(c).FillWidth().Padding(7, 14).Children(func() {
				ui.Text(c, kv.Key).FontSize(12).Width(120).TextColor(t.TextMuted)
				ui.Text(c, kv.Value).FontSize(12).Grow(1)
			})
			return
		}
		i -= len(s.Rows)
	}
}

func doctorSectionHeader(c *ui.Context, title string, t ui.Theme) {
	ui.Row(c).FillWidth().Padding(14, 14, 4, 14).Children(func() {
		ui.Text(c, title).FontSize(14).Bold()
	})
}

func doctorIssueRow(c *ui.Context, a *app.App, is engine.DoctorIssue, t ui.Theme) {
	icon := "ℹ️"
	color := ui.Hex("#6aa0f5")
	if is.Severity == "warn" {
		icon = "⚠️"
		color = ui.Hex("#e08c3c")
	}
	ui.Row(c).FillWidth().Padding(8, 14).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, icon).FontSize(14)
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, is.Title).FontSize(13).Bold().TextColor(color)
			if is.Detail != "" {
				ui.Text(c, is.Detail).FontSize(11).TextColor(t.TextMuted)
			}
		})
		if is.Action != "" {
			ui.Button(c, "前往"+is.Action).OnClick(func() {
				a.Tab = actionTab(is.Action)
			})
		}
	})
}

func actionTab(name string) app.Tab {
	switch name {
	case "清理页":
		return app.TabClean
	case "软件页":
		return app.TabSoftware
	case "优化页":
		return app.TabOptimize
	case "分析页":
		return app.TabAnalyze
	case "状态页":
		return app.TabStatus
	}
	return app.TabClean
}
