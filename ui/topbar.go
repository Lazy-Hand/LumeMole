package ui

import (
	"github.com/egoist/mygo/ui"

	"lumemole/app"
)

// TopBar renders the Mole-style header: logo left, centered capsule tabs,
// status chip right.
func TopBar(c *ui.Context, a *app.App) {
	t := c.Theme()
	bar := c.TitleBar()

	ui.Row(c).FillWidth().Padding(14, 22, 10, 22).AlignItems(ui.Center).DragWindow().Children(func() {
		// Left: logo (fixed width so tabs stay centered). bar.Left clears
		// the hidden-titlebar traffic lights; it is 0 on other platforms.
		ui.Row(c).Width(200).Gap(8).Padding(0, 0, 0, bar.Left).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "🐹").FontSize(20)
			ui.Text(c, "LumeMole").FontSize(15).Bold()
		})

		// Center: capsule tab control. Interactive children still get
		// their clicks — DragWindow only handles presses on bare areas.
		ui.Row(c).Grow(1).Justify(ui.Center).Children(func() {
			pillTabs(c, a)
		})

		// Right: status chip (fixed width, mirrors the left 200).
		ui.Row(c).Width(200).Justify(ui.End).Children(func() {
			statusChip(c, a, *t)
		})
	})
}

// pillTabs draws the centered capsule tab control, styled after Mole's
// segmented top bar: rounded container, selected tab is a lighter pill.
func pillTabs(c *ui.Context, a *app.App) {
	t := c.Theme()
	tabs := ui.TabsBase(c, (*int)(&a.Tab), len(app.TabNames))
	tabs.List.Gap(2).Padding(3).Background(t.Surface).Radius(22).Children(func() {
		for i, name := range app.TabNames {
			selected := a.Tab == app.Tab(i)
			tab := tabs.Tab(i).Padding(7, 16).Radius(18)
			if selected {
				tab.Background(t.SurfaceHover)
				tab.Children(func() {
					ui.Text(c, name).FontSize(13).Bold()
				})
			} else {
				tab.Children(func() {
					ui.Text(c, name).FontSize(13).TextColor(t.TextMuted)
				})
			}
		}
	})
}

// statusChip shows the small right-side chip (Mole shows "激活 Mole" there;
// we show an FDA warning or a subtle ok dot).
func statusChip(c *ui.Context, a *app.App, t ui.Theme) {
	if a.Permissions != nil && !a.Permissions.FullDiskAccess {
		ui.Row(c).Gap(6).Padding(5, 12).Background(ui.RGBA(230, 140, 60, 0.15)).Radius(14).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(6, 6).Radius(3).Background(ui.Hex("#e08c3c"))
			ui.Text(c, "需要授权").FontSize(12).TextColor(ui.Hex("#e08c3c"))
		})
		return
	}
	ui.Row(c).Gap(6).Padding(5, 12).Background(ui.RGBA(62, 207, 168, 0.12)).Radius(14).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(t.Accent)
		ui.Text(c, "LumeMole").FontSize(12).TextColor(t.Accent)
	})
}
