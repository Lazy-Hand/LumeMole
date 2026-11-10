package ui

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lumemole/app"
)

// Shell renders the whole window: top bar + active page.
// Called from the root view each frame.
func Shell(c *ui.Context, a *app.App) {
	c.SetTheme(app.LumeTheme())
	t := c.Theme()
	ui.Column(c).Fill().Children(func() {
		TopBar(c, a)
		ui.Divider(c).BorderColor(t.Border)
		// Page area fills remaining space.
		ui.Box(c).Grow(1).Children(func() {
			switch {
			case a.Permissions != nil && !a.Permissions.FullDiskAccess:
				Onboarding(c, a)
			default:
				switch a.Tab {
				case app.TabStatus:
					StatusPage(c, a)
				case app.TabAnalyze:
					AnalyzePage(c, a)
				case app.TabClean:
					CleanPage(c, a)
				case app.TabSoftware:
					SoftwarePage(c, a)
				case app.TabOptimize:
					OptimizePage(c, a)
				}
			}
		})
	})
}

// placeholder is the "coming soon" body for pages not yet implemented.
func placeholder(c *ui.Context, icon, title, sub string) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Gap(14).Children(func() {
		ui.Text(c, icon).FontSize(52)
		ui.Text(c, title).FontSize(22).Bold()
		ui.Text(c, sub).FontSize(13).TextColor(t.TextMuted)
	})
}

// OpenSystemSettingsFDA opens System Settings on the Full Disk Access pane.
func OpenSystemSettingsFDA() {
	_ = mygo.Shell.OpenExternal(
		"x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles")
}
