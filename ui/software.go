package ui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lumemole/app"
	"lumemole/engine"
)

// SoftwarePage replicates Mole's 软件 tab: app inventory with sizes,
// last-used info, per-app uninstall via remnant scan.
func SoftwarePage(c *ui.Context, a *app.App) {
	t := c.Theme()
	st := softwareState(a)

	if st.Apps == nil && !st.Loading && st.Err == "" {
		st.Scan(func() { a.Win.Invalidate() })
	}

	if st.Loading && len(st.Apps) == 0 {
		ui.Column(c).Fill().Center().Gap(14).Children(func() {
			ui.Spinner(c).Size(28, 28)
			ui.Text(c, "正在扫描已安装应用…").TextColor(t.TextMuted)
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
				st.Scan(func() { a.Win.Invalidate() })
			}
		})
		return
	}

	ui.Column(c).Fill().Children(func() {
		softTopBar(c, a, st, *t)
		ui.Divider(c).BorderColor(t.Border)
		appList(c, a, st, *t)
	})
}

func softwareState(a *app.App) *engine.AppState {
	if a.Software == nil {
		a.Software = engine.NewAppState()
	}
	return a.Software
}

// softTopBar: sub-tabs + summary + search.
func softTopBar(c *ui.Context, a *app.App, st *engine.AppState, t ui.Theme) {
	ui.Row(c).FillWidth().Padding(12, 20, 10, 20).AlignItems(ui.Center).Children(func() {
		// Sub-tabs.
		ui.Row(c).Gap(2).Padding(3).Background(t.Surface).Radius(16).Children(func() {
			labels := []string{"卸载", "更新", "启动项"}
			for i, l := range labels {
				idx := i
				el := ui.Box(c).Padding(5, 14).Radius(12).Children(func() {
					ui.Text(c, l).FontSize(12)
				})
				if st.SubTab == idx {
					el.Background(t.SurfaceHover)
					el.Children(func() { ui.Text(c, l).FontSize(12).Bold() })
				} else {
					el.OnClick(func() { st.SubTab = idx })
				}
			}
		})
		ui.Spacer(c)

		// Summary.
		var total int64
		for _, ap := range st.Apps {
			total += ap.TotalSize()
		}
		ui.Textf(c, "已安装 %d 个 · %s", len(st.Apps), engine.FmtBytes(total)).
			FontSize(12).TextColor(t.TextMuted)
		ui.Spacer(c)

		// Search.
		ui.SearchField(c, &st.Query).Width(200).Placeholder("搜索应用")
	})
}

// appList: table of apps.
func appList(c *ui.Context, a *app.App, st *engine.AppState, t ui.Theme) {
	switch st.SubTab {
	case 0:
		uninstallList(c, a, st, t)
	case 1:
		updatesList(c, a, st, t)
	case 2:
		startupList(c, a, st, t)
	}
}

