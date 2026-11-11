package ui

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"lumemole/app"
	"lumemole/engine"
)

// MenuPanel renders the tray popover: a compact version of the status
// page replicating Mole's menu-bar HUD — health hero + 4×2 metric grid +
// top processes + bottom toolbar.
func MenuPanel(c *ui.Context, a *app.App) {
	t := c.Theme()
	st := statusState(a)

	if !st.Available {
		ui.Column(c).Fill().Center().Gap(12).Padding(30).Children(func() {
			ui.Text(c, "⚠️").FontSize(32)
			ui.Text(c, "未找到 Mole CLI").FontSize(14).Bold()
			ui.Text(c, "状态监控依赖 status-go").FontSize(11).TextColor(t.TextMuted)
		})
		return
	}

	st.StartWatch(func() {
		if a.Win != nil {
			a.Win.Invalidate()
		}
		if a.PanelWin != nil {
			a.PanelWin.Invalidate()
		}
	})

	snap := st.Snapshot
	if snap == nil {
		ui.Column(c).Fill().Center().Gap(12).Padding(30).Children(func() {
			ui.Spinner(c).Size(24, 24)
			ui.Text(c, "正在采集…").FontSize(12).TextColor(t.TextMuted)
		})
		return
	}

	ui.Column(c).Fill().Children(func() {
		// Scrollable metric area.
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Padding(12).Gap(10).Children(func() {
				panelHeader(c, snap, *t)
				panelGrid(c, snap, st, *t)
				panelProcs(c, snap, *t)
			})
		})
		// Bottom toolbar.
		ui.Divider(c).BorderColor(t.Border)
		panelToolbar(c, a, snap, *t)
	})
}

// panelHeader: health score + hardware chips.
func panelHeader(c *ui.Context, s *engine.StatusSnapshot, t ui.Theme) {
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Textf(c, "%d", s.HealthScore).FontSize(36).Bold()
			ui.Column(c).Gap(2).Children(func() {
				ui.Text(c, engine.HealthLabel(s.HealthScore)).FontSize(13).
					TextColor(ui.Hex(engine.HealthColorHex(s.HealthScore)))
				ui.Text(c, "已运行 "+s.Uptime).FontSize(10).TextColor(t.TextMuted)
			})
		})
		// Hardware chips.
		ui.Row(c).Gap(6).Children(func() {
			chip(c, s.Hardware.CPUModel, t)
			chip(c, s.Hardware.TotalRAM, t)
			chip(c, "macOS "+s.Hardware.OSVersion, t)
		})
	})
}

func chip(c *ui.Context, text string, t ui.Theme) {
	if text == "" {
		return
	}
	ui.Box(c).Padding(4, 8).Radius(6).Background(t.SurfaceHover).Children(func() {
		ui.Text(c, text).FontSize(10).TextColor(t.TextMuted)
	})
}

// panelGrid: 4×2 metric cards.
func panelGrid(c *ui.Context, s *engine.StatusSnapshot, st *engine.StatusState, t ui.Theme) {
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			panelTile(c, t, "🧠", "CPU", tempStr(s.Thermal.CPUTemp), func() {
				ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
					ui.Textf(c, "%.0f", s.CPU.Usage).FontSize(24).Bold()
					ui.Text(c, "%").FontSize(14).TextColor(t.TextMuted)
				})
				if len(st.CPUHist) > 1 {
					panelSparkline(c, st.CPUHist, t.Accent)
				}
				ui.Textf(c, "%d 核", s.CPU.CoreCount).FontSize(10).TextColor(t.TextMuted)
			})
			g := s.PrimaryGPU()
			gUsage := 0.0
			if g != nil && g.Usage >= 0 {
				gUsage = g.Usage
			}
			panelTile(c, t, "🎮", "GPU", tempStr(s.Thermal.GPUTemp), func() {
				ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
					ui.Textf(c, "%.0f", gUsage).FontSize(24).Bold()
					ui.Text(c, "%").FontSize(14).TextColor(t.TextMuted)
				})
				ui.Text(c, "低负载").FontSize(10).TextColor(t.TextMuted)
			})
			memCol := t.Accent
			if s.Memory.UsedPercent > 85 {
				memCol = ui.Hex("#e06c3c")
			} else if s.Memory.UsedPercent > 70 {
				memCol = ui.Hex("#e0c53c")
			}
			panelTile(c, t, "💾", "内存", memPressure(s), func() {
				ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
					ui.Textf(c, "%.0f", s.Memory.UsedPercent).FontSize(24).Bold()
					ui.Text(c, "%").FontSize(14).TextColor(t.TextMuted)
				})
				if len(st.MemHist) > 1 {
					panelSparkline(c, st.MemHist, memCol)
				}
				ui.Textf(c, "%.1f GB", float64(s.Memory.Used)/1e9).FontSize(10).TextColor(t.TextMuted)
			})
			d := s.PrimaryDisk()
			panelTile(c, t, "💿", "可用空间", "", func() {
				if d == nil {
					ui.Text(c, "—").FontSize(20).TextColor(t.TextMuted)
					return
				}
				ui.Text(c, engine.FmtBytes(d.Total-d.Used)).FontSize(24).Bold()
				ui.Progress(c, d.UsedPercent/100).FillWidth()
				ui.Textf(c, "%.0f%%", d.UsedPercent).FontSize(10).TextColor(t.TextMuted)
			})
		})
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			rx, tx := s.TotalNetRate()
			panelTile(c, t, "🌐", "网络", "Wi-Fi", func() {
				ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
					ui.Text(c, engine.FmtMBs(rx)).FontSize(24).Bold()
					ui.Text(c, "↓").FontSize(12).TextColor(t.TextMuted)
				})
				if len(st.RxHist) > 1 {
					panelSparkline(c, st.RxHist, ui.Hex("#4aa3e0"))
				}
				ui.Textf(c, "↑%s", engine.FmtMBs(tx)).FontSize(10).TextColor(t.TextMuted)
			})
			panelTile(c, t, "🌀", "风扇", "", func() {
				ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
					ui.Textf(c, "%.0f", s.Thermal.FanSpeed).FontSize(24).Bold()
					ui.Text(c, "RPM").FontSize(11).TextColor(t.TextMuted)
				})
				ui.Text(c, "自动调节").FontSize(10).TextColor(t.TextMuted)
			})
			bat := s.FirstBattery()
			panelTile(c, t, "🔋", "电池", "", func() {
				if bat == nil {
					ui.Text(c, "无电池").FontSize(16).TextColor(t.TextMuted)
					return
				}
				ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
					ui.Textf(c, "%d", bat.Percent).FontSize(24).Bold()
					ui.Text(c, "%").FontSize(14).TextColor(t.TextMuted)
					if s.Thermal.AdapterPower > 0 {
						ui.Textf(c, "⚡%.0fW", s.Thermal.AdapterPower).FontSize(10).TextColor(t.Accent)
					}
				})
				ui.Text(c, bat.Status).FontSize(10).TextColor(t.TextMuted)
			})
			panelTile(c, t, "🗑️", "废纸篓", "", func() {
				ui.Text(c, engine.FmtBytes(s.TrashSize)).FontSize(24).Bold()
				ui.Text(c, "可清空").FontSize(10).TextColor(t.TextMuted)
			})
		})
	})
}

