package main

import (
	"log"

	"github.com/egoist/mygo"
	upd "github.com/egoist/mygo/plugins/updater"
	updaterui "github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"

	"lumemole/app"
	moleui "lumemole/ui"
)

func main() {
	a := app.NewApp()

	// Sparkle-style update window: background daily check + "检查更新…"
	// menu item. Dev builds skip; the plugin no-ops when the build can't
	// update itself.
	mygo.Use(updaterui.Plugin)

	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Label: "LumeMole", Submenu: []*mygo.MenuItem{
				{Role: mygo.RoleAbout},
				upd.MenuItem(),
				mygo.Separator(),
				{Role: mygo.RoleServices},
				mygo.Separator(),
				{Role: mygo.RoleHide},
				{Role: mygo.RoleHideOthers},
				{Role: mygo.RoleUnhide},
				mygo.Separator(),
				{Role: mygo.RoleQuit},
			}},
			{Role: mygo.RoleEditMenu},
			{Role: mygo.RoleWindowMenu},
		}))
		w := mygo.NewWindow(mygo.WindowOptions{
			Title:         "LumeMole",
			Width:         1280,
			Height:        860,
			MinWidth:      980,
			MinHeight:     640,
			TitleBarStyle: mygo.TitleBarHidden,
			Vibrancy:      mygo.VibrancyWindow,
			StateKey:      "main",
			Content:       ui.View(func(c *ui.Context) { moleui.Shell(c, a) }),
		})
		a.Win = w
		setupMenuBar(a)
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