// uninstallList: the app inventory with sizes and checkboxes.
func uninstallList(c *ui.Context, a *app.App, st *engine.AppState, t ui.Theme) {
	apps := st.FilteredApps()
	cols := []ui.TableColumn{
		{Title: "名称", Width: 0, Sortable: true},
		{Title: "占用体积", Width: 110, Align: ui.End, Sortable: true},
		{Title: "最近使用", Width: 130, Align: ui.End, Sortable: true},
		{Title: "操作", Width: 90, Align: ui.End},
	}
	ui.Table(c, appTableState, cols, len(apps), func(row, col int) {
		if row >= len(apps) {
			return
		}
		ap := apps[row]
		switch col {
		case 0:
			ui.Row(c).Gap(10).Padding(6, 4).AlignItems(ui.Center).Children(func() {
				appIcon(c, ap.Path)
				ui.Column(c).Gap(2).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, ap.Name).FontSize(13).Bold()
						if ap.Running {
							ui.Text(c, "活跃中").FontSize(10).TextColor(t.Accent)
						}
					})
					ui.Row(c).Gap(6).Children(func() {
						if ap.Version != "" {
							ui.Text(c, ap.Version).FontSize(11).TextColor(t.TextMuted)
						}
						if ap.DataSize > 0 {
							ui.Textf(c, "数据 %s", engine.FmtBytes(ap.DataSize)).
								FontSize(11).TextColor(t.TextMuted)
						}
					})
				})
			})
		case 1:
			ui.Text(c, engine.FmtBytes(ap.TotalSize())).FontSize(12)
		case 2:
			ui.Text(c, lastUsedLabel(ap.LastUsed)).FontSize(12).TextColor(t.TextMuted)
		case 3:
			ui.Row(c).Justify(ui.End).Children(func() {
				ui.Button(c, "卸载").OnClick(func() {
					st.SelectApp(row, func() { a.Win.Invalidate() })
					confirmUninstall(c, a, st, ap)
				})
			})
		}
	}).Grow(1).OnChange(func() {
		// Row selection: show remnant pane below.
	})
}

var appTableState = &ui.ListState{}

// appIcon renders the .app's icon via mygo's FileIcon (PNG bytes → Image).
func appIcon(c *ui.Context, appPath string) {
	data, err := mygo.Shell.FileIcon(appPath, 32)
	if err != nil || len(data) == 0 {
		ui.Text(c, "📦").FontSize(20)
		return
	}
	img, err := ui.DecodeImage(data)
	if err != nil {
		ui.Text(c, "📦").FontSize(20)
		return
	}
	ui.Image(c, img).Size(24, 24)
}

func lastUsedLabel(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return "刚刚"
	case d < 24*time.Hour:
		return fmt.Sprintf("%.0f 小时前", d.Hours())
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%.0f 天前", d.Hours()/24)
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%.0f 个月前", d.Hours()/(24*30))
	default:
		return fmt.Sprintf("%.0f 年前", d.Hours()/(24*365))
	}
}

// confirmUninstall shows a modal asking for confirmation with remnant summary.
func confirmUninstall(c *ui.Context, a *app.App, st *engine.AppState, ap *engine.AppInfo) {
	c.AddToast(ui.Toast{
		ID:    "uninstall-" + ap.BundleID,
		Title: "卸载 " + ap.Name + "?",
		Description: fmt.Sprintf("将移入废纸篓并清理 %d 项关联文件（约 %s）",
			len(st.Remnants), remnantSizeStr(st.Remnants)),
		Action: "卸载",
		Type:   "warning",
		OnAction: func() {
			doUninstall(a, st, ap)
		},
		Timeout: -1,
	})
}

func remnantSizeStr(rs []engine.Remnant) string {
	var n int64
	for _, r := range rs {
		n += r.Size
	}
	return engine.FmtBytes(n)
}

// doUninstall trashes the app bundle + selected remnants.
func doUninstall(a *app.App, st *engine.AppState, ap *engine.AppInfo) {
	go func() {
		// App bundle to Trash.
		_ = mygo.Shell.TrashItem(ap.Path)
		for _, r := range st.Remnants {
			if r.Selected && !r.System {
				_ = mygo.Shell.TrashItem(r.Path)
			}
		}
		// Rescan inventory.
		st.Scan(func() { a.Win.Invalidate() })
	}()
}

// updatesList — MVP: brew cask outdated.
func updatesList(c *ui.Context, a *app.App, st *engine.AppState, t ui.Theme) {
	placeholder(c, "🔄", "应用更新", "暂支持 Homebrew Cask 更新检测\n\nmas / Sparkle / Electron 更新将在后续版本加入")
}

// startupList — LaunchAgents + login items.
func startupList(c *ui.Context, a *app.App, st *engine.AppState, t ui.Theme) {
	placeholder(c, "🚀", "启动项", "列出自启动项\n(~/Library/LaunchAgents + 登录项)")
}
