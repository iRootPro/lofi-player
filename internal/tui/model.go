package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/iRootPro/lofi-player/internal/config"
	"github.com/iRootPro/lofi-player/internal/theme"
)

// Model is the root Bubble Tea model.
//
// All Model methods use value receivers — Bubble Tea expects an
// immutable update style and pointer receivers create subtle races
// (plan §4.2).
type Model struct {
	cfg    *config.Config
	theme  theme.Theme
	styles Styles
	keys   KeyMap

	cursor     int
	playingIdx int
	playing    bool
	volume     int

	width, height int
	showFullHelp  bool
}

// NewModel constructs the root model from a loaded config. If cfg.Theme
// names an unknown theme, Tokyo Night is used.
func NewModel(cfg *config.Config) Model {
	t, _ := theme.Lookup(cfg.Theme)
	return Model{
		cfg:        cfg,
		theme:      t,
		styles:     NewStyles(t),
		keys:       DefaultKeyMap(),
		cursor:     0,
		playingIdx: -1,
		playing:    false,
		volume:     clampVolume(cfg.Volume),
	}
}

// Init returns the initial command. Phase 0 has no asynchronous work
// to start, so this is a no-op.
func (m Model) Init() tea.Cmd {
	return nil
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
