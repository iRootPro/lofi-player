package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/iRootPro/lofi-player/internal/audio"
	"github.com/iRootPro/lofi-player/internal/config"
	sharepkg "github.com/iRootPro/lofi-player/internal/share"
	"github.com/iRootPro/lofi-player/internal/theme"
)

const (
	volumeStep                = 5
	settingsMaxBufferSeconds  = 600
	settingsMaxInitialSeconds = 10
)

// Update applies a message to the model and returns the new model plus
// any commands to run. Receiver is by value; never mutate m through a
// pointer (plan §4.2).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Out-of-band tea.Tick chains must be handled before the modal
	// dispatch — the modal updaters only know about tea.KeyMsg and
	// would silently drop the tick, killing the chain forever.
	switch tick := msg.(type) {
	case ambientSaveTickMsg:
		if tick.seq == m.ambientSaveSeq && m.saveAmbient != nil && m.mixer != nil {
			if err := m.saveAmbient(m.mixer.Volumes()); err != nil {
				m.toast = &Toast{
					Message: "Could not save ambient mixer settings.",
					Kind:    ToastError,
				}
				return m, clearToastAfter()
			}
		}
		return m, nil
	case pulseTickMsg:
		m.pulseDim = !m.pulseDim
		return m, pulseTick()
	case logoTickMsg:
		if m.playing {
			m.logo.advance()
		}
		return m, logoTick()
	case clockTickMsg:
		m.nowTime = tick.At
		return m, clockTick()
	case reconnectStreamMsg:
		station, active := m.activeStation()
		if tick.seq != m.reconnectSeq || !m.bufferingStalled || !m.playing || !active {
			return m, nil
		}
		m.loading = true
		requestID := m.nextPlayRequestID()
		m.currentTrack = Track{}
		m.streamInfo = audio.StreamInfoChanged{}
		m.cacheSeconds = 0
		m.playStartedAt = time.Time{}
		m.toast = &Toast{Message: "reconnecting stream…", Kind: ToastInfo}
		return m, tea.Batch(playCmd(m.player, station.URL, requestID), clearToastAfter())
	case discoverySearchTickMsg:
		if tick.session != m.discovery.session || tick.seq != m.discovery.searchSeq || tick.query != m.discovery.input.Value() || m.mode != modeDiscovery {
			return m, nil
		}
		if m.discoveryClient == nil {
			m.discovery.err = "Radio Browser is unavailable"
			return m, nil
		}
		m.discovery.loading = true
		m.discovery.err = ""
		return m, discoverySearchCmd(m.discoveryClient, tick.session, tick.seq, tick.query)
	case discoverySearchResultMsg:
		if tick.session != m.discovery.session || tick.seq != m.discovery.searchSeq || m.mode != modeDiscovery {
			return m, nil
		}
		m.discovery.loading = false
		if tick.err != nil {
			m.discovery.results = nil
			m.discovery.cursor = 0
			m.discovery.cached = false
			m.discovery.err = discoveryErrorMessage(tick.err)
			return m, nil
		}
		m.discovery.results = tick.results.Stations
		m.discovery.cursor = 0
		m.discovery.cached = tick.results.Cached
		m.discovery.err = ""
		return m, nil
	case clearToastMsg:
		m.toast = nil
		return m, nil
	case shareCopiedMsg:
		if tick.Err != nil {
			// Keep the share modal open so the visible YAML remains available
			// for manual copying when the host clipboard is unavailable.
			m.toast = &Toast{Message: "Clipboard is unavailable. Copy the station text manually.", Kind: ToastError}
			return m, clearToastAfter()
		}
		m.mode = m.modePrev
		m.shareSnippet = ""
		m.toast = &Toast{Message: "station snippet copied", Kind: ToastSuccess}
		return m, clearToastAfter()
	case importClipboardMsg:
		if tick.Err != nil {
			m.toast = &Toast{Message: "Could not import stations. Check the copied station data.", Kind: ToastError}
			return m, clearToastAfter()
		}
		m.importStations, m.importSkipped = m.newStationsOnly(tick.Stations)
		if len(m.importStations) == 0 {
			m.toast = &Toast{Message: "all pasted stations already exist", Kind: ToastInfo}
			return m, clearToastAfter()
		}
		m.modePrev = m.mode
		m.mode = modeImportStations
		return m, nil
	}
	// Modal states intercept ordinary UI input first so forms can capture text
	// without the global keymap stealing characters like 'q'. Audio events are
	// deliberately exempt: a preview may start while discovery is open, and the
	// event subscription must stay alive rather than being swallowed by a form.
	if !isAudioEventMsg(msg) && m.mode == modeAddStation {
		return m.updateAddStation(msg)
	}
	if !isAudioEventMsg(msg) && m.mode == modeMixer {
		return m.updateMixer(msg)
	}
	if !isAudioEventMsg(msg) && m.mode == modeConfirmDelete {
		return m.updateConfirmDelete(msg)
	}
	if !isAudioEventMsg(msg) && m.mode == modeShareStation {
		return m.updateShareStation(msg)
	}
	if !isAudioEventMsg(msg) && m.mode == modeImportStations {
		return m.updateImportStations(msg)
	}
	if !isAudioEventMsg(msg) && m.mode == modeThemePicker {
		return m.updateThemePicker(msg)
	}
	if !isAudioEventMsg(msg) && m.mode == modeSettings {
		return m.updateSettings(msg)
	}
	if !isAudioEventMsg(msg) && !isGlobalModelMsg(msg) && m.mode == modeDiscovery {
		return m.updateDiscovery(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case MetadataChangedMsg:
		if msg.RequestID != m.playRequestID {
			return m, waitForEvent(m.player)
		}
		m.currentTrack = Track{Title: msg.Title, Artist: msg.Artist}
		return m, waitForEvent(m.player)

	case PlaybackStartedMsg:
		if msg.RequestID != m.playRequestID {
			return m, waitForEvent(m.player)
		}
		// mpv can emit pause=false while idle (for example if a hardware
		// media key is pressed before any station has been loaded). Do not
		// let that phantom event make the TUI claim something is playing.
		if !m.hasActiveStation() {
			m.playing = false
			m.loading = false
			m.bufferingStalled = false
			m.reconnectSeq++
			return m, waitForEvent(m.player)
		}
		m.playing = true
		m.loading = false
		m.bufferingStalled = false
		m.reconnectSeq++
		// First start after a fresh Play call: anchor the uptime
		// counter. Resume after pause keeps the previous anchor so
		// "listening 1h 23m" doesn't reset every time the user toggles.
		if m.playStartedAt.IsZero() {
			m.playStartedAt = time.Now()
		}
		return m, waitForEvent(m.player)

	case PlaybackLoadStartedMsg:
		if msg.RequestID != m.playRequestID {
			return m, waitForEvent(m.player)
		}
		return m, waitForEvent(m.player)

	case PlaybackPausedMsg:
		if msg.RequestID != m.playRequestID {
			return m, waitForEvent(m.player)
		}
		m.playing = false
		m.loading = false
		m.bufferingStalled = false
		m.reconnectSeq++
		return m, waitForEvent(m.player)

	case PlaybackErrorMsg:
		if msg.RequestID != m.playRequestID {
			return m, waitForEvent(m.player)
		}
		m.toast = &Toast{Message: "Could not play this station. Try another one.", Kind: ToastError}
		m.playing = false
		m.loading = false
		m.playingIdx = -1
		m.previewStation = nil
		m.previewUUID = ""
		m.currentTrack = Track{}
		m.streamInfo = audio.StreamInfoChanged{}
		m.cacheSeconds = 0
		m.bufferingStalled = false
		m.reconnectSeq++
		m.playStartedAt = time.Time{}
		return m, tea.Batch(clearToastAfter(), waitForEvent(m.player))

	case EOFMsg:
		if msg.RequestID != m.playRequestID {
			return m, waitForEvent(m.player)
		}
		m.playing = false
		m.loading = false
		m.playingIdx = -1
		m.previewStation = nil
		m.previewUUID = ""
		m.currentTrack = Track{}
		m.streamInfo = audio.StreamInfoChanged{}
		m.cacheSeconds = 0
		m.bufferingStalled = false
		m.reconnectSeq++
		m.playStartedAt = time.Time{}
		return m, waitForEvent(m.player)

	case StreamInfoChangedMsg:
		if msg.RequestID != m.playRequestID {
			return m, waitForEvent(m.player)
		}
		m.streamInfo = audio.StreamInfoChanged{
			Bitrate:    msg.Bitrate,
			Codec:      msg.Codec,
			SampleRate: msg.SampleRate,
			Channels:   msg.Channels,
		}
		return m, waitForEvent(m.player)

	case CacheStateChangedMsg:
		if msg.RequestID != m.playRequestID {
			return m, waitForEvent(m.player)
		}
		m.cacheSeconds = msg.Seconds
		return m, waitForEvent(m.player)

	case BufferingChangedMsg:
		if msg.RequestID != m.playRequestID {
			return m, waitForEvent(m.player)
		}
		m.bufferingStalled = msg.Stalled
		if !msg.Stalled {
			m.reconnectSeq++
			return m, waitForEvent(m.player)
		}
		// paused-for-cache also happens during initial buffering. Only arm
		// the self-heal watchdog after a station was already playing; slow
		// startup should keep buffering instead of reload-looping.
		if m.playing && !m.loading && m.hasActiveStation() {
			m.reconnectSeq++
			seq := m.reconnectSeq
			m.toast = &Toast{Message: "stream stalled — reconnecting if it does not recover", Kind: ToastInfo}
			return m, tea.Batch(waitForEvent(m.player), reconnectStreamAfter(seq), clearToastAfter())
		}
		return m, waitForEvent(m.player)

	case CommandFailedMsg:
		// Transient IPC failure (mpv was busy, request timed out) —
		// not a stream death. Toast and move on; do NOT touch
		// playingIdx / streamInfo / playStartedAt the way
		// PlaybackErrorMsg does.
		m.toast = &Toast{
			Message: "Could not " + msg.Action + ". Try again.",
			Kind:    ToastError,
		}
		return m, clearToastAfter()

	case clearToastMsg:
		m.toast = nil
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case pulseTickMsg:
		m.pulseDim = !m.pulseDim
		return m, pulseTick()

	case logoTickMsg:
		// Re-arm unconditionally so the chain stays alive across
		// pause/resume; only advance the shimmer while playing so
		// it freezes (rather than crawls) when paused.
		if m.playing {
			m.logo.advance()
		}
		return m, logoTick()
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if m.cursor < len(m.cfg.Stations)-1 {
			m.cursor++
		}
		return m, nil

	case key.Matches(msg, m.keys.PlayPause):
		return m.togglePlayPause()

	case key.Matches(msg, m.keys.VolUp):
		m.volume = clampVolume(m.volume + volumeStep)
		return m, setVolumeCmd(m.player, m.volume)

	case key.Matches(msg, m.keys.VolDown):
		m.volume = clampVolume(m.volume - volumeStep)
		return m, setVolumeCmd(m.player, m.volume)

	case key.Matches(msg, m.keys.ThemeCycle):
		return m.openThemePicker(), nil

	case key.Matches(msg, m.keys.Mini):
		if m.mode == modeFull {
			m.mode = modeMini
		} else {
			m.mode = modeFull
		}
		return m, nil

	case key.Matches(msg, m.keys.AddStation):
		m.modePrev = m.mode
		m.mode = modeAddStation
		m.addForm = newAddStationForm()
		// Tell bubbles/textinput to start its cursor blink.
		return m, m.addForm.name.Cursor.BlinkCmd()

	case key.Matches(msg, m.keys.Discovery):
		m.modePrev = m.mode
		m.mode = modeDiscovery
		m.discoverySession++
		m.discovery = newDiscoveryModel(m.discoverySession)
		return m, m.discovery.input.Cursor.BlinkCmd()

	case key.Matches(msg, m.keys.EditStation):
		if m.cursor < 0 || m.cursor >= len(m.cfg.Stations) {
			return m, nil
		}
		m.modePrev = m.mode
		m.mode = modeAddStation
		m.addForm = newEditStationForm(m.cursor, m.cfg.Stations[m.cursor])
		return m, m.addForm.name.Cursor.BlinkCmd()

	case key.Matches(msg, m.keys.DeleteStation):
		if m.cursor < 0 || m.cursor >= len(m.cfg.Stations) {
			return m, nil
		}
		m.modePrev = m.mode
		m.mode = modeConfirmDelete
		m.pendingDeleteIdx = m.cursor
		return m, nil

	case key.Matches(msg, m.keys.ShareStation):
		if m.previewStation != nil {
			return m.savePlayingDiscoveryStation()
		}
		if m.cursor < 0 || m.cursor >= len(m.cfg.Stations) {
			return m, nil
		}
		snippet, err := sharepkg.MarshalStation(m.cfg.Stations[m.cursor])
		if err != nil {
			m.toast = &Toast{Message: "Could not prepare this station for sharing.", Kind: ToastError}
			return m, clearToastAfter()
		}
		m.modePrev = m.mode
		m.mode = modeShareStation
		m.shareSnippet = snippet
		return m, nil

	case key.Matches(msg, m.keys.ImportStation):
		return m, importClipboardCmd()

	case key.Matches(msg, m.keys.MixerOpen):
		m.modePrev = m.mode
		m.mode = modeMixer
		return m, nil

	case key.Matches(msg, m.keys.SettingsOpen):
		return m.openSettings(), nil

	case key.Matches(msg, m.keys.StreamInfo):
		m.showStreamInfo = !m.showStreamInfo
		return m, nil

	case key.Matches(msg, m.keys.Help):
		m.showFullHelp = !m.showFullHelp
		return m, nil
	}
	return m, nil
}

