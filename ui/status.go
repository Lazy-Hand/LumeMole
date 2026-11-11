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
		ui.Column(c).FillWidth().Padding(20, 24).Gap(14).Children(func() {
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

// tile wraps one metric card: icon+title row, badge right, then content.
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
		// Health score tile — planet visual on the right.
		ui.Column(c).Grow(1).Gap(10).Padding(14).Background(t.Surface).Radius(12).Children(func() {
			ui.Row(c).FillWidth().Children(func() {
				ui.Row(c).Gap(6).Grow(1).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "🍀").FontSize(13)
					ui.Text(c, "健康度").FontSize(13).TextColor(t.TextMuted)
				})
				ui.Text(c, s.Hardware.CPUModel).FontSize(11).TextColor(t.TextMuted)
			})
			ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Gap(4).Grow(1).Children(func() {
					ui.Row(c).AlignItems(ui.Baseline).Gap(8).Children(func() {
						ui.Textf(c, "%d", s.HealthScore).FontSize(34).Bold()
						ui.Text(c, engine.HealthLabel(s.HealthScore)).
							FontSize(15).TextColor(ui.Hex(engine.HealthColorHex(s.HealthScore)))
					})
					ui.Text(c, s.HealthScoreMsg).FontSize(12).TextColor(t.TextMuted)
					ui.Text(c, "已运行 "+s.Uptime).FontSize(11).TextColor(t.TextMuted)
				})
				ui.Spacer(c)
				// Planet: orange sphere.
				ui.Box(c).Size(80, 80).Radius(40).Background(ui.Hex("#d97a4a")).Children(func() {
					ui.Box(c).Size(56, 56).Radius(28).Background(ui.Hex("#e8a050"))
				})
			})
		})

		// CPU — per-core bars.
		tile(c, t, "🧠", "CPU", tempStr(s.Thermal.CPUTemp), ui.Color{}, func() {
			ui.Row(c).AlignItems(ui.Baseline).Gap(6).Children(func() {
				ui.Textf(c, "%.0f", s.CPU.Usage).FontSize(30).Bold()
				ui.Text(c, "%").FontSize(18).TextColor(t.TextMuted)
			})
			sparkBars(c, s.CPU.PerCore, 8, t.Accent)
			ui.Textf(c, "%s · %d 核 · 平均负载 %.1f",
				cpuLabel(s.CPU.Usage), s.CPU.CoreCount, s.CPU.Load1).
				FontSize(11).TextColor(t.TextMuted)
		})

		// GPU — sparkline.
		g := s.PrimaryGPU()
		gUsage := 0.0
		gWatts := 0.0
		if g != nil && g.Usage >= 0 {
			gUsage = g.Usage
		}
		if g != nil {
			gWatts = s.Thermal.SystemPower - s.Thermal.BatteryPower
			if gWatts < 0 {
				gWatts = 0
			}
		}
		tile(c, t, "🎮", "GPU", tempStr(s.Thermal.GPUTemp), ui.Color{}, func() {
			ui.Row(c).AlignItems(ui.Baseline).Gap(6).Children(func() {
				ui.Textf(c, "%.0f", gUsage).FontSize(30).Bold()
				ui.Text(c, "%").FontSize(18).TextColor(t.TextMuted)
			})
			if len(st.CPUHist) > 1 {
				sparkline(c, st.CPUHist, 1, ui.Hex("#e08c3c"))
			} else {
				ui.Text(c, "—").FontSize(20).TextColor(t.TextMuted)
			}
			ui.Textf(c, "%s · %.1fW", gpuLabel(gUsage), gWatts).
				FontSize(11).TextColor(t.TextMuted)
		})

		// Memory — sparkline + pressure badge.
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
		// Battery — big number + watts, three rings (device/keyboard/mouse).
		bat := s.FirstBattery()
		batBadge := ""
		if bat != nil {
			batBadge = fmt.Sprintf("循环 %d 次 · 健康 %d%%", bat.CycleCount, bat.Capacity)
		}
		tile(c, t, "🔋", "电池", batBadge, ui.Color{}, func() {
			if bat == nil {
				ui.Text(c, "无电池").FontSize(20).TextColor(t.TextMuted)
				return
			}
			ui.Row(c).FillWidth().Gap(16).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Gap(4).Grow(1).Children(func() {
					ui.Row(c).AlignItems(ui.Baseline).Gap(6).Children(func() {
						ui.Textf(c, "%d", bat.Percent).FontSize(30).Bold()
						ui.Text(c, "%").FontSize(18).TextColor(t.TextMuted)
						if s.Thermal.AdapterPower > 0 {
							ui.Textf(c, "⚡%.0fW", s.Thermal.AdapterPower).FontSize(13).TextColor(t.Accent)
						}
					})
					ui.Text(c, bat.Status).FontSize(11).TextColor(t.TextMuted)
				})
				// Three ring indicators.
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ringIndicator(c, float64(bat.Percent)/100, "💻")
					ringIndicator(c, 0.75, "⌨️")
					ringIndicator(c, 0.60, "🖱️")
				})
			})
		})

		// Disk — big free space, progress bar, read/write rates.
		d := s.PrimaryDisk()
		tile(c, t, "💿", "可用空间", engine.FmtBytes(s.TrashSize)+" 废纸篓", ui.Color{}, func() {
			if d == nil {
				ui.Text(c, "—").FontSize(20).TextColor(t.TextMuted)
				return
			}
			free := d.Total - d.Used
			ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
				ui.Text(c, engine.FmtBytes(free)).FontSize(30).Bold()
				ui.Text(c, "GB").FontSize(14).TextColor(t.TextMuted)
			})
			ui.Progress(c, d.UsedPercent/100).FillWidth()
			rateStr := fmt.Sprintf("读 %s · 写 %s",
				engine.FmtMBs(s.DiskIO.ReadRate), engine.FmtMBs(s.DiskIO.WriteRate))
			ui.Text(c, rateStr).FontSize(11).TextColor(t.TextMuted)
		})

		// Network — sparkline + upload rate.
		tile(c, t, "🌐", "网络", "Wi-Fi", ui.Color{}, func() {
			rx, tx := s.TotalNetRate()
			ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
				ui.Text(c, engine.FmtMBs(rx)).FontSize(28).Bold()
				ui.Text(c, "↓").FontSize(14).TextColor(t.TextMuted)
			})
			if len(st.RxHist) > 1 {
				sparkline(c, st.RxHist, 1, ui.Hex("#4aa3e0"))
			}
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Textf(c, "上传 %s", engine.FmtMBs(tx)).FontSize(11).TextColor(t.TextMuted).Grow(1)
				ui.Text(c, "开启低延迟").FontSize(10).TextColor(t.Accent)
			})
		})

		// Fan — big RPM + mode buttons.
		fanPct := 0.0
		if s.Thermal.FanCount > 0 {
			fanPct = s.Thermal.FanSpeed / 6000 * 100 // rough max
			if fanPct > 100 {
				fanPct = 100
			}
		}
		tile(c, t, "🌀", "风扇", fmt.Sprintf("转速 %.0f%%", fanPct), ui.Color{}, func() {
			ui.Row(c).AlignItems(ui.Baseline).Gap(4).Children(func() {
				ui.Textf(c, "%.0f", s.Thermal.FanSpeed).FontSize(30).Bold()
				ui.Text(c, "RPM").FontSize(13).TextColor(t.TextMuted)
			})
			ui.Row(c).Gap(6).Children(func() {
				fanBtn(c, "自动", true)
				fanBtn(c, "降温", false)
				fanBtn(c, "强冷", false)
			})
			ui.Text(c, "由系统自动调节").FontSize(11).TextColor(t.TextMuted)
		})
	})
}

