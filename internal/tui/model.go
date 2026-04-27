package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

// Options groups the optional initial-state knobs NewModel honors. A
// zero value is a valid starting point: theme defaults to cfg.Theme,
// volume to cfg.Volume, no autoplay.
type Options struct {
	// Theme overrides cfg.Theme when non-empty (used to apply the
	// state.json snapshot from a previous session).
	Theme string
	// Volume overrides cfg.Volume when > 0.
	Volume int
	// AutoplayStation is the index in cfg.Stations to start playing on
	// startup. -1 (or out-of-range) means no autoplay.
	AutoplayStation int
	// SaveAmbient is called with a channel-volume snapshot after the
	// 500ms debounce quiets. Persistence behavior is the caller's
	// responsibility; a returned error surfaces as an error toast,
	// matching the AddStation save-failure pattern.
	SaveAmbient func(map[string]int) error
	// ShowStreamInfo overrides the default visibility of the
	// stream-info row under the now-playing card. nil = use default
	// (true). Pointer rather than bool so the caller can express
	// "user has explicitly hidden it" distinctly from "unset".
	ShowStreamInfo *bool
}

// viewMode chooses between full, mini, and modal layouts.
type viewMode int

const (
	modeFull viewMode = iota
	modeMini
	modeAddStation
	modeMixer
	modeConfirmDelete
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
	// loading is true between dispatching playCmd (or autoplay on
	// startup) and the first PlaybackStarted event from mpv. While
	// loading is true, the status indicator renders the spinner
	// instead of the ●/◯ glyph.
	loading bool
	volume  int

	currentTrack Track
	toast        *Toast

	// spinner ticks at ~10 Hz and renders the buffering indicator in the
	// status slot while mpv is loading a stream. The tick keeps running
	// globally; the renderer only consults it when loading is true.
	spinner spinner.Model

	// pulseDim alternates every pulseInterval to give the live ●
	// indicator a soft heartbeat while a station is playing. The renderer
	// only consults it when playing && !loading; the tick runs globally
	// and is cheap.
	pulseDim bool

	// logo drives the shimmer that runs across the ASCII logo
	// rendered next to the now-playing card. The tick advances
	// only while playing, so the shimmer freezes on pause.
	logo logo

	autoplayURL string

	width, height int
	showFullHelp  bool
	mode          viewMode

	// modePrev is the layout to restore when a modal (modeAddStation,
	// modeMixer, modeConfirmDelete) closes. modeFull during everyday
	// usage; modeMini if the modal was opened from compact mode.
	modePrev viewMode
	addForm  addStationForm

	// pendingDeleteIdx points at the station awaiting user confirmation
	// in modeConfirmDelete. -1 outside that mode.
	pendingDeleteIdx int

	mixer   *audio.AmbientMixer
	mixerUI mixerModel

	// ambientSaveSeq is bumped on every mixer keypress; the matching
	// ambientSaveTickMsg only fires the save callback when its seq
	// still equals this value (debounce coalescing).
	ambientSaveSeq int
	saveAmbient    func(map[string]int) error

	// streamInfo is the latest technical info reported by mpv for the
	// current station (bitrate, codec, sample rate, channels). All
	// fields zero when nothing is playing or before mpv reports.
	streamInfo audio.StreamInfoChanged
	// cacheSeconds is how much audio mpv has buffered ahead. Drives
	// the buffer-health indicator under the now-playing card.
	cacheSeconds float64
	// playStartedAt is when the current session started (i.e. last
	// successful Play call). Drives the "listening 1h 23m" uptime
	// label. Zero when nothing is playing.
	playStartedAt time.Time
	// nowTime is the wall clock as of the last clockTickMsg, used to
	// derive uptime from playStartedAt without re-rendering once a
	// second pulling time.Now() at render time.
	nowTime time.Time

	// showStreamInfo toggles visibility of the technical info row
	// (bitrate / codec / sample / uptime / buffer-bar). Default true;
	// the user toggles with `i` and the choice persists via state.
	showStreamInfo bool
}

// NewModel constructs the root model. NewModel does not take ownership
// of the Player or AmbientMixer — the caller (main) is responsible for
// closing them.
func NewModel(cfg *config.Config, player *audio.Player, mixer *audio.AmbientMixer, opts Options) Model {
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
	loading := false
	autoplayURL := ""
	if opts.AutoplayStation >= 0 && opts.AutoplayStation < len(cfg.Stations) {
		cursor = opts.AutoplayStation
		playingIdx = opts.AutoplayStation
		playing = true
		loading = true
		autoplayURL = cfg.Stations[opts.AutoplayStation].URL
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(t.Muted)

	showStreamInfo := true
	if opts.ShowStreamInfo != nil {
		showStreamInfo = *opts.ShowStreamInfo
	}

	return Model{
		cfg:            cfg,
		player:         player,
		theme:          t,
		styles:         NewStyles(t),
		keys:           DefaultKeyMap(),
		cursor:         cursor,
		playingIdx:     playingIdx,
		playing:        playing,
		loading:        loading,
		volume:         volume,
		spinner:        sp,
		autoplayURL:    autoplayURL,
		mixer:            mixer,
		mixerUI:          newMixerModel(mixer),
		saveAmbient:      opts.SaveAmbient,
		showStreamInfo:   showStreamInfo,
		pendingDeleteIdx: -1,
	}
}

// Init starts the long-lived event subscription that bridges audio
// events into the Update loop, plus the buffering spinner and live-
// indicator pulse tick loops. If the model was constructed with an
// AutoplayStation, the corresponding playCmd is also dispatched.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{waitForEvent(m.player), m.spinner.Tick, pulseTick(), logoTick(), clockTick()}
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

// ShowStreamInfo returns whether the stream-info row is currently
// visible — used by main on shutdown to persist the toggle across
// sessions.
func (m Model) ShowStreamInfo() bool { return m.showStreamInfo }

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
