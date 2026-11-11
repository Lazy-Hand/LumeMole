package main

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lumemole/app"
	"lumemole/engine"
	moleui "lumemole/ui"
)

// MenuBarState holds the tray and its toggles.
type MenuBarState struct {
	Tray          *mygo.Tray
	panel         *mygo.Window // tray popover flyout
	keepAwakeOff  func()       // release from Power.KeepAwake
	caffeinateCmd *exec.Cmd    // "风扇降温" uses caffeinate-like; here we just track

	// Live metrics from status-go, updating the tray title + tooltip.
	metricStop chan struct{}
	metricOn   bool
}

var menuBar MenuBarState

// setupMenuBar builds the tray icon + menu mirroring Mole's 菜单栏 extras.
func setupMenuBar(a *app.App) {
	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID:      "lumemole.tray",
		Title:   "🐹",
		ToolTip: "LumeMole",
		Menu:    trayMenu(a),
	})
	if err != nil {
		return
	}
	menuBar.Tray = tray
	menuBar.startMetrics(a)

	// Left-click opens the status popover; right-click opens the menu.
	tray.OnClick(func() {
		togglePanel(a)
	})
}

// startMetrics streams status-go snapshots into the tray title/tooltip.
// The title shows a compact CPU/MEM readout; the tooltip carries the rest.
func (m *MenuBarState) startMetrics(a *app.App) {
	if m.metricOn {
		return
	}
	p := engine.FindStatusProvider()
	if p == nil {
		return
	}
	m.metricOn = true
	m.metricStop = make(chan struct{})
	snaps, errs := p.Watch(2*time.Second, m.metricStop)
	go func() {
		for {
			select {
			case s, ok := <-snaps:
				if !ok {
					return
				}
				m.applySnapshot(s)
			case _, ok := <-errs:
				if !ok {
					return
				}
			}
		}
	}()
	_ = a // reserved for future "open status page at metric" wiring
}

// applySnapshot refreshes tray text from one status snapshot.
func (m *MenuBarState) applySnapshot(s *engine.StatusSnapshot) {
	if m.Tray == nil {
		return
	}
	// Compact title: figure-space-padded so the bar item doesn't jitter.
	title := fmt.Sprintf("🐹 %s%% %s%%",
		padNum(int(s.CPU.Usage), 2), padNum(int(s.Memory.UsedPercent), 2))
	m.Tray.SetTitle(title)

	var rx, tx float64
	for _, n := range s.Network {
		rx += n.RxRateMBs
		tx += n.TxRateMBs
	}
	tip := fmt.Sprintf("LumeMole · CPU %.0f%% · 内存 %.0f%% · ↓%.1f ↑%.1f MB/s",
		s.CPU.Usage, s.Memory.UsedPercent, rx, tx)
	if len(s.Batteries) > 0 {
		tip += fmt.Sprintf(" · 电池 %d%%", s.Batteries[0].Percent)
	}
	if s.Thermal.FanSpeed > 0 {
		tip += fmt.Sprintf(" · 风扇 %.0f rpm", s.Thermal.FanSpeed)
	}
	m.Tray.SetToolTip(tip)
}

// padNum right-pads a small int with figure spaces for fixed-width digits.
func padNum(n, width int) string {
	s := fmt.Sprintf("%d", n)
	for len(s) < width {
		s = " " + s
	}
	return s
}

// stopMetrics ends the status watch.
func (m *MenuBarState) stopMetrics() {
	if m.metricStop != nil {
		close(m.metricStop)
		m.metricStop = nil
	}
	m.metricOn = false
}

