package ui

import (
	"github.com/egoist/mygo/ui"

	"lumemole/app"
	"lumemole/engine"
)

// OptimizePage renders system-maintenance tasks, replicating Mole's 优化 tab.
func OptimizePage(c *ui.Context, a *app.App) {
	t := c.Theme()
	st := optimizeState(a)

	ui.Column(c).Fill().Padding(20, 24).Gap(16).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Gap(4).Children(func() {
				ui.Text(c, "系统维护").FontSize(22).Bold()
				ui.Text(c, "刷新缓存、修复配置、清理系统数据库").FontSize(13).TextColor(t.TextMuted)
			})
		})

		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).FillWidth().Gap(10).Children(func() {
				for i := range st.Tasks {
					task := &st.Tasks[i]
					taskCard(c, a, st, task, *t)
				}
			})
		})
	})
}

func optimizeState(a *app.App) *engine.OptimizeState {
	if a.Optimize == nil {
		a.Optimize = engine.NewOptimizeState()
	}
	return a.Optimize
}

func taskCard(c *ui.Context, a *app.App, st *engine.OptimizeState, task *engine.OptimizeTask, t ui.Theme) {
	running := st.Running[task.ID]
	result := st.Results[task.ID]

	ui.Row(c).FillWidth().Gap(14).Padding(14).Background(t.Surface).Radius(10).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Gap(4).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, task.Name).FontSize(14).Bold()
				if task.NeedsSudo {
					ui.Text(c, "需管理员").FontSize(10).Padding(2, 8).Background(ui.RGBA(224, 197, 60, 0.15)).Radius(8).TextColor(ui.Hex("#e0c53c"))
				}
				if task.Status != nil {
					if s := task.Status(); s != "" {
						ui.Text(c, s).FontSize(10).TextColor(t.Accent)
					}
				}
			})
			ui.Text(c, task.Description).FontSize(12).TextColor(t.TextMuted)
			if result != "" {
				col := t.Accent
				if len(result) > 3 && result[:3] == "失败" {
					col = ui.Hex("#e06c3c")
				}
				ui.Text(c, result).FontSize(11).TextColor(col)
			}
		})
		ui.Row(c).Gap(8).Children(func() {
			if running {
				ui.Spinner(c).Size(16, 16)
			} else {
				ui.Button(c, "运行").Width(70).OnClick(func() {
					st.RunTask(task.ID, func() { a.Win.Invalidate() })
				})
			}
		})
	})
}
