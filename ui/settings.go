package ui

import (
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lumemole/app"
)

// SettingsView is the gear-popover content, shown from the top bar.
func SettingsView(c *ui.Context, a *app.App) {
	t := c.Theme()
	ui.Column(c).Width(320).Gap(14).Padding(18).Children(func() {
		ui.Text(c, "设置").FontSize(16).Bold()

		settingRow(c, "开机启动", "登录后自动打开 LumeMole", func() ui.Element {
			return ui.Switch(c, &a.Settings.OpenAtLogin).OnClick(func() {
				_ = mygo.App.SetOpenAtLogin(a.Settings.OpenAtLogin)
				a.SaveSettings()
			})
		})

		ui.Divider(c).BorderColor(t.Border)

		settingRow(c, "完全磁盘访问", "清理 ~/Library 与废纸篓必需", func() ui.Element {
			if a.Permissions != nil && a.Permissions.FullDiskAccess {
				return ui.Text(c, "已授权 ✓").FontSize(12).TextColor(t.Accent)
			}
			return ui.Button(c, "去授权").Width(80).OnClick(OpenSystemSettingsFDA)
		})

		ui.Divider(c).BorderColor(t.Border)

		settingRow(c, "白名单", "保护路径不被清理 (与 mole CLI 共享)", func() ui.Element {
			return ui.Button(c, "编辑").Width(80).OnClick(func() {
				_ = mygo.Shell.OpenPath(whitelistPath())
			})
		})

		ui.Divider(c).BorderColor(t.Border)

		ui.Row(c).FillWidth().Children(func() {
			ui.Text(c, "LumeMole v0.1.0").FontSize(11).TextColor(t.TextMuted).Grow(1)
			ui.Button(c, "关于").Width(70).OnClick(func() {
				_ = mygo.Shell.OpenExternal("https://github.com/lazyhand/LumeMole")
			})
		})
	})
}

func settingRow(c *ui.Context, title, desc string, control func() ui.Element) {
	t := c.Theme()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, title).FontSize(13).Bold()
			ui.Text(c, desc).FontSize(11).TextColor(t.TextMuted)
		})
		control()
	})
}

func whitelistPath() string {
	home, _ := os.UserHomeDir()
	return home + "/.config/mole/whitelist"
}
