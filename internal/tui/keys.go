package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap describes the keyboard bindings available in the TUI.
// FullHelp returns the grouped bindings shown in the help card
// (toggled by `?`); short / mini help variants used to live here too
// but were dropped when the inline help bar moved into the frame's
// bottom border.
type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	PlayPause  key.Binding
	VolUp      key.Binding
	VolDown    key.Binding
	ThemeCycle key.Binding
	Mini       key.Binding
	AddStation key.Binding
	MixerOpen  key.Binding
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
			key.WithHelp("space", "play"),
		),
		VolUp: key.NewBinding(
			key.WithKeys("+", "="),
			key.WithHelp("+", "vol+"),
		),
		VolDown: key.NewBinding(
			key.WithKeys("-", "_"),
			key.WithHelp("-", "vol-"),
		),
		ThemeCycle: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "theme"),
		),
		Mini: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "mini"),
		),
		AddStation: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add"),
		),
		MixerOpen: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "mixer"),
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

// FullHelp returns the bindings grouped by category for the full help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.AddStation, k.MixerOpen},
		{k.PlayPause, k.VolUp, k.VolDown},
		{k.ThemeCycle, k.Mini},
		{k.Help, k.Quit},
	}
}
