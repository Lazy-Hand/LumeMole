package ui

import (
	"github.com/egoist/mygo/ui"

	"lumemole/app"
	"lumemole/engine"
)

// AIPage is the AI-tools cleanup & maintenance view, reached from the
// clean page's entry card. Bands: safe / review / tool — safe items
// pre-selected, review items opt-in.
func AIPage(c *ui.Context, a *app.App) {
	t := c.Theme()
	st := aiState(a)

	if st.Report == nil && !st.Scanning && st.Err == "" {
		aiLanding(c, a, st, *t)
		return
	}
	if st.Scanning {
		ui.Column(c).Fill().Center().Gap(16).Children(func() {
			ui.Spinner(c).Size(28, 28)
			ui.Text(c, "正在扫描 AI 工具数据…").FontSize(14).TextColor(t.TextMuted)
		})
		return
	}
	if st.Done {
		aiDone(c, a, st, *t)
		return
	}
	aiReport(c, a, st, *t)
}

func aiState(a *app.App) *engine.AIState {
	if a.AI == nil {
		a.AI = &engine.AIState{}
	}
	return a.AI
}

// aiLanding mirrors Mole's landing: title + tool chips + scan button.
func aiLanding(c *ui.Context, a *app.App, st *engine.AIState, t ui.Theme) {
	ui.Column(c).Fill().Center().Gap(20).Padding(40).Children(func() {
		ui.Text(c, "🌙").FontSize(72)
		ui.Text(c, "AI 工具清理与维护").FontSize(24).Bold()
		ui.Text(c, "扫描 Claude Code、Codex、Cursor、Gemini 的缓存、聊天记录与旧版本").FontSize(13).TextColor(t.TextMuted)

		if ui.PrimaryButton(c, "扫描 AI 数据").Width(160).Height(38).Clicked() {
			st.Scan(func() { a.Win.Invalidate() })
		}

		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "🔒").FontSize(12)
			ui.Text(c, "聊天记录逐项确认后才会清理").FontSize(12).TextColor(t.TextMuted)
		})

		if ui.Button(c, "← 返回清理").Clicked() {
			a.ShowAI = false
		}
	})
}

// aiReport lists items grouped by band with checkboxes.
func aiReport(c *ui.Context, a *app.App, st *engine.AIState, t ui.Theme) {
	r := st.Report

	ui.Column(c).Fill().Padding(16, 20).Gap(12).Children(func() {
		// Summary header.
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Gap(4).Children(func() {
				ui.Row(c).AlignItems(ui.Baseline).Gap(8).Children(func() {
					ui.Text(c, engine.FmtBytes(r.Total())).FontSize(32).Bold()
					ui.Textf(c, "可清理 · %d 项", countSelected(r)).FontSize(14).TextColor(t.TextMuted)
				})
			})
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				if ui.Button(c, "重新扫描").Clicked() {
					st.Report = nil
					st.Scan(func() { a.Win.Invalidate() })
				}
				if st.Cleaning {
					ui.Spinner(c).Size(20, 20)
				} else {
					ui.PrimaryButton(c, "清理所选").Height(34).OnClick(func() {
						st.Clean(func() { a.Win.Invalidate() })
					})
				}
			})
		})

		ui.Divider(c).BorderColor(t.Border)

		if len(r.Items) == 0 {
			ui.Column(c).Fill().Center().Gap(12).Children(func() {
				ui.Text(c, "✨").FontSize(48)
				ui.Text(c, "当前没有符合清理条件的项目").FontSize(15).TextColor(t.TextMuted)
			})
			return
		}

		ui.List(c, aiListState, len(r.Items), func(row int) {
			it := &r.Items[row]
			// Band header above the first item of each band.
			if row == 0 || r.Items[row-1].Band != it.Band {
				bandHeader(c, it.Band, t)
			}
			ui.Row(c).FillWidth().Gap(10).Padding(9, 14).AlignItems(ui.Center).Children(func() {
				ui.Checkbox(c, &it.Selected, "")
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, it.Title).FontSize(13).Bold()
						ui.Text(c, it.Tool).FontSize(10).Padding(2, 6).
							Background(t.Surface).Radius(8).TextColor(t.TextMuted)
					})
					ui.Text(c, it.Detail).FontSize(11).TextColor(t.TextMuted)
				})
				ui.Text(c, engine.FmtBytes(it.Size)).FontSize(12)
			})
		}).Grow(1)

		if st.Err != "" {
			ui.Text(c, st.Err).FontSize(11).TextColor(ui.Hex("#e08c3c"))
		}
	})
}

var aiListState = &ui.ListState{}

func bandHeader(c *ui.Context, band engine.AIBand, t ui.Theme) {
	var title, note string
	switch band {
	case engine.BandSafe:
		title, note = "缓存与旧版本", "缓存可重新生成，已默认勾选"
	case engine.BandReview:
		title, note = "对话与快照", "逐项确认后再清理，可能是唯一副本"
	case engine.BandTool:
		title, note = "闲置工具与维护", "卸载闲置工具、清理残留或压缩数据库"
	}
	ui.Row(c).FillWidth().Padding(10, 14, 4, 14).Gap(10).AlignItems(ui.Baseline).Children(func() {
		ui.Text(c, title).FontSize(14).Bold()
		ui.Text(c, note).FontSize(11).TextColor(t.TextMuted)
	})
}

func countSelected(r *engine.AIReport) int {
	n := 0
	for _, it := range r.Items {
		if it.Selected {
			n++
		}
	}
	return n
}

// aiDone is the result overlay after cleaning.
func aiDone(c *ui.Context, a *app.App, st *engine.AIState, t ui.Theme) {
	ui.Column(c).Fill().Center().Gap(16).Children(func() {
		ui.Text(c, "✅").FontSize(60)
		ui.Textf(c, "已清理 %s", engine.FmtBytes(st.Freed)).FontSize(22).Bold()
		if st.Err != "" {
			ui.Text(c, st.Err).FontSize(12).TextColor(ui.Hex("#e08c3c"))
		}
		ui.Row(c).Gap(10).Children(func() {
			if ui.Button(c, "再扫一次").Clicked() {
				st.Report = nil
				st.Done = false
				st.Scan(func() { a.Win.Invalidate() })
			}
			if ui.PrimaryButton(c, "完成").Clicked() {
				st.Report = nil
				st.Done = false
				a.ShowAI = false
			}
		})
	})
}

// aiEntryCard is the entry point rendered on the clean page.
func aiEntryCard(c *ui.Context, a *app.App, t ui.Theme) {
	ui.Row(c).FillWidth().Padding(12, 16).Background(t.Surface).Radius(12).
		AlignItems(ui.Center).Gap(12).Children(func() {
		ui.Text(c, "🤖").FontSize(22)
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, "AI 工具清理").FontSize(13).Bold()
			ui.Text(c, "Claude / Codex / Cursor / Gemini 的缓存、聊天记录与旧版本").FontSize(11).TextColor(t.TextMuted)
		})
		ui.Button(c, "打开").OnClick(func() {
			a.ShowAI = true
		})
	})
}

