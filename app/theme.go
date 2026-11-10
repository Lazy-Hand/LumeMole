package app

import "github.com/egoist/mygo/ui"

// LumeTheme returns the dark theme tuned to Mole's look:
// near-black background with a cool cast, teal accent.
func LumeTheme() *ui.Theme {
	t := *ui.DarkTheme()
	t.Background = ui.Hex("#16121f")
	t.Surface = ui.Hex("#242031")
	t.SurfaceHover = ui.Hex("#2e2940")
	t.SurfacePressed = ui.Hex("#383249")
	t.Border = ui.Hex("#3a3450")
	t.Text = ui.Hex("#e8e4f0")
	t.TextMuted = ui.Hex("#8d86a0")
	t.Accent = ui.Hex("#3ecfa8")
	t.AccentHover = ui.Hex("#4adcb5")
	t.AccentPressed = ui.Hex("#34b795")
	t.AccentText = ui.Hex("#0d1f1a")
	t.InactiveAccent = t.Accent
	t.Selection = ui.RGBA(62, 207, 168, 0.3)
	return &t
}
