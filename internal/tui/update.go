package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/iRootPro/lofi-player/internal/config"
	"github.com/iRootPro/lofi-player/internal/theme"
)

const volumeStep = 5

// Update applies a message to the model and returns the new model plus
// any commands to run. Receiver is by value; never mutate m through a
// pointer (plan §4.2).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Modal states intercept input first so the form can capture text
	// without the global keymap stealing characters like 'q'.
	if m.mode == modeAddStation {
		return m.updateAddStation(msg)
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
		return m, tea.Batch(clearToastAfter(), waitForEvent(m.player))

	case EOFMsg:
		m.playing = false
		m.loading = false
		m.playingIdx = -1
		m.currentTrack = Track{}
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

	case key.Matches(msg, m.keys.Help):
		m.showFullHelp = !m.showFullHelp
		return m, nil
	}
	return m, nil
}

// updateAddStation routes input to the add-station modal form. On
// submission the new station is appended to cfg.Stations and the
// config file is rewritten; on cancellation no state changes.
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

	// Append, persist, point cursor at new station.
	m.cfg.Stations = append(m.cfg.Stations, result.Station)
	m.cursor = len(m.cfg.Stations) - 1

	if err := config.Save(m.cfg); err != nil {
		m.toast = &Toast{
			Message: fmt.Sprintf("station added in memory but config save failed: %v", err),
			Kind:    ToastError,
		}
		return m, clearToastAfter()
	}

	m.toast = &Toast{
		Message: "added: " + result.Station.Name,
		Kind:    ToastSuccess,
	}
	return m, clearToastAfter()
}

// togglePlayPause is the meat of the space binding. State update is
// optimistic — the actual confirmation arrives via the audio event
// subscription, which may correct us if mpv disagrees.
func (m Model) togglePlayPause() (tea.Model, tea.Cmd) {
	if len(m.cfg.Stations) == 0 {
		return m, nil
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
	return m, playCmd(m.player, m.cfg.Stations[m.cursor].URL)
}