// panelTile: one compact metric card.
func panelTile(c *ui.Context, t ui.Theme, icon, title, badge string, build func()) {
	ui.Column(c).Grow(1).Gap(6).Padding(8, 10).Background(t.Surface).Radius(10).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Row(c).Gap(4).Grow(1).AlignItems(ui.Center).Children(func() {
				ui.Text(c, icon).FontSize(11)
				ui.Text(c, title).FontSize(11).TextColor(t.TextMuted)
			})
			if badge != "" {
				ui.Text(c, badge).FontSize(10).TextColor(t.TextMuted)
			}
		})
		build()
	})
}

// panelProcs: top-5 process list.
func panelProcs(c *ui.Context, s *engine.StatusSnapshot, t ui.Theme) {
	if len(s.TopProcesses) == 0 {
		return
	}
	ui.Column(c).FillWidth().Padding(8, 10).Background(t.Surface).Radius(10).Children(func() {
		ui.Textf(c, "高占用进程 (%d)", s.Procs).FontSize(11).TextColor(t.TextMuted)
		for i, p := range s.TopProcesses {
			if i >= 5 {
				break
			}
			ui.Row(c).FillWidth().Padding(4, 0).AlignItems(ui.Center).Children(func() {
				ui.Text(c, p.Name).FontSize(12).Grow(1).SingleLine()
				ui.Text(c, engine.FmtBytes(p.MemoryBytes)).FontSize(11).TextColor(t.TextMuted).Width(80)
				ui.Textf(c, "%.0f%%", p.CPU).FontSize(11).TextColor(t.TextMuted).Width(50)
			})
		}
	})
}

// panelToolbar: bottom action bar.
func panelToolbar(c *ui.Context, a *app.App, s *engine.StatusSnapshot, t ui.Theme) {
	ui.Row(c).FillWidth().Padding(8, 12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "📷🎤").FontSize(11)
		ui.Text(c, "未在使用").FontSize(10).TextColor(t.TextMuted)
		ui.Spacer(c)
		toolbarBtn(c, "☕", "常亮", a.KeepAwakeOn)
		toolbarBtn(c, "🧹", "清洁", false)
		ui.Spacer(c)
		ui.Column(c).Gap(1).Children(func() {
			ui.Text(c, "9 GB").FontSize(12).Bold()
			ui.Text(c, "已清理").FontSize(9).TextColor(t.TextMuted)
		})
		ui.Column(c).Gap(1).Children(func() {
			ui.Textf(c, "%d", 0).FontSize(12).Bold()
			ui.Text(c, "已卸载").FontSize(9).TextColor(t.TextMuted)
		})
		ui.Column(c).Gap(1).Children(func() {
			ui.Textf(c, "%d", 5).FontSize(12).Bold()
			ui.Text(c, "已优化").FontSize(9).TextColor(t.TextMuted)
		})
	})
}

func toolbarBtn(c *ui.Context, icon, label string, active bool) {
	col := ui.Color{}
	if active {
		col = ui.Hex("#3ecfa8")
	}
	ui.Column(c).Gap(1).Padding(4, 8).Children(func() {
		ui.Text(c, icon).FontSize(14)
		ui.Text(c, label).FontSize(9).TextColor(col)
	})
}

func panelSparkline(c *ui.Context, vals []float64, col ui.Color) {
	if len(vals) < 2 {
		return
	}
	xs := make([]float32, len(vals))
	ys := make([]float32, len(vals))
	for i, v := range vals {
		xs[i] = float32(i) / float32(len(vals)-1)
		ys[i] = float32(1 - v/100)
		if ys[i] < 0 {
			ys[i] = 0
		}
	}
	ui.Box(c).FillWidth().Height(20).Draw(func(p *ui.Painter, r ui.Rect) {
		path := chart.Area(r, xs, ys, nil, chart.CurveLinear)
		p.FillPath(path, ui.RGBA(col.R, col.G, col.B, 0.2))
		line := chart.Line(r, xs, ys, chart.CurveLinear)
		p.StrokePath(line, 1, col)
	})
}

