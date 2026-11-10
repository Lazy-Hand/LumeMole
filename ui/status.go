package ui

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"lumemole/app"
	"lumemole/engine"
)

// StatusPage renders the system monitor dashboard, replicating Mole's
// 状态 tab: health hero + metric tiles grid + top process table.
func StatusPage(c *ui.Context, a *app.App) {
	t := c.Theme()
	st := statusState(a)

	if !st.Available {
		placeholder(c, "⚠️", "未找到 Mole CLI",
			"状态监控依赖 mole 的 status-go 组件，请先通过 Homebrew 安装 mole:\n  brew install mole")
		return
	}

	// Start the watch once. It stops when we leave the page (handled in shell).
	st.StartWatch(func() {
		if a.Win != nil {
			a.Win.Invalidate()
		}
	})

	snap := st.Snapshot
	if snap == nil {
		ui.Column(c).Fill().Center().Gap(16).Children(func() {
			ui.Spinner(c).Size(28, 28)
			ui.Text(c, "正在采集系统状态…").TextColor(t.TextMuted)
		})
		return
	}

	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(20, 24).Gap(14).Children(func() {
			topRow(c, snap, st, *t)
			midRow(c, snap, st, *t)
			processTable(c, snap, *t)
		})
	})
}

// statusState lazily builds the status state on the app.
func statusState(a *app.App) *engine.StatusState {
	if a.Status == nil {
		a.Status = engine.NewStatusState()
	}
	return a.Status
}

// tile wraps one metric card with a consistent look.
func tile(c *ui.Context, t ui.Theme, icon, title, badge string, badgeColor ui.Color, build func()) {
	ui.Column(c).Grow(1).Gap(10).Padding(14).Background(t.Surface).Radius(12).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Row(c).Gap(6).Grow(1).AlignItems(ui.Center).Children(func() {
				ui.Text(c, icon).FontSize(13)
				ui.Text(c, title).FontSize(13).TextColor(t.TextMuted)
			})
			if badge != "" {
				ui.Text(c, badge).FontSize(11).TextColor(badgeColor)
			}
		})
		build()
	})
}

// topRow: health + CPU + GPU + memory.
func topRow(c *ui.Context, s *engine.StatusSnapshot, st *engine.StatusState, t ui.Theme) {
	ui.Row(c).FillWidth().Gap(14).Children(func() {
		// Health score tile.
		ui.Column(c).Grow(1).Gap(10).Padding(14).Background(t.Surface).Radius(12).Children(func() {
			ui.Row(c).FillWidth().Children(func() {
				ui.Row(c).Gap(6).Grow(1).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "🍀").FontSize(13)
					ui.Text(c, "健康度").FontSize(13).TextColor(t.TextMuted)
				})
				ui.Text(c, s.Hardware.CPUModel).FontSize(11).TextColor(t.TextMuted)
			})
			ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					ui.Row(c).AlignItems(ui.Baseline).Gap(8).Children(func() {
						ui.Textf(c, "%d", s.HealthScore).FontSize(34).Bold()
						ui.Text(c, engine.HealthLabel(s.HealthScore)).
							FontSize(15).TextColor(ui.Hex(engine.HealthColorHex(s.HealthScore)))
					})
					ui.Text(c, s.HealthScoreMsg).FontSize(12).TextColor(t.TextMuted)
					ui.Text(c, "已运行 "+s.Uptime).FontSize(11).TextColor(t.TextMuted)
				})
				ui.Spacer(c)
			})
		})

		// CPU
		tile(c, t, "🧠", "CPU", tempStr(s.Thermal.CPUTemp), ui.Color{}, func() {
			ui.Row(c).AlignItems(ui.Baseline).Gap(6).Children(func() {
				ui.Textf(c, "%.0f", s.CPU.Usage).FontSize(30).Bold()
				ui.Text(c, "%").FontSize(18).TextColor(t.TextMuted)
			})
			sparkBars(c, s.CPU.PerCore, 8, t.Accent)
			ui.Textf(c, "%d 核 · 平均负载 %.1f", s.CPU.CoreCount, s.CPU.Load1).
				FontSize(11).TextColor(t.TextMuted)
		})

		// GPU
		tile(c, t, "🎮", "GPU", tempStr(s.Thermal.GPUTemp), ui.Color{}, func() {
			g := s.PrimaryGPU()
			usage := 0.0
			if g != nil && g.Usage >= 0 {
				usage = g.Usage
			}
			ui.Row(c).AlignItems(ui.Baseline).Gap(6).Children(func() {
				ui.Textf(c, "%.0f", usage).FontSize(30).Bold()
				ui.Text(c, "%").FontSize(18).TextColor(t.TextMuted)
			})
			if len(st.CPUHist) > 1 {
				sparkline(c, st.CPUHist, 1, t.Accent)
			} else {
				ui.Text(c, "—").FontSize(20).TextColor(t.TextMuted)
			}
			ui.Text(c, "低负载").FontSize(11).TextColor(t.TextMuted)
		})

		// Memory
		memColor := t.Accent
		if s.Memory.UsedPercent > 85 {
			memColor = ui.Hex("#e06c3c")
		} else if s.Memory.UsedPercent > 70 {
			memColor = ui.Hex("#e0c53c")
		}
		tile(c, t, "💾", "内存", memPressure(s), memColor, func() {
			ui.Row(c).AlignItems(ui.Baseline).Gap(6).Children(func() {
				ui.Textf(c, "%.0f", s.Memory.UsedPercent).FontSize(30).Bold()
				ui.Text(c, "%").FontSize(18).TextColor(t.TextMuted)
			})
			if len(st.MemHist) > 1 {
				sparkline(c, st.MemHist, 1, memColor)
			}
			ui.Textf(c, "%.1f GB · 交换 %.1f GB",
				float64(s.Memory.Used)/1e9, float64(s.Memory.SwapUsed)/1e9).
				FontSize(11).TextColor(t.TextMuted)
		})
	})
}

