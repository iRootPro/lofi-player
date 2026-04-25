package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/iRootPro/lofi-player/internal/audio"
	"github.com/iRootPro/lofi-player/internal/config"
	"github.com/iRootPro/lofi-player/internal/theme"
)

// Track is the currently-playing title + artist as reported by mpv's
// metadata stream. Both fields may be empty when no metadata is yet
// available for the active station.
type Track struct {
	Title  string
	Artist string
}

// Model is the root Bubble Tea model.
//
// All Model methods use value receivers — Bubble Tea expects an
// immutable update style and pointer receivers create subtle races
// (plan §4.2).
type Model struct {
	cfg    *config.Config
	player *audio.Player
	theme  theme.Theme
	styles Styles
	keys   KeyMap

	cursor     int
	playingIdx int
	playing    bool
	volume     int

	currentTrack Track
	lastError    string

	width, height int
	showFullHelp  bool
}

// NewModel constructs the root model from a loaded config and an
// initialized audio Player. NewModel does not take ownership of the
// Player — the caller (main) is responsible for Close.
func NewModel(cfg *config.Config, player *audio.Player) Model {
	t, _ := theme.Lookup(cfg.Theme)
	return Model{
		cfg:        cfg,
		player:     player,
		theme:      t,
		styles:     NewStyles(t),
		keys:       DefaultKeyMap(),
		cursor:     0,
		playingIdx: -1,
		playing:    false,
		volume:     clampVolume(cfg.Volume),
	}
}

// Init starts the long-lived event subscription that bridges audio
// events into the Update loop.
func (m Model) Init() tea.Cmd {
	return waitForEvent(m.player)
}

func clampVolume(v int) int {
	switch {
	case v < 0:
		return 0
	case v > 100:
		return 100
	default:
		return v
	}
}
