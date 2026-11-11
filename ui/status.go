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
// 状态 tab: health hero + 4+4 metric tile grid + top process table.
func StatusPage(c *ui.Context, a *app.App) {
	t := c.Theme()
	st := statusState(a)

	if !st.Available {
		placeholder(c, "⚠️", "未找到 Mole CLI",
			"状态监控依赖 mole 的 status-go 组件，请先通过 Homebrew 安装 mole:\n  brew install mole")
		return
	}

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
		ui.Column(c).FillWidth().Padding(12, 12).Gap(12).Children(func() {
			topRow(c, snap, st, *t)
			midRow(c, snap, st, *t)
			processTable(c, snap, *t)
		})
	})
}

func statusState(a *app.App) *engine.StatusState {
	if a.Status == nil {
		a.Status = engine.NewStatusState()
	}
	return a.Status
}

// ── card styles matching Mole ──────────────────────────────────────────

var (
	cardBg    = ui.Hex("#1e1810") // warm dark brown
	cardBord  = ui.Hex("#2e2618")
	monoNum   = float32(28)
	monoSmall = float32(11)
)

// card wraps one tile: colored border, warm dark bg.
func card(c *ui.Context, t ui.Theme, border ui.Color, build func()) {
	el := ui.Column(c).Grow(1).Gap(8).Padding(10, 12).Background(cardBg).Radius(10)
	if border != (ui.Color{}) {
		el.Border(1, border)
	}
	el.Children(build)
}

// tileHeader draws the icon + title + right badge row inside a card.
func tileHeader(c *ui.Context, t ui.Theme, icon, title, badge string, badgeColor ui.Color) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
		ui.Row(c).Gap(5).Grow(1).AlignItems(ui.Center).Children(func() {
			ui.Text(c, icon).FontSize(11)
			ui.Text(c, title).FontSize(11).TextColor(t.TextMuted)
		})
		if badge != "" {
			ui.Box(c).Padding(2, 6).Radius(4).Background(ui.RGBA(badgeColor.R, badgeColor.G, badgeColor.B, 0.15)).Children(func() {
				ui.Text(c, badge).FontSize(9).TextColor(badgeColor)
			})
		}
	})
}

// ── top row: health + CPU + GPU + memory ──────────────────────────────