func isAudioEventMsg(msg tea.Msg) bool {
	switch msg.(type) {
	case MetadataChangedMsg, PlaybackLoadStartedMsg, PlaybackStartedMsg, PlaybackPausedMsg,
		PlaybackErrorMsg, EOFMsg, StreamInfoChangedMsg, CacheStateChangedMsg,
		BufferingChangedMsg, CommandFailedMsg:
		return true
	default:
		return false
	}
}

// isGlobalModelMsg identifies framework messages that must reach the root
// update loop even while a modal owns keyboard input. In particular, spinner
// ticks keep the discovery Play feedback animated and schedule the next tick;
// window-size updates keep the overlay aligned after a terminal resize.
func isGlobalModelMsg(msg tea.Msg) bool {
	switch msg.(type) {
	case spinner.TickMsg, tea.WindowSizeMsg:
		return true
	default:
		return false
	}
}

// updateDiscovery owns the public directory modal. Text stays in the input
// while it has focus; Down enters results and Up from the first result returns
// to the input. Tab remains an alternate focus switch.
func (m Model) updateDiscovery(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, isKey := msg.(tea.KeyMsg)
	if isKey {
		switch km.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.mode = m.modePrev
			return m, nil
		case "tab", "shift+tab":
			next := discoveryResultsFocus
			if m.discovery.focus == discoveryResultsFocus {
				next = discoveryInputFocus
			}
			m.discovery = m.discovery.setFocus(next)
			if m.discovery.focus == discoveryInputFocus {
				return m, m.discovery.input.Cursor.BlinkCmd()
			}
			return m, nil
		}

		if m.discovery.focus == discoveryResultsFocus {
			switch km.String() {
			case "j", "down":
				if m.discovery.cursor < len(m.discovery.results)-1 {
					m.discovery.cursor++
				}
				return m, nil
			case "k", "up":
				if m.discovery.cursor > 0 {
					m.discovery.cursor--
					return m, nil
				}
				m.discovery = m.discovery.setFocus(discoveryInputFocus)
				return m, m.discovery.input.Cursor.BlinkCmd()
			case "enter":
				return m.previewDiscovery()
			case "s":
				return m.saveDiscoveryStation()
			}
		}
		// Keep printable j/k available to type queries such as "jazz".
		// The Down arrow, unlike a rune key, is an unambiguous request to
		// enter the result list once one is available.
		if m.discovery.focus == discoveryInputFocus && km.String() == "down" && len(m.discovery.results) > 0 {
			m.discovery = m.discovery.setFocus(discoveryResultsFocus)
			return m, nil
		}
	}

	if m.discovery.focus != discoveryInputFocus {
		return m, nil
	}
	before := m.discovery.input.Value()
	var cmd tea.Cmd
	m.discovery.input, cmd = m.discovery.input.Update(msg)
	if m.discovery.input.Value() == before {
		return m, cmd
	}
	m.discovery.searchSeq++
	m.discovery.results = nil
	m.discovery.cursor = 0
	m.discovery.loading = false
	m.discovery.cached = false
	m.discovery.err = ""
	m.discovery.notice = ""
	query := m.discovery.input.Value()
	if len([]rune(strings.TrimSpace(query))) < discoveryMinQueryRunes {
		return m, cmd
	}
	return m, tea.Batch(cmd, discoverySearchTick(m.discovery.session, m.discovery.searchSeq, query))
}

