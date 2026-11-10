package ui

import (
	"github.com/egoist/mygo/ui"

	"lumemole/app"
)

// Onboarding shows when Full Disk Access is not granted.
func Onboarding(c *ui.Context, a *app.App) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Gap(18).Padding(40).Children(func() {
		ui.Text(c, "🔐").FontSize(56)
		ui.Text(c, "需要完全磁盘访问权限").FontSize(22).Bold()
		ui.Text(c, "LumeMole 需要读取 ~/Library、废纸篓等受保护目录才能扫描和清理。\n请在系统设置中为 LumeMole 开启「完全磁盘访问权限」，然后点击下方按钮重试。").
			FontSize(13).TextColor(t.TextMuted).TextAlign(ui.Center)

		ui.Row(c).Gap(12).Children(func() {
			if ui.PrimaryButton(c, "打开系统设置").Clicked() {
				OpenSystemSettingsFDA()
			}
			if ui.Button(c, "我已授权，重试").Clicked() {
				a.Permissions = app.ProbePermissions()
			}
		})

		ui.Text(c, "开发构建使用 ad-hoc 签名，每次重新编译后可能需要重新授权").FontSize(11).TextColor(t.TextMuted)
	})
}
