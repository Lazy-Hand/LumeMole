package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"lumemole/app"
	moleui "lumemole/ui"
)

func TestAllPagesRender(t *testing.T) {
	a := &app.App{Permissions: &app.Permissions{FullDiskAccess: true}}
	for _, tab := range []app.Tab{app.TabClean, app.TabSoftware, app.TabOptimize, app.TabAnalyze, app.TabStatus} {
		a.Tab = tab
		tt := ui.NewTester(func(c *ui.Context) { moleui.Shell(c, a) }, 1280, 860)
		if !tt.HasText("LumeMole") {
			t.Fatalf("tab %d missing app title; texts %q", tab, tt.Texts())
		}
	}
}