func (m Model) previewDiscovery() (tea.Model, tea.Cmd) {
	station, ok := m.discovery.selected()
	if !ok {
		return m, nil
	}
	preview := config.Station{Name: station.Name, URL: station.StreamURL, Kind: config.KindStream}
	m.previewStation = &preview
	m.previewUUID = station.UUID
	m.playingIdx = -1
	m.playing = true
	m.loading = true
	requestID := m.nextPlayRequestID()
	m.currentTrack = Track{}
	m.streamInfo = audio.StreamInfoChanged{}
	m.cacheSeconds = 0
	m.bufferingStalled = false
	m.reconnectSeq++
	m.playStartedAt = time.Time{}
	return m, tea.Batch(playCmd(m.player, preview.URL, requestID), discoveryClickCmd(m.discoveryClient, station.UUID))
}

// saveDiscoveryStation persists the selected public result only after an
// explicit save action. It deliberately writes a copy first: if config.Save
// fails, the live library stays untouched and preview remains transient.
func (m Model) saveDiscoveryStation() (tea.Model, tea.Cmd) {
	station, ok := m.discovery.selected()
	if !ok {
		return m, nil
	}
	m, message, kind := m.saveDiscoveredStation(station.Name, station.StreamURL)
	m.discovery.notice = message
	m.discovery.noticeKind = kind
	return m, nil
}

