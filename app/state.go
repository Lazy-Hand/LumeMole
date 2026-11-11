package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/egoist/mygo"

	"lumemole/engine"
)

// Tab identifies the five main pages, in top-bar order.
type Tab int

const (
	TabClean Tab = iota
	TabSoftware
	TabOptimize
	TabAnalyze
	TabStatus
	TabDoctor
)

// TabNames are the top-bar labels.
var TabNames = []string{"清理", "软件", "优化", "分析", "状态", "诊断"}

// Settings persists user preferences to ~/.config/lumemole/config.json.
type Settings struct {
	OpenAtLogin bool `json:"open_at_login"`
}

// App is the root application state. All fields are read and written on the
// main thread (view builds, callbacks, win.Update closures); background
// goroutines funnel changes through win.Update, so plain fields suffice.
// The mutex guards only data touched by real background writers.
type App struct {
	Win *mygo.Window
	// PanelWin is the tray popover window, set when the flyout opens.
	PanelWin   *mygo.Window
	KeepAwakeOn bool

	Tab      Tab
	Settings Settings

	// Permissions is the TCC probe result, filled at startup.
	Permissions *Permissions

	// Status is the Phase 1 status page state (watch goroutine + history).
	Status *engine.StatusState
	// Analyze is the Phase 2 disk-analysis state.
	Analyze *engine.AnalyzeState
	// Clean is the Phase 3 clean-page state.
	Clean *engine.CleanState
	// Software is the Phase 4 software-management state.
	Software *engine.AppState
	// Optimize is the Phase 5 maintenance state.
	Optimize *engine.OptimizeState
	// AI is the AI-tools cleanup state (a sub-view of the clean page).
	AI *engine.AIState
	// ShowAI switches the clean page to the AI-tools view.
	ShowAI bool
	// Doctor is the system-diagnostic state.
	Doctor *engine.DoctorState

	mu sync.Mutex
}

// NewApp loads settings and probes permissions.
func NewApp() *App {
	a := &App{Settings: Settings{OpenAtLogin: false}}
	a.loadSettings()
	a.Permissions = ProbePermissions()
	return a
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "lumemole")
}

func configPath() string { return filepath.Join(configDir(), "config.json") }

func (a *App) loadSettings() {
	data, err := os.ReadFile(configPath())
	if err == nil {
		_ = json.Unmarshal(data, &a.Settings)
	}
}

// SaveSettings persists settings to disk.
func (a *App) SaveSettings() {
	_ = os.MkdirAll(configDir(), 0o755)
	data, _ := json.MarshalIndent(a.Settings, "", "  ")
	_ = os.WriteFile(configPath(), data, 0o644)
}
