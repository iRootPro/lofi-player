package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/iRootPro/lofi-player/internal/audio"
	"github.com/iRootPro/lofi-player/internal/config"
	"github.com/iRootPro/lofi-player/internal/theme"
)

const volumeStep = 5

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
					Message: fmt.Sprintf("ambient state save failed: %v", err),
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
	}
	// Modal states intercept input first so the form can capture text
	// without the global keymap stealing characters like 'q'.
	if m.mode == modeAddStation {
		return m.updateAddStation(msg)
	}
	if m.mode == modeMixer {
		return m.updateMixer(msg)
	}
	if m.mode == modeConfirmDelete {
		return m.updateConfirmDelete(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case MetadataChangedMsg:
		m.currentTrack = Track{Title: msg.Title, Artist: msg.Artist}
		return m, waitForEvent(m.player)

	case PlaybackStartedMsg:
		m.playing = true
		m.loading = false
		// First start after a fresh Play call: anchor the uptime
		// counter. Resume after pause keeps the previous anchor so
		// "listening 1h 23m" doesn't reset every time the user toggles.
		if m.playStartedAt.IsZero() {
			m.playStartedAt = time.Now()
		}
		return m, waitForEvent(m.player)

	case PlaybackPausedMsg:
		m.playing = false
		m.loading = false
		return m, waitForEvent(m.player)

	case PlaybackErrorMsg:
		m.toast = &Toast{Message: msg.Err.Error(), Kind: ToastError}
		m.playing = false
		m.loading = false
		m.playingIdx = -1
		m.currentTrack = Track{}
		m.streamInfo = audio.StreamInfoChanged{}
		m.cacheSeconds = 0
		m.playStartedAt = time.Time{}
		return m, tea.Batch(clearToastAfter(), waitForEvent(m.player))

	case EOFMsg:
		m.playing = false
		m.loading = false
		m.playingIdx = -1
		m.currentTrack = Track{}
		m.streamInfo = audio.StreamInfoChanged{}
		m.cacheSeconds = 0
		m.playStartedAt = time.Time{}
		return m, waitForEvent(m.player)

	case StreamInfoChangedMsg:
		m.streamInfo = audio.StreamInfoChanged{
			Bitrate:    msg.Bitrate,
			Codec:      msg.Codec,
			SampleRate: msg.SampleRate,
			Channels:   msg.Channels,
		}
		return m, waitForEvent(m.player)

	case CacheStateChangedMsg:
		m.cacheSeconds = msg.Seconds
		return m, waitForEvent(m.player)

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
		next, _ := theme.Lookup(theme.Next(m.theme.Name))
		m.theme = next
		m.styles = NewStyles(next)
		// Spinner color is baked at construction; refresh it so the
		// spinner stays in sync with the active theme's Muted tone.
		m.spinner.Style = lipgloss.NewStyle().Foreground(next.Muted)
		return m, nil

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

	case key.Matches(msg, m.keys.MixerOpen):
		m.modePrev = m.mode
		m.mode = modeMixer
		return m, nil

	case key.Matches(msg, m.keys.StreamInfo):
		m.showStreamInfo = !m.showStreamInfo
		return m, nil

	case key.Matches(msg, m.keys.Help):
		m.showFullHelp = !m.showFullHelp
		return m, nil
	}
	return m, nil
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
			Message: fmt.Sprintf("station %s in memory but config save failed: %v", verb, err),
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
		m.playStartedAt = time.Time{}
		if m.player != nil {
			cmd = pauseCmd(m.player)
		}
	case m.playingIdx > idx:
		m.playingIdx--
	}

	if err := config.Save(m.cfg); err != nil {
		m.toast = &Toast{
			Message: fmt.Sprintf("removed in memory but config save failed: %v", err),
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
		// Toggle pause/resume on the currently-playing station.
		if m.playing {
			m.playing = false
			return m, pauseCmd(m.player)
		}
		m.playing = true
		return m, resumeCmd(m.player)
	}
	// Switching to a different station — replace playback. Mark the
	// model as loading; the spinner takes over the status slot until
	// PlaybackStarted arrives from mpv.
	m.playingIdx = m.cursor
	m.playing = true
	m.loading = true
	m.currentTrack = Track{}
	m.streamInfo = audio.StreamInfoChanged{}
	m.cacheSeconds = 0
	m.playStartedAt = time.Time{}
	return m, playCmd(m.player, m.cfg.Stations[m.cursor].URL)
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
	m.mixerUI = m.mixerUI.handle(km.String())
	nm, cmd := m.scheduleAmbientSave()
	return nm, cmd
}

// scheduleAmbientSave bumps the debounce sequence and returns a tick
// that will fire the save callback after ambientSaveDebounce — unless
// a newer keypress bumps the seq again first.
func (m Model) scheduleAmbientSave() (Model, tea.Cmd) {
	m.ambientSaveSeq++
	return m, ambientSaveTick(m.ambientSaveSeq)
}
