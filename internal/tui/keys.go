package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap describes the keyboard bindings available in the TUI.
//
// It satisfies the help.KeyMap interface (ShortHelp + FullHelp) so that
// the help renderer can read bindings without knowing about Model.
type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	PlayPause  key.Binding
	VolUp      key.Binding
	VolDown    key.Binding
	ThemeCycle key.Binding
	Mini       key.Binding
	Pomodoro   key.Binding
	AddStation key.Binding
	Help       key.Binding
	Quit       key.Binding
}

// DefaultKeyMap returns the Phase 0 keybindings from project plan §6.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/↑", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/↓", "down"),
		),
		// The literal " " is required — Bubble Tea reports the spacebar
		// as a single space, not the string "space" (plan §6 pitfall).
		PlayPause: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "play/pause"),
		),
		VolUp: key.NewBinding(
			key.WithKeys("+", "="),
			key.WithHelp("+", "vol up"),
		),
		VolDown: key.NewBinding(
			key.WithKeys("-", "_"),
			key.WithHelp("-", "vol down"),
		),
		ThemeCycle: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "theme"),
		),
		Mini: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "mini"),
		),
		Pomodoro: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "pomodoro"),
		),
		AddStation: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add station"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

// ShortHelp returns the bindings shown in the compact help bar.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.PlayPause, k.Down, k.VolUp, k.AddStation, k.Pomodoro, k.ThemeCycle, k.Mini, k.Help, k.Quit}
}

// FullHelp returns the bindings grouped by category for the full help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.AddStation},
		{k.PlayPause, k.VolUp, k.VolDown},
		{k.Pomodoro, k.ThemeCycle, k.Mini},
		{k.Help, k.Quit},
	}
}

// MiniShortHelp returns the smaller binding set shown at the bottom of
// the mini-mode view: just the basics that fit on one line.
func (k KeyMap) MiniShortHelp() []key.Binding {
	return []key.Binding{k.PlayPause, k.Mini, k.Quit}
}