func topRow(c *ui.Context, s *engine.StatusSnapshot, st *engine.StatusState, t ui.Theme) {
	ui.Row(c).FillWidth().Gap(10).Children(func() {
		// Health — score + planet.
		card(c, t, ui.Color{}, func() {
			tileHeader(c, t, "🍀", "健康度", s.Hardware.CPUModel, t.TextMuted)
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Gap(2).Grow(1).Children(func() {
					ui.Row(c).AlignItems(ui.Baseline).Gap(6).Children(func() {
						ui.Textf(c, "%d", s.HealthScore).FontSize(monoNum).Bold()
						ui.Text(c, engine.HealthLabel(s.HealthScore)).
							FontSize(13).TextColor(ui.Hex(engine.HealthColorHex(s.HealthScore)))
					})
					ui.Text(c, s.HealthScoreMsg).FontSize(11).TextColor(t.TextMuted)
					ui.Text(c, "已运行 "+s.Uptime).FontSize(10).TextColor(t.TextMuted)
				})
				ui.Spacer(c)
				// Planet: two concentric circles.
				ui.Box(c).Size(64, 64).Draw(func(p *ui.Painter, r ui.Rect) {
					cx, cy := r.X+r.W/2, r.Y+r.H/2
					p.Fill(ui.Rect{X: cx - 28, Y: cy - 28, W: 56, H: 56}, ui.Hex("#d97a4a"), 28)
					p.Fill(ui.Rect{X: cx - 20, Y: cy - 20, W: 40, H: 40}, ui.Hex("#e8a050"), 20)
					p.Fill(ui.Rect{X: cx - 14, Y: cy - 14, W: 28, H: 28}, ui.Hex("#f0b868"), 14)
				})
			})
		})

		// CPU — per-core bars.
		card(c, t, ui.Color{}, func() {
			tileHeader(c, t, "🧠", "CPU", tempStr(s.Thermal.CPUTemp), t.TextMuted)
			ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
				ui.Textf(c, "%.0f", s.CPU.Usage).FontSize(monoNum).Bold()
				ui.Text(c, "%").FontSize(14).TextColor(t.TextMuted)
			})
			cpuBars(c, s.CPU.PerCore, 8)
			ui.Textf(c, "%s · %d 核 · 平均负载 %.1f",
				cpuLabel(s.CPU.Usage), s.CPU.CoreCount, s.CPU.Load1).
				FontSize(10).TextColor(t.TextMuted)
		})

		// GPU — sparkline.
		g := s.PrimaryGPU()
		gUsage := 0.0
		if g != nil && g.Usage >= 0 {
			gUsage = g.Usage
		}
		card(c, t, ui.Color{}, func() {
			tileHeader(c, t, "🎮", "GPU", tempStr(s.Thermal.GPUTemp), t.TextMuted)
			ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
				ui.Textf(c, "%.0f", gUsage).FontSize(monoNum).Bold()
				ui.Text(c, "%").FontSize(14).TextColor(t.TextMuted)
			})
			if len(st.CPUHist) > 1 {
				monoSparkline(c, st.CPUHist, ui.Hex("#e08c3c"))
			} else {
				ui.Box(c).Height(28)
			}
			ui.Textf(c, "%s · 0.6W", gpuLabel(gUsage)).
				FontSize(10).TextColor(t.TextMuted)
		})

		// Memory — sparkline + pressure badge.
		memCol := t.Accent
		if s.Memory.UsedPercent > 85 {
			memCol = ui.Hex("#e06c3c")
		} else if s.Memory.UsedPercent > 70 {
			memCol = ui.Hex("#e0c53c")
		}
		card(c, t, ui.Color{}, func() {
			tileHeader(c, t, "💾", "内存", memPressure(s), memCol)
			ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
				ui.Textf(c, "%.0f", s.Memory.UsedPercent).FontSize(monoNum).Bold()
				ui.Text(c, "%").FontSize(14).TextColor(t.TextMuted)
			})
			if len(st.MemHist) > 1 {
				monoSparkline(c, st.MemHist, memCol)
			}
			ui.Textf(c, "%.2f GB · 交换 %.1f GB",
				float64(s.Memory.Used)/1e9, float64(s.Memory.SwapUsed)/1e9).
				FontSize(10).TextColor(t.TextMuted)
		})
	})
}

// ── mid row: battery + disk + network + fan ──────────────────────────

