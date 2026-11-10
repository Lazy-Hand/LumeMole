package main

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lumemole/app"
)

// MenuBarState holds the tray and its toggles.
type MenuBarState struct {
	Tray          *mygo.Tray
	keepAwakeOff  func() // release from Power.KeepAwake
	keepAwakeOn   bool
	caffeinateCmd *exec.Cmd // "风扇降温" uses caffeinate-like; here we just track
}

var menuBar MenuBarState

// setupMenuBar builds the tray icon + menu mirroring Mole's 菜单栏 extras.
func setupMenuBar(a *app.App) {
	// 18x18 template PNG (a simple mole-ish dot); generated at runtime
	// would need an encoder — use a small built-in via image/png later.
	// For now: title-only tray (text shows in menu bar).
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
		{Label: "保持常亮", Type: mygo.MenuItemCheckbox, Checked: menuBar.keepAwakeOn,
			Click: func(item *mygo.MenuItem, _ *mygo.Window) {
				toggleKeepAwake()
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

// toggleKeepAwake uses mygo's native Power.KeepAwake.
func toggleKeepAwake() {
	if menuBar.keepAwakeOn {
		if menuBar.keepAwakeOff != nil {
			menuBar.keepAwakeOff()
			menuBar.keepAwakeOff = nil
		}
		menuBar.keepAwakeOn = false
	} else {
		menuBar.keepAwakeOff = mygo.Power.KeepAwake("LumeMole 保持常亮", true)
		menuBar.keepAwakeOn = true
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
	if menuBar.Tray != nil {
		menuBar.Tray.SetToolTip(msg)
	}
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