// midRow: battery + disk + network + fan.
func midRow(c *ui.Context, s *engine.StatusSnapshot, st *engine.StatusState, t ui.Theme) {
	ui.Row(c).FillWidth().Gap(14).Children(func() {
		// Battery
		bat := s.FirstBattery()
		batBadge := ""
		if bat != nil {
			batBadge = fmt.Sprintf("健康 %d%%", bat.Capacity)
		}
		tile(c, t, "🔋", "电池", batBadge, ui.Color{}, func() {
			if bat == nil {
				ui.Text(c, "无电池").FontSize(20).TextColor(t.TextMuted)
				return
			}
			ui.Row(c).AlignItems(ui.Baseline).Gap(6).Children(func() {
				ui.Textf(c, "%d", bat.Percent).FontSize(30).Bold()
				ui.Text(c, "%").FontSize(18).TextColor(t.TextMuted)
				if s.Thermal.AdapterPower > 0 {
					ui.Textf(c, "⚡%.0fW", s.Thermal.AdapterPower).FontSize(13).TextColor(t.Accent)
				}
			})
			ui.Text(c, bat.Status+" · 循环 "+fmt.Sprint(bat.CycleCount)+" 次").
				FontSize(11).TextColor(t.TextMuted)
		})

		// Disk
		d := s.PrimaryDisk()
		tile(c, t, "💿", "可用空间", engine.FmtBytes(s.TrashSize)+" 废纸篓", ui.Color{}, func() {
			if d == nil {
				ui.Text(c, "—").FontSize(20).TextColor(t.TextMuted)
				return
			}
			free := d.Total - d.Used
			ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
				ui.Text(c, engine.FmtBytes(free)).FontSize(30).Bold()
			})
			ui.Progress(c, d.UsedPercent/100).FillWidth()
			ui.Textf(c, "已用 %s · %.0f%%", engine.FmtBytes(d.Used), d.UsedPercent).
				FontSize(11).TextColor(t.TextMuted)
		})

		// Network
		tile(c, t, "🌐", "网络", "Wi-Fi", ui.Color{}, func() {
			rx, tx := s.TotalNetRate()
			ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
				ui.Text(c, engine.FmtMBs(rx)).FontSize(28).Bold()
				ui.Text(c, "↓").FontSize(14).TextColor(t.TextMuted)
			})
			if len(st.RxHist) > 1 {
				sparkline(c, st.RxHist, 1, ui.Hex("#4aa3e0"))
			}
			ui.Textf(c, "上传 %s", engine.FmtMBs(tx)).FontSize(11).TextColor(t.TextMuted)
		})

		// Fan
		tile(c, t, "🌀", "风扇", fmt.Sprintf("转速 %.0f%%", s.Thermal.FanSpeed), ui.Color{}, func() {
			ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
				ui.Textf(c, "%.0f", s.Thermal.FanSpeed).FontSize(30).Bold()
				ui.Text(c, "RPM").FontSize(13).TextColor(t.TextMuted)
			})
			ui.Text(c, "由系统自动调节").FontSize(11).TextColor(t.TextMuted)
		})
	})
}