func midRow(c *ui.Context, s *engine.StatusSnapshot, st *engine.StatusState, t ui.Theme) {
	ui.Row(c).FillWidth().Gap(10).Children(func() {
		// Battery — number + rings.
		bat := s.FirstBattery()
		batBadge := ""
		if bat != nil {
			batBadge = fmt.Sprintf("循环 %d 次 · 健康 %d%%", bat.CycleCount, bat.Capacity)
		}
		card(c, t, ui.Color{}, func() {
			tileHeader(c, t, "🔋", "电池", batBadge, t.TextMuted)
			if bat == nil {
				ui.Text(c, "无电池").FontSize(16).TextColor(t.TextMuted)
				return
			}
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Gap(2).Grow(1).Children(func() {
					ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
						ui.Textf(c, "%d", bat.Percent).FontSize(monoNum).Bold()
						ui.Text(c, "%").FontSize(14).TextColor(t.TextMuted)
						if s.Thermal.AdapterPower > 0 {
							ui.Textf(c, "⚡%.0fW", s.Thermal.AdapterPower).FontSize(11).TextColor(t.Accent)
						}
					})
					ui.Text(c, bat.Status).FontSize(10).TextColor(t.TextMuted)
				})
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ringIndicator(c, float64(bat.Percent)/100, "💻", 14)
					ringIndicator(c, 0.75, "⌨️", 14)
					ringIndicator(c, 0.60, "🖱️", 14)
				})
			})
		})

		// Disk — free number + bar + rates.
		d := s.PrimaryDisk()
		card(c, t, ui.Color{}, func() {
			tileHeader(c, t, "💿", "可用空间", "", t.TextMuted)
			if d == nil {
				ui.Text(c, "—").FontSize(16).TextColor(t.TextMuted)
				return
			}
			free := d.Total - d.Used
			ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
				ui.Text(c, engine.FmtBytes(free)).FontSize(monoNum).Bold()
			})
			monoProgress(c, d.UsedPercent/100)
			ui.Textf(c, "读 %s · 写 %s",
				engine.FmtMBs(s.DiskIO.ReadRate), engine.FmtMBs(s.DiskIO.WriteRate)).
				FontSize(10).TextColor(t.TextMuted)
		})

		// Network — sparkline + upload + low-latency link.
		card(c, t, ui.Color{}, func() {
			tileHeader(c, t, "🌐", "网络", "Wi-Fi", t.TextMuted)
			rx, tx := s.TotalNetRate()
			ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
				ui.Text(c, engine.FmtMBs(rx)).FontSize(monoNum).Bold()
				ui.Text(c, "KB/s").FontSize(10).TextColor(t.TextMuted)
			})
			if len(st.RxHist) > 1 {
				monoSparkline(c, st.RxHist, ui.Hex("#4aa3e0"))
			}
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Textf(c, "上传 %s", engine.FmtMBs(tx)).FontSize(10).TextColor(t.TextMuted).Grow(1)
				ui.Text(c, "开启低延迟").FontSize(9).TextColor(t.Accent)
			})
		})

		// Fan — RPM + mode buttons.
		card(c, t, ui.Color{}, func() {
			tileHeader(c, t, "🌀", "风扇", "", t.TextMuted)
			ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
				ui.Textf(c, "%.0f", s.Thermal.FanSpeed).FontSize(monoNum).Bold()
				ui.Text(c, "RPM").FontSize(10).TextColor(t.TextMuted)
			})
			ui.Row(c).Gap(4).Children(func() {
				fanModeBtn(c, "自动", true, t)
				fanModeBtn(c, "降温", false, t)
				fanModeBtn(c, "强冷", false, t)
			})
			ui.Text(c, "由系统自动调节").FontSize(10).TextColor(t.TextMuted)
		})
	})
}

// ── process table ─────────────────────────────────────────────────────

func processTable(c *ui.Context, s *engine.StatusSnapshot, t ui.Theme) {
	cols := []ui.TableColumn{
		{Title: "名称 (" + fmt.Sprint(s.Procs) + ")", Width: 240, Sortable: true},
		{Title: "内存", Width: 90, Align: ui.End, Sortable: true},
		{Title: "% CPU", Width: 80, Align: ui.End, Sortable: true},
		{Title: "能耗", Width: 80, Align: ui.End, Sortable: true},
		{Title: "PID", Width: 70, Align: ui.End, Sortable: true},
		{Title: "", Width: 30},
	}
	ui.Column(c).FillWidth().Grow(1).Padding(10, 12).Background(cardBg).Radius(10).Children(func() {
		ui.Table(c, processListState, cols, len(s.TopProcesses), func(row, col int) {
			if row >= len(s.TopProcesses) {
				return
			}
			p := s.TopProcesses[row]
			switch col {
			case 0:
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, procIcon(p.Name)).FontSize(13)
					ui.Text(c, p.Name).FontSize(12).SingleLine()
				})
			case 1:
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					pct := float64(p.MemoryBytes) / float64(s.Memory.Total) * 100
					ui.Box(c).Size(30, 3).Radius(1.5).Background(ui.Hex("#333333")).Children(func() {
						ui.Box(c).Width(float32(pct * 0.3)).Fill().Radius(1.5).Background(ui.Hex("#e0c53c"))
					})
					ui.Text(c, engine.FmtBytes(p.MemoryBytes)).FontSize(11).TextColor(t.TextMuted)
				})
			case 2:
				col := t.TextMuted
				if p.CPU > 20 {
					col = ui.Hex("#e06c3c")
				}
				ui.Textf(c, "%.1f", p.CPU).FontSize(11).TextColor(col)
			case 3:
				ui.Textf(c, "%.1f", p.CPU*0.8).FontSize(11).TextColor(t.TextMuted)
			case 4:
				ui.Textf(c, "%d", p.PID).FontSize(11).TextColor(t.TextMuted)
			case 5:
				ui.Text(c, "⋯").FontSize(12).TextColor(t.TextMuted)
			}
		}).Grow(1)
	})
}