func trayMenu(a *app.App) *mygo.Menu {
	showMain := func(item *mygo.MenuItem, win *mygo.Window) {
		if a.Win != nil {
			a.Win.Show()
			a.Win.Focus()
		}
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: "显示主窗口", Click: showMain},
		mygo.Separator(),
		{Label: "保持常亮", Type: mygo.MenuItemCheckbox, Checked: a.KeepAwakeOn,
			Click: func(item *mygo.MenuItem, _ *mygo.Window) {
				toggleKeepAwake(a)
			}},
		{Label: "清洁屏幕", Click: func(*mygo.MenuItem, *mygo.Window) { showCleanScreen() }},
		{Label: "风扇降温", Click: func(*mygo.MenuItem, *mygo.Window) { a.Tab = app.TabStatus; showMain(nil, nil) }},
		{Label: "网络测速", Click: func(*mygo.MenuItem, *mygo.Window) { go speedTest() }},
		mygo.Separator(),
		{Label: "设置", Click: showMain},
		{Label: "关于 LumeMole", Click: func(*mygo.MenuItem, *mygo.Window) {
			_ = mygo.Shell.OpenExternal("https://github.com/lazyhand/LumeMole")
		}},
		mygo.Separator(),
		{Label: "退出", Accelerator: "CmdOrCtrl+Shift+N", Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	})
}

// togglePanel opens or closes the tray popover flyout.
func togglePanel(a *app.App) {
	if menuBar.panel != nil {
		menuBar.panel.Close()
		menuBar.panel = nil
		a.PanelWin = nil
		return
	}
	panel := mygo.NewFlyout(mygo.FlyoutOptions{
		Tray:       menuBar.Tray,
		Popover:    true,
		Focusable:  true,
		FitContent: true,
		MinWidth:   320,
		MaxWidth:   480,
		MinHeight:  200,
		MaxHeight:  600,
		Content: ui.View(func(c *ui.Context) {
			moleui.MenuPanel(c, a)
		}),
	})
	panel.Show()
	menuBar.panel = panel
	a.PanelWin = panel

	// When the flyout closes (click outside, Escape), clear our ref.
	panel.OnClosed(func() {
		menuBar.panel = nil
		a.PanelWin = nil
	})
}

// toggleKeepAwake uses mygo's native Power.KeepAwake.
func toggleKeepAwake(a *app.App) {
	if a.KeepAwakeOn {
		if menuBar.keepAwakeOff != nil {
			menuBar.keepAwakeOff()
			menuBar.keepAwakeOff = nil
		}
		a.KeepAwakeOn = false
	} else {
		menuBar.keepAwakeOff = mygo.Power.KeepAwake("LumeMole 保持常亮", true)
		a.KeepAwakeOn = true
	}
}

// showCleanScreen opens a fullscreen black window for screen cleaning.
func showCleanScreen() {
	var w *mygo.Window
	w = mygo.NewWindow(mygo.WindowOptions{
		Title:      "清洁屏幕",
		FullScreen: true,
		Frameless:  true,
		Content: ui.View(func(c *ui.Context) {
			ui.Box(c).Fill().Background(ui.Hex("#000000")).OnClick(func() {
				if w != nil {
					w.Close()
				}
			})
			c.OnShortcut(0, ui.KeyEscape, func() {
				if w != nil {
					w.Close()
				}
			})
			ui.Column(c).Fill().Center().Children(func() {
				ui.Text(c, "点击或按 Esc 退出").FontSize(18).TextColor(ui.Hex("#666666"))
			})
		}),
	})
	w.Show()
}

// speedTest runs a quick download+ping measurement, posts results to tray.
func speedTest() {
	// Ping first.
	ping := pingHost("223.5.5.5") // AliDNS
	// Download test via Cloudflare speed endpoint.
	down := downloadSpeed("https://speed.cloudflare.com/__down?bytes=10000000")
	msg := fmt.Sprintf("延迟 %s · 下载 %s", ping, down)
	// Also notify.
	mygo.NewNotification(mygo.NotificationOptions{
		Title: "网络测速",
		Body:  msg,
	}).Show()
}

func pingHost(host string) string {
	start := time.Now()
	err := exec.Command("ping", "-c", "1", "-t", "2", host).Run()
	if err != nil {
		return "—"
	}
	return fmt.Sprintf("%.0fms", time.Since(start).Seconds()*1000)
}

func downloadSpeed(url string) string {
	start := time.Now()
	err := exec.Command("curl", "-fsSL", "-o", "/dev/null",
		"--max-time", "10", url).Run()
	if err != nil {
		return "测速失败"
	}
	sec := time.Since(start).Seconds()
	mb := 10.0 // bytes requested
	return fmt.Sprintf("%.1f MB/s", mb/sec)
}
