package tui

import (
	"github.com/charmbracelet/bubbles/key"

	"github.com/iRootPro/lofi-player/internal/i18n"
)

// KeyMap describes the keyboard bindings available in the TUI.
// FullHelp returns the grouped bindings shown in the help card
// (toggled by `?`); short / mini help variants used to live here too
// but were dropped when the inline help bar moved into the frame's
// bottom border.
type KeyMap struct {
	Up            key.Binding
	Down          key.Binding
	PlayPause     key.Binding
	VolUp         key.Binding
	VolDown       key.Binding
	ThemeCycle    key.Binding
	Mini          key.Binding
	AddStation    key.Binding
	Discovery     key.Binding
	EditStation   key.Binding
	DeleteStation key.Binding
	ShareStation  key.Binding
	SavePreview   key.Binding
	ImportStation key.Binding
	MixerOpen     key.Binding
	SettingsOpen  key.Binding
	StreamInfo    key.Binding
	Doctor        key.Binding
	History       key.Binding
	Update        key.Binding
	Help          key.Binding
	Quit          key.Binding
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
			key.WithHelp("t", "themes"),
		),
		Mini: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "mini"),
		),
		AddStation: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add"),
		),
		Discovery: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "discover"),
		),
		EditStation: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit"),
		),
		DeleteStation: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		ShareStation: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "share"),
		),
		SavePreview: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "save preview"),
		),
		ImportStation: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "paste"),
		),
		MixerOpen: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "mixer"),
		),
		SettingsOpen: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "settings"),
		),
		StreamInfo: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "info"),
		),
		Doctor: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "doctor"),
		),
		History: key.NewBinding(
			key.WithKeys("H"),
			key.WithHelp("H", "history"),
		),
		Update: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "update"),
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

// KeyMapFor keeps keyboard bindings language-neutral while translating the
// descriptions shown in the full help card.
func KeyMapFor(locale i18n.Locale) KeyMap {
	k := DefaultKeyMap()
	for _, item := range []struct {
		binding *key.Binding
		desc    string
	}{
		{&k.Up, "select"}, {&k.Down, "select"}, {&k.PlayPause, "play_pause"}, {&k.VolUp, "volume"}, {&k.VolDown, "volume"},
		{&k.ThemeCycle, "themes"}, {&k.Mini, "app"}, {&k.AddStation, "add_station"}, {&k.Discovery, "discover"}, {&k.EditStation, "edit_station"},
		{&k.DeleteStation, "delete"}, {&k.ShareStation, "share_station"}, {&k.SavePreview, "save_preview"}, {&k.ImportStation, "import"}, {&k.MixerOpen, "mixer"},
		{&k.SettingsOpen, "settings"}, {&k.StreamInfo, "details"}, {&k.Doctor, "doctor_report"}, {&k.History, "history"}, {&k.Update, "update_title"}, {&k.Help, "help"}, {&k.Quit, "close"},
	} {
		h := item.binding.Help()
		item.binding.SetHelp(h.Key, i18n.T(locale, item.desc))
	}
	return k
}

// FullHelp returns the bindings grouped by category for the full help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.AddStation, k.Discovery, k.EditStation, k.DeleteStation, k.ShareStation, k.SavePreview, k.ImportStation, k.MixerOpen},
		{k.PlayPause, k.VolUp, k.VolDown},
		{k.ThemeCycle, k.SettingsOpen, k.Mini, k.StreamInfo, k.Doctor, k.History, k.Update, k.Help, k.Quit},
	}
}