// savePlayingDiscoveryStation lets the user save a transient Radio Browser
// station after leaving discovery. It intentionally targets the active stream,
// not the library cursor, so navigation cannot save the wrong station.
func (m Model) savePlayingDiscoveryStation() (tea.Model, tea.Cmd) {
	if m.previewStation == nil {
		return m, nil
	}
	preview := *m.previewStation
	m, message, kind := m.saveDiscoveredStation(preview.Name, preview.URL)
	m.toast = &Toast{Message: message, Kind: kind}
	return m, clearToastAfter()
}

// saveDiscoveredStation is the shared persistence path for a selected catalog
// result and for the active unsaved station after discovery closes.
func (m Model) saveDiscoveredStation(name, streamURL string) (Model, string, ToastKind) {
	for i, existing := range m.cfg.Stations {
		if existing.URL != streamURL {
			continue
		}
		m.cursor = i
		m.promotePreviewIfMatches(i, streamURL)
		return m, "Already in your library: " + existing.Name, ToastInfo
	}

	added := config.Station{Name: name, URL: streamURL, Kind: config.KindStream}
	stations := append(append([]config.Station(nil), m.cfg.Stations...), added)
	candidate := *m.cfg
	candidate.Stations = stations
	if err := config.Save(&candidate); err != nil {
		return m, "Could not save this station. Check your config directory permissions.", ToastError
	}
	m.cfg.Stations = stations
	m.cursor = len(stations) - 1
	m.promotePreviewIfMatches(m.cursor, added.URL)
	return m, "Saved " + added.Name + " to your library.", ToastSuccess
}