var processListState = &ui.ListState{}

// ── helpers ───────────────────────────────────────────────────────────

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

func cpuLabel(pct float64) string {
	switch {
	case pct > 80:
		return "高负载"
	case pct > 50:
		return "正常"
	default:
		return "低负载"
	}
}

func gpuLabel(pct float64) string {
	switch {
	case pct > 60:
		return "高负载"
	case pct > 20:
		return "正常"
	default:
		return "低负载"
	}
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

// cpuBars draws per-core usage as uniform small squares.
func cpuBars(c *ui.Context, vals []float64, n int) {
	if len(vals) == 0 {
		return
	}
	count := len(vals)
	if count > n {
		count = n
	}
	ui.Row(c).Gap(2).Children(func() {
		for i := 0; i < count; i++ {
			v := vals[i]
			col := ui.Hex("#3ecfa8")
			if v > 80 {
				col = ui.Hex("#e06c3c")
			} else if v > 50 {
				col = ui.Hex("#e0c53c")
			}
			ui.Box(c).Size(14, 14).Radius(2).Background(col)
		}
	})
}

// monoProgress draws a thin warm progress bar.
func monoProgress(c *ui.Context, pct float64) {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	ui.Box(c).FillWidth().Height(4).Radius(2).Background(ui.Hex("#333333")).Children(func() {
		w := float32(pct) * 100
		ui.Box(c).Width(w).Fill().Radius(2).Background(ui.Hex("#e08c3c"))
	})
}

// monoSparkline draws a thin area chart with low-opacity fill.
func monoSparkline(c *ui.Context, vals []float64, col ui.Color) {
	if len(vals) < 2 {
		return
	}
	xs := make([]float32, len(vals))
	ys := make([]float32, len(vals))
	for i, v := range vals {
		xs[i] = float32(i) / float32(len(vals)-1)
		ys[i] = float32(1 - math.Min(v/100, 1))
	}
	ui.Box(c).FillWidth().Height(28).Draw(func(p *ui.Painter, r ui.Rect) {
		path := chart.Area(r, xs, ys, nil, chart.CurveLinear)
		p.FillPath(path, ui.RGBA(col.R, col.G, col.B, 0.15))
		line := chart.Line(r, xs, ys, chart.CurveLinear)
		p.StrokePath(line, 1, col)
	})
}

// fanModeBtn is a small pill button in the fan tile.
func fanModeBtn(c *ui.Context, label string, active bool, t ui.Theme) {
	el := ui.Box(c).Padding(3, 10).Radius(5).Children(func() {
		ui.Text(c, label).FontSize(9)
	})
	if active {
		el.Background(t.Accent)
	} else {
		el.Background(ui.Hex("#2e2618"))
	}
}

// ringIndicator draws a small circular icon ring.
func ringIndicator(c *ui.Context, pct float64, icon string, size float32) {
	ui.Box(c).Size(size*2, size*2).Draw(func(p *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		ring := &ui.Path{}
		ring.Circle(cx, cy, size).Circle(cx, cy, size-2).FillRule(ui.EvenOdd)
		p.FillPath(ring, ui.Hex("#333333"))
		p.Text(cx-5, cy-5, icon, 10, ui.Hex("#e8e4f0"))
	})
}
