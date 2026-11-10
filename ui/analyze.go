package ui

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lumemole/app"
	"lumemole/engine"
)

// AnalyzePage renders the disk-usage explorer, replicating Mole's 分析 tab:
// breadcrumb + left list + right treemap.
func AnalyzePage(c *ui.Context, a *app.App) {
	t := c.Theme()
	st := analyzeState(a)

	if st.Provider == nil || !st.Provider.Available() {
		placeholder(c, "⚠️", "未找到 Mole CLI",
			"磁盘分析依赖 mole 的 analyze-go 组件，请先通过 Homebrew 安装 mole:\n  brew install mole")
		return
	}

	// First visit: scan overview.
	if st.Result == nil && !st.Loading && st.Err == "" {
		st.ScanTo("", func() {
			if a.Win != nil {
				a.Win.Invalidate()
			}
		})
	}

	if st.Loading && st.Result == nil {
		ui.Column(c).Fill().Center().Gap(14).Children(func() {
			ui.Spinner(c).Size(28, 28)
			label := "正在扫描磁盘…"
			if st.ScanPath != "" {
				label = "正在扫描 " + st.ScanPath + "…"
			}
			ui.Text(c, label).TextColor(t.TextMuted)
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
				st.ScanTo(st.ScanPath, func() { a.Win.Invalidate() })
			}
		})
		return
	}

	ui.Column(c).Fill().Padding(16, 20).Gap(12).Children(func() {
		breadcrumb(c, a, st, *t)
		ui.Row(c).Fill().Gap(14).Children(func() {
			entryList(c, a, st, *t)
			treemapView(c, a, st, *t)
		})
	})
}

func analyzeState(a *app.App) *engine.AnalyzeState {
	if a.Analyze == nil {
		a.Analyze = engine.NewAnalyzeState()
	}
	return a.Analyze
}

// breadcrumb draws the path navigation row.
func breadcrumb(c *ui.Context, a *app.App, st *engine.AnalyzeState, t ui.Theme) {
	ui.Row(c).FillWidth().Gap(4).AlignItems(ui.Center).Children(func() {
		if len(st.Crumbs) <= 1 {
			ui.Text(c, "整个磁盘").FontSize(13).TextColor(t.TextMuted)
		} else {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				for i, p := range st.Crumbs {
					idx := i
					last := i == len(st.Crumbs)-1
					label := engine.CrumbLabel(p)
					if last {
						ui.Text(c, label).FontSize(13).Bold()
					} else {
						ui.Row(c).AlignItems(ui.Center).Children(func() {
							ui.Text(c, label).FontSize(13).TextColor(t.Accent)
						}).OnClick(func() {
							st.JumpToCrumb(idx, func() { a.Win.Invalidate() })
						})
						ui.Text(c, "›").FontSize(13).TextColor(t.TextMuted)
					}
				}
			})
			ui.Spacer(c)
		}

		// Right side: summary + rescan.
		if st.Result != nil {
			ui.Textf(c, "%d 项 · %s", len(st.Result.Entries), engine.FmtBytes(st.Result.TotalSize)).
				FontSize(12).TextColor(t.TextMuted)
		}
		if ui.Button(c, "↻").Width(30).Clicked() {
			cur := st.ScanPath
			delete(st.Cache, cur)
			st.ScanTo(cur, func() { a.Win.Invalidate() })
		}
	})
}

// entryList is the left column: scrollable sorted entries.
func entryList(c *ui.Context, a *app.App, st *engine.AnalyzeState, t ui.Theme) {
	n := 0
	if st.Result != nil {
		n = len(st.Result.Entries)
	}
	ui.Column(c).Width(320).Fill().Background(t.Surface).Radius(12).Children(func() {
		ui.List(c, analyzeListState, n, func(row int) {
			if st.Result == nil || row >= len(st.Result.Entries) {
				return
			}
			e := st.Result.Entries[row]
			path := e.Path
			isDir := e.IsDir
			icon := "📄"
			if isDir {
				icon = "📁"
			}
			rowEl := ui.Row(c).FillWidth().Gap(8).Padding(8, 12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, icon).FontSize(14)
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, e.Name).FontSize(13).SingleLine()
					ui.Row(c).Gap(6).Children(func() {
						ui.Text(c, engine.FmtBytes(e.Size)).FontSize(11).TextColor(t.TextMuted)
						if e.ScanStatus == "partial" {
							ui.Text(c, "部分").FontSize(10).TextColor(ui.Hex("#e0c53c"))
						}
						if e.Cleanable {
							ui.Text(c, "可清理").FontSize(10).TextColor(t.Accent)
						}
					})
				})
				if isDir {
					ui.Text(c, "›").FontSize(16).TextColor(t.TextMuted)
				}
			})
			if isDir {
				rowEl.OnClick(func() {
					st.ScanTo(path, func() { a.Win.Invalidate() })
				})
			}
		}).Grow(1)
	})
}

var analyzeListState = &ui.ListState{}

// treemapView renders the squarified treemap on the right.
func treemapView(c *ui.Context, a *app.App, st *engine.AnalyzeState, t ui.Theme) {
	ui.Box(c).Grow(1).Fill().Background(t.Surface).Radius(12).Children(func() {
		if st.Result == nil {
			return
		}
		nodes := engine.BuildTree(st.Result)
		if len(nodes) == 0 {
			ui.Column(c).Fill().Center().Children(func() {
				ui.Text(c, "无数据").TextColor(t.TextMuted)
			})
			return
		}
		rects := engine.Layout(nodes)
		palette := treemapPalette()
		ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
			for i, rc := range rects {
				col := palette[i%len(palette)]
				x := r.X + float32(rc.X)*r.W
				y := r.Y + float32(rc.Y)*r.H
				w := float32(rc.W) * r.W
				h := float32(rc.H) * r.H
				rect := ui.Rect{X: x + 1, Y: y + 1, W: w - 2, H: h - 2}
				p.Fill(rect, col, 6)

				if w > 60 && h > 36 {
					name := rc.Node.Name
					size := engine.FmtBytes(rc.Node.Size)
					p.Text(rect.X+8, rect.Y+8, name, 12, ui.Hex("#f5f2fa"))
					p.Text(rect.X+8, rect.Y+24, size, 10, ui.Hex("#c8c0d8"))
				}
			}
		})
	})
}

// treemapPalette: warm→cool gradient of accent-adjacent colors.
func treemapPalette() []ui.Color {
	return []ui.Color{
		ui.Hex("#d97a4a"), // terracotta
		ui.Hex("#c98a4a"), // amber
		ui.Hex("#b89a4a"), // gold
		ui.Hex("#a0aa50"), // olive
		ui.Hex("#7ab868"), // sage
		ui.Hex("#5ac4a0"), // mint
		ui.Hex("#4aa8c0"), // teal
		ui.Hex("#5088c0"), // steel
		ui.Hex("#6878b8"), // periwinkle
		ui.Hex("#8868a8"), // lavender
		ui.Hex("#a06090"), // mauve
		ui.Hex("#b06070"), // rose
	}
}

var _ = mygo.Shell