func (m *Model) promotePreviewIfMatches(idx int, streamURL string) {
	if m.previewStation == nil || m.previewStation.URL != streamURL {
		return
	}
	m.playingIdx = idx
	m.previewStation = nil
	m.previewUUID = ""
}

// updateAddStation routes input to the add/edit station modal form.
// The form is shared between both flows; result.EditIdx >= 0 means
// the user was editing an existing entry (in-place update), -1 means
// appending a brand-new one.
func (m Model) updateAddStation(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, result, stillOpen, cmd := m.addForm.update(msg)
	m.addForm = form
	if stillOpen {
		return m, cmd
	}

	// Form closed. Restore the previous layout.
	m.mode = m.modePrev
	if result.Cancelled {
		return m, nil
	}

	editing := result.EditIdx >= 0 && result.EditIdx < len(m.cfg.Stations)
	var verb string
	if editing {
		m.cfg.Stations[result.EditIdx] = result.Station
		m.cursor = result.EditIdx
		verb = "updated"
	} else {
		m.cfg.Stations = append(m.cfg.Stations, result.Station)
		m.cursor = len(m.cfg.Stations) - 1
		verb = "added"
	}

	if err := config.Save(m.cfg); err != nil {
		m.toast = &Toast{
			Message: "Could not save your station changes. Check your config directory permissions.",
			Kind:    ToastError,
		}
		return m, clearToastAfter()
	}

	m.toast = &Toast{
		Message: verb + ": " + result.Station.Name,
		Kind:    ToastSuccess,
	}
	return m, clearToastAfter()
}