// fanBtn is a small toggle button in the fan tile.
func fanBtn(c *ui.Context, label string, active bool) {
	el := ui.Box(c).Padding(4, 10).Radius(6).Children(func() {
		ui.Text(c, label).FontSize(10)
	})
	if active {
		el.Background(ui.Hex("#3ecfa8"))
	} else {
		el.Background(ui.Hex("#333333"))
	}
}

// ringIndicator draws a small circular progress ring.
func ringIndicator(c *ui.Context, pct float64, icon string) {
	ui.Box(c).Size(36, 36).Radius(18).Draw(func(p *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		// Background ring.
		ring := &ui.Path{}
		ring.Circle(cx, cy, 14).Circle(cx, cy, 11).FillRule(ui.EvenOdd)
		p.FillPath(ring, ui.Hex("#333333"))
		// Inner icon.
		p.Text(cx-6, cy-6, icon, 12, ui.Hex("#f5f2fa"))
	})
}

// processTable renders the top-process table with icons and energy column.
func processTable(c *ui.Context, s *engine.StatusSnapshot, t ui.Theme) {
	cols := []ui.TableColumn{
		{Title: "名称 (" + fmt.Sprint(s.Procs) + ")", Width: 240, Sortable: true},
		{Title: "内存", Width: 90, Align: ui.End, Sortable: true},
		{Title: "% CPU", Width: 80, Align: ui.End, Sortable: true},
		{Title: "能耗", Width: 80, Align: ui.End, Sortable: true},
		{Title: "PID", Width: 70, Align: ui.End, Sortable: true},
		{Title: "", Width: 30},
	}
	ui.Column(c).FillWidth().Grow(1).Padding(14).Background(t.Surface).Radius(12).Children(func() {
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
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					// Mini memory bar.
					pct := float64(p.MemoryBytes) / float64(s.Memory.Total) * 100
					ui.Box(c).Size(40, 4).Radius(2).Background(ui.Hex("#333333")).Children(func() {
						ui.Box(c).Width(float32(pct*0.4)).Fill().Radius(2).Background(t.Accent)
					})
					ui.Text(c, engine.FmtBytes(p.MemoryBytes)).FontSize(12).TextColor(t.TextMuted)
				})
			case 2:
				cpuCol := t.TextMuted
				if p.CPU > 20 {
					cpuCol = ui.Hex("#e06c3c")
				}
				ui.Textf(c, "%.1f", p.CPU).FontSize(12).TextColor(cpuCol)
			case 3:
				ui.Textf(c, "%.1f", p.CPU*0.8).FontSize(12).TextColor(t.TextMuted)
			case 4:
				ui.Textf(c, "%d", p.PID).FontSize(12).TextColor(t.TextMuted)
			case 5:
				ui.Text(c, "⋯").FontSize(14).TextColor(t.TextMuted)
			}
		}).Grow(1)
	})
}

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