// processTable renders the top-process table.
func processTable(c *ui.Context, s *engine.StatusSnapshot, t ui.Theme) {
	cols := []ui.TableColumn{
		{Title: "名称", Width: 220, Sortable: true},
		{Title: "内存", Width: 90, Align: ui.End, Sortable: true},
		{Title: "% CPU", Width: 80, Align: ui.End, Sortable: true},
		{Title: "PID", Width: 80, Align: ui.End, Sortable: true},
	}
	ui.Column(c).FillWidth().Grow(1).Padding(14).Background(t.Surface).Radius(12).Children(func() {
		ui.Row(c).FillWidth().Padding(0, 0, 10, 0).Children(func() {
			ui.Textf(c, "进程 (%d)", s.Procs).FontSize(13).TextColor(t.TextMuted).Grow(1)
		})
		ui.Table(c, processListState, cols, len(s.TopProcesses), func(row, col int) {
			if row >= len(s.TopProcesses) {
				return
			}
			p := s.TopProcesses[row]
			switch col {
			case 0:
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, procIcon(p.Name)).FontSize(14)
					ui.Text(c, p.Name).FontSize(13).SingleLine()
				})
			case 1:
				ui.Text(c, engine.FmtBytes(p.MemoryBytes)).FontSize(12).TextColor(t.TextMuted)
			case 2:
				ui.Textf(c, "%.1f", p.CPU).FontSize(12)
			case 3:
				ui.Textf(c, "%d", p.PID).FontSize(12).TextColor(t.TextMuted)
			}
		}).Grow(1)
	})
}

// processListState persists table selection/scroll across frames.
var processListState = &ui.ListState{}

func procIcon(name string) string {
	switch {
	case len(name) > 6 && name[:6] == "kernel":
		return "⚙️"
	default:
		return "▫️"
	}
}

func tempStr(v float64) string {
	if v <= 0 {
		return ""
	}
	return fmt.Sprintf("%.0f°C", v)
}

func memPressure(s *engine.StatusSnapshot) string {
	if s.Memory.UsedPercent > 85 {
		return "压力高"
	}
	if s.Memory.UsedPercent > 70 {
		return "压力偏高"
	}
	return ""
}

// sparkBars draws per-core CPU usage as a row of small bars.
func sparkBars(c *ui.Context, vals []float64, n int, col ui.Color) {
	if len(vals) == 0 {
		return
	}
	count := len(vals)
	if count > n {
		count = n
	}
	ui.Row(c).Gap(3).Children(func() {
		for i := 0; i < count; i++ {
			h := float32(6 + vals[i]/100*18)
			ui.Box(c).Size(8, h).Radius(2).Background(col)
		}
	})
}

// sparkline draws a small filled area chart from history values (0..100 scale
// assumed; values are clamped).
func sparkline(c *ui.Context, vals []float64, max float64, col ui.Color) {
	if len(vals) < 2 {
		return
	}
	if max <= 0 {
		max = 100
	}
	xs := make([]float32, len(vals))
	ys := make([]float32, len(vals))
	for i, v := range vals {
		xs[i] = float32(i) / float32(len(vals)-1)
		ys[i] = float32(1 - math.Min(v/max, 1))
	}
	ui.Box(c).FillWidth().Height(36).Draw(func(p *ui.Painter, r ui.Rect) {
		path := chart.Area(r, xs, ys, nil, chart.CurveLinear)
		p.FillPath(path, ui.RGBA(col.R, col.G, col.B, 0.25))
		line := chart.Line(r, xs, ys, chart.CurveLinear)
		p.StrokePath(line, 1.5, col)
	})
}