func (m Model) updateShareStation(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "enter":
		return m, copyShareCmd(m.shareSnippet)
	case "esc":
		m.mode = m.modePrev
		m.shareSnippet = ""
		return m, nil
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateImportStations(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "enter":
		return m.commitImport()
	case "esc", "n", "N":
		m.mode = m.modePrev
		m.importStations = nil
		m.importSkipped = 0
		return m, nil
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) openSettings() Model {
	m.modePrev = m.mode
	m.mode = modeSettings
	m.settingsCursor = 0
	m.settingsBufferSeconds = clampInt(m.cfg.BufferSeconds, 0, settingsMaxBufferSeconds)
	m.settingsInitialBufferSeconds = clampInt(m.cfg.InitialBufferSeconds, 0, settingsMaxInitialSeconds)
	return m
}

func (m Model) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	adjust := func(delta int) {
		switch m.settingsCursor {
		case 0:
			m.settingsBufferSeconds = clampInt(m.settingsBufferSeconds+delta, 0, settingsMaxBufferSeconds)
		case 1:
			m.settingsInitialBufferSeconds = clampInt(m.settingsInitialBufferSeconds+delta, 0, settingsMaxInitialSeconds)
		}
	}
	zero := func() {
		switch m.settingsCursor {
		case 0:
			m.settingsBufferSeconds = 0
		case 1:
			m.settingsInitialBufferSeconds = 0
		}
	}

	switch km.String() {
	case "up", "k":
		m.settingsCursor = (m.settingsCursor + 1) % 2
		return m, nil
	case "down", "j":
		m.settingsCursor = (m.settingsCursor + 1) % 2
		return m, nil
	case "left", "h", "-", "_":
		adjust(-5)
		return m, nil
	case "right", "l", "+", "=":
		adjust(5)
		return m, nil
	case "0":
		zero()
		return m, nil
	case "enter":
		m.cfg.BufferSeconds = m.settingsBufferSeconds
		m.cfg.InitialBufferSeconds = m.settingsInitialBufferSeconds
		m.mode = m.modePrev
		if err := config.Save(m.cfg); err != nil {
			m.toast = &Toast{Message: "Could not save settings. Check your config directory permissions.", Kind: ToastError}
			return m, clearToastAfter()
		}
		m.toast = &Toast{Message: "settings saved — restart to apply buffer changes", Kind: ToastSuccess}
		return m, clearToastAfter()
	case "esc":
		m.mode = m.modePrev
		return m, nil
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func (m Model) openThemePicker() Model {
	m.modePrev = m.mode
	m.mode = modeThemePicker
	m.themeBeforePicker = m.theme.Name
	m.themeCursor = m.currentThemeIndex()
	return m
}

func (m Model) updateThemePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	names := theme.Names()
	if len(names) == 0 {
		m.mode = m.modePrev
		return m, nil
	}

	move := func(delta int) {
		m.themeCursor = (m.themeCursor + delta + len(names)) % len(names)
		m = m.applyTheme(names[m.themeCursor])
	}

	switch km.String() {
	case "up", "k":
		move(-1)
		return m, nil
	case "down", "j":
		move(1)
		return m, nil
	case "enter":
		name := m.theme.Name
		m.mode = m.modePrev
		m.themeBeforePicker = ""
		if m.saveTheme != nil {
			if err := m.saveTheme(name); err != nil {
				m.toast = &Toast{Message: "Could not save the selected theme.", Kind: ToastError}
				return m, clearToastAfter()
			}
		}
		return m, nil
	case "esc":
		if m.themeBeforePicker != "" {
			m = m.applyTheme(m.themeBeforePicker)
		}
		m.mode = m.modePrev
		m.themeBeforePicker = ""
		return m, nil
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// updateConfirmDelete handles the delete-confirmation modal. y/Y/enter
// commits, n/N/esc cancels. Anything else is ignored so the user can't
// accidentally dismiss it by stray keys.
func (m Model) updateConfirmDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "y", "Y", "enter":
		return m.commitDelete()
	case "n", "N", "esc":
		m.mode = m.modePrev
		m.pendingDeleteIdx = -1
		return m, nil
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) commitImport() (tea.Model, tea.Cmd) {
	count := len(m.importStations)
	if count == 0 {
		m.mode = m.modePrev
		return m, nil
	}
	m.cfg.Stations = append(m.cfg.Stations, m.importStations...)
	m.cursor = len(m.cfg.Stations) - 1
	m.mode = m.modePrev
	m.importStations = nil
	m.importSkipped = 0

	if err := config.Save(m.cfg); err != nil {
		m.toast = &Toast{
			Message: "Could not save imported stations. Check your config directory permissions.",
			Kind:    ToastError,
		}
		return m, clearToastAfter()
	}

	msg := fmt.Sprintf("imported %d station", count)
	if count != 1 {
		msg += "s"
	}
	m.toast = &Toast{Message: msg, Kind: ToastSuccess}
	return m, clearToastAfter()
}

func (m Model) newStationsOnly(stations []config.Station) ([]config.Station, int) {
	seen := make(map[string]struct{}, len(m.cfg.Stations)+len(stations))
	for _, st := range m.cfg.Stations {
		seen[st.URL] = struct{}{}
	}
	out := make([]config.Station, 0, len(stations))
	skipped := 0
	for _, st := range stations {
		if _, ok := seen[st.URL]; ok {
			skipped++
			continue
		}
		seen[st.URL] = struct{}{}
		out = append(out, st)
	}
	return out, skipped
}

func (m Model) currentThemeIndex() int {
	for i, name := range theme.Names() {
		if name == m.theme.Name {
			return i
		}
	}
	return 0
}

func (m Model) applyTheme(name string) Model {
	next, _ := theme.Lookup(name)
	m.theme = next
	m.styles = NewStyles(next)
	// Spinner color is baked at construction; refresh it so the
	// spinner stays in sync with the active theme's Muted tone.
	m.spinner.Style = lipgloss.NewStyle().Foreground(next.Muted)
	return m
}

// commitDelete removes the pending station from cfg.Stations,
// rewrites the config, and adjusts cursor / playingIdx so the model
// stays consistent (deleting the currently-playing station pauses
// playback and clears the now-playing card).
func (m Model) commitDelete() (tea.Model, tea.Cmd) {
	idx := m.pendingDeleteIdx
	m.mode = m.modePrev
	m.pendingDeleteIdx = -1
	if idx < 0 || idx >= len(m.cfg.Stations) {
		return m, nil
	}
	deleted := m.cfg.Stations[idx]

	m.cfg.Stations = append(m.cfg.Stations[:idx], m.cfg.Stations[idx+1:]...)

	// Cursor stays at the same index unless that pushes it past the
	// new end of the list.
	if m.cursor >= len(m.cfg.Stations) {
		m.cursor = len(m.cfg.Stations) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}

	// Adjust playingIdx for the shift, and pause playback if the
	// deleted entry was the one currently playing.
	var cmd tea.Cmd
	switch {
	case m.playingIdx == idx:
		m.playingIdx = -1
		m.playing = false
		m.loading = false
		m.currentTrack = Track{}
		m.streamInfo = audio.StreamInfoChanged{}
		m.cacheSeconds = 0
		m.bufferingStalled = false
		m.reconnectSeq++
		m.playStartedAt = time.Time{}
		if m.player != nil {
			cmd = pauseCmd(m.player)
		}
	case m.playingIdx > idx:
		m.playingIdx--
	}

	if err := config.Save(m.cfg); err != nil {
		m.toast = &Toast{
			Message: "Could not save the station removal. Check your config directory permissions.",
			Kind:    ToastError,
		}
		return m, tea.Batch(clearToastAfter(), cmd)
	}

	m.toast = &Toast{
		Message: "deleted: " + deleted.Name,
		Kind:    ToastSuccess,
	}
	return m, tea.Batch(clearToastAfter(), cmd)
}

// togglePlayPause is the meat of the space binding. State update is
// optimistic — the actual confirmation arrives via the audio event
// subscription, which may correct us if mpv disagrees.
func (m Model) togglePlayPause() (tea.Model, tea.Cmd) {
	if m.previewStation != nil {
		// The library cursor remains where it was while a transient result
		// previews. Selecting another saved station and pressing space must
		// replace preview rather than merely pause/resume it.
		if m.cursor >= 0 && m.cursor < len(m.cfg.Stations) && m.cfg.Stations[m.cursor].URL != m.previewStation.URL {
			m.previewStation = nil
			m.previewUUID = ""
		} else {
			if m.playing {
				m.playing = false
				m.loading = false
				m.bufferingStalled = false
				m.reconnectSeq++
				return m, pauseCmd(m.player)
			}
			m.playing = true
			m.loading = true
			requestID := m.nextPlayRequestID()
			m.bufferingStalled = false
			m.reconnectSeq++
			m.currentTrack = Track{}
			m.streamInfo = audio.StreamInfoChanged{}
			m.cacheSeconds = 0
			m.playStartedAt = time.Time{}
			return m, playCmd(m.player, m.previewStation.URL, requestID)
		}
	}
	if len(m.cfg.Stations) == 0 {
		return m, nil
	}
	// Refuse to play YouTube stations when yt-dlp isn't on $PATH —
	// otherwise mpv emits a generic "stream load failed" error and
	// the user has to guess at the cause.
	if !m.youtubeReady && m.cfg.Stations[m.cursor].IsYouTube() {
		m.toast = &Toast{
			Message: "yt-dlp not installed — install it to play YouTube stations",
			Kind:    ToastError,
		}
		return m, clearToastAfter()
	}
	if m.cursor == m.playingIdx {
		// Toggle pause/resume on the currently-playing station. Direct live
		// streams are reloaded on resume instead of merely unpaused: after a
		// laptop sleep mpv can be left with a dead HTTP connection, and
		// set pause=false will not reconnect it. YouTube keeps true resume
		// semantics because those URLs are finite media, not live radio.
		if m.playing {
			m.playing = false
			m.loading = false
			m.bufferingStalled = false
			m.reconnectSeq++
			return m, pauseCmd(m.player)
		}
		m.playing = true
		m.bufferingStalled = false
		m.reconnectSeq++
		if !m.cfg.Stations[m.cursor].IsYouTube() {
			m.loading = true
			requestID := m.nextPlayRequestID()
			m.currentTrack = Track{}
			m.streamInfo = audio.StreamInfoChanged{}
			m.cacheSeconds = 0
			m.playStartedAt = time.Time{}
			return m, playCmd(m.player, m.cfg.Stations[m.cursor].URL, requestID)
		}
		return m, resumeCmd(m.player)
	}
	// Switching to a different station — replace playback. Mark the
	// model as loading; the spinner takes over the status slot until
	// PlaybackStarted arrives from mpv.
	m.playingIdx = m.cursor
	m.playing = true
	m.loading = true
	requestID := m.nextPlayRequestID()
	m.currentTrack = Track{}
	m.streamInfo = audio.StreamInfoChanged{}
	m.cacheSeconds = 0
	m.bufferingStalled = false
	m.reconnectSeq++
	m.playStartedAt = time.Time{}
	return m, playCmd(m.player, m.cfg.Stations[m.cursor].URL, requestID)
}

// updateMixer routes input while the ambient mixer modal is open.
// Close keys (esc/x) and global quit are intercepted; everything else
// is delegated to mixerUI.handle.
func (m Model) updateMixer(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "esc", "x":
		m.mode = m.modePrev
		nm, cmd := m.scheduleAmbientSave()
		return nm, cmd
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	var ipcCmd tea.Cmd
	m.mixerUI, ipcCmd = m.mixerUI.handle(km.String())
	nm, saveCmd := m.scheduleAmbientSave()
	return nm, tea.Batch(ipcCmd, saveCmd)
}

// scheduleAmbientSave bumps the debounce sequence and returns a tick
// that will fire the save callback after ambientSaveDebounce — unless
// a newer keypress bumps the seq again first.
func (m Model) scheduleAmbientSave() (Model, tea.Cmd) {
	m.ambientSaveSeq++
	return m, ambientSaveTick(m.ambientSaveSeq)
}
