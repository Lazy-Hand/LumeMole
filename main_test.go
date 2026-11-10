package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"lumemole/app"
	moleui "lumemole/ui"
)

// The shell builds without a window: tabs switch state.
func TestShellTabs(t *testing.T) {
	a := &app.App{Permissions: &app.Permissions{FullDiskAccess: true}}
	tt := ui.NewTester(func(c *ui.Context) { moleui.Shell(c, a) }, 1280, 860)
	if !tt.HasText("LumeMole") {
		t.Fatalf("missing app title; texts %q", tt.Texts())
	}
	// All five tabs present.
	for _, name := range app.TabNames {
		if !tt.HasText(name) {
			t.Fatalf("missing tab %q; texts %q", name, tt.Texts())
		}
	}
	// Click 状态 tab → placeholder shows.
	if err := tt.Click("状态"); err != nil {
		t.Fatal(err)
	}
	if a.Tab != app.TabStatus {
		t.Fatalf("tab = %v, want TabStatus", a.Tab)
	}
}

// Without FDA the onboarding page shows instead of pages.
func TestOnboardingShownWhenNoFDA(t *testing.T) {
	a := &app.App{Permissions: &app.Permissions{FullDiskAccess: false}}
	tt := ui.NewTester(func(c *ui.Context) { moleui.Shell(c, a) }, 1280, 860)
	if !tt.HasText("需要完全磁盘访问权限") {
		t.Fatalf("missing onboarding; texts %q", tt.Texts())
	}
}
