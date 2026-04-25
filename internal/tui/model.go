package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/iRootPro/lofi-player/internal/audio"
	"github.com/iRootPro/lofi-player/internal/config"
	"github.com/iRootPro/lofi-player/internal/pomodoro"
	"github.com/iRootPro/lofi-player/internal/theme"
)

// Track is the currently-playing title + artist as reported by mpv's
// metadata stream. Both fields may be empty when no metadata is yet
// available for the active station.
type Track struct {
	Title  string
	Artist string
}

// Options groups the optional initial-state knobs NewModel honors. A
// zero value is a valid starting point: theme defaults to cfg.Theme,
// volume to cfg.Volume, no autoplay, fresh pomodoro stats.
type Options struct {
	// Theme overrides cfg.Theme when non-empty (used to apply the
	// state.json snapshot from a previous session).
	Theme string
	// Volume overrides cfg.Volume when > 0.
	Volume int
	// AutoplayStation is the index in cfg.Stations to start playing on
	// startup. -1 (or out-of-range) means no autoplay.
	AutoplayStation int
	// Stats restores the persisted pomodoro stats. Zero value means
	// "fresh slate".
	Stats pomodoro.Stats
}

// viewMode chooses between full, mini, and modal layouts.
type viewMode int

const (
	modeFull viewMode = iota
	modeMini
	modeAddStation
)

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

	// volumeDisplayed is the currently-rendered (animated) volume value;
	// volumeVelocity is the spring's velocity. Both fields are updated
	// on each volTickMsg until the spring settles at volume.
	volumeDisplayed float64
	volumeVelocity  float64
	volumeAnimating bool

	currentTrack Track
	toast        *Toast

	// spinner ticks at ~10 Hz and renders the buffering placeholder
	// inside the now-playing card while ICY/media-title metadata is in
	// flight. The tick keeps running globally; the renderer only uses
	// it when there's nothing else to show.
	spinner spinner.Model

	autoplayURL string

	session        pomodoro.Session
	stats          pomodoro.Stats
	pomoTickActive bool

	width, height int
	showFullHelp  bool
	mode          viewMode

	// modePrev is the layout to restore when a modal (modeAddStation)
	// closes. modeFull during everyday usage; modeMini if the modal
	// was opened from compact mode.
	modePrev viewMode
	addForm  addStationForm
}

// NewModel constructs the root model. NewModel does not take ownership
// of the Player — the caller (main) is responsible for Close.
func NewModel(cfg *config.Config, player *audio.Player, opts Options) Model {
	themeName := cfg.Theme
	if opts.Theme != "" {
		themeName = opts.Theme
	}
	t, _ := theme.Lookup(themeName)

	volume := clampVolume(cfg.Volume)
	if opts.Volume > 0 {
		volume = clampVolume(opts.Volume)
	}

	cursor := 0
	playingIdx := -1
	playing := false
	autoplayURL := ""
	if opts.AutoplayStation >= 0 && opts.AutoplayStation < len(cfg.Stations) {
		cursor = opts.AutoplayStation
		playingIdx = opts.AutoplayStation
		playing = true
		autoplayURL = cfg.Stations[opts.AutoplayStation].URL
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(t.Muted)

	return Model{
		cfg:             cfg,
		player:          player,
		theme:           t,
		styles:          NewStyles(t),
		keys:            DefaultKeyMap(),
		cursor:          cursor,
		playingIdx:      playingIdx,
		playing:         playing,
		volume:          volume,
		volumeDisplayed: float64(volume),
		spinner:         sp,
		autoplayURL:     autoplayURL,
		session:         pomodoro.New(),
		stats:           opts.Stats,
	}
}

// Init starts the long-lived event subscription that bridges audio
// events into the Update loop, plus the buffering spinner's tick loop.
// If the model was constructed with an AutoplayStation, the
// corresponding playCmd is also dispatched.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{waitForEvent(m.player), m.spinner.Tick}
	if m.autoplayURL != "" {
		cmds = append(cmds, playCmd(m.player, m.autoplayURL))
	}
	return tea.Batch(cmds...)
}

// ThemeName returns the active theme name — used by main on shutdown to
// persist the user's selection.
func (m Model) ThemeName() string { return m.theme.Name }

// Volume returns the active volume (0..100) — used by main on shutdown
// to persist the user's selection.
func (m Model) Volume() int { return m.volume }

// LastStationName returns the display name of the currently-playing (or
// most-recently-played) station, or empty string if nothing is playing.
func (m Model) LastStationName() string {
	if m.playingIdx < 0 || m.playingIdx >= len(m.cfg.Stations) {
		return ""
	}
	return m.cfg.Stations[m.playingIdx].Name
}

// Stats returns the current pomodoro stats — used by main on shutdown
// to persist the user's session history.
func (m Model) Stats() pomodoro.Stats { return m.stats }

// pomoConfig returns the runtime pomodoro.Config derived from the YAML
// config's PomodoroConfig (which uses minutes for friendliness).
func (m Model) pomoConfig() pomodoro.Config {
	pc := m.cfg.Pomodoro
	return pomodoro.Config{
		FocusDuration:        time.Duration(pc.FocusMinutes) * time.Minute,
		ShortBreakDuration:   time.Duration(pc.ShortBreakMinutes) * time.Minute,
		LongBreakDuration:    time.Duration(pc.LongBreakMinutes) * time.Minute,
		RoundsUntilLongBreak: pc.RoundsUntilLongBreak,
	}
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
