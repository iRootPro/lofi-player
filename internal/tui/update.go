package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

const volumeStep = 5

// Update applies a message to the model and returns the new model plus
// any commands to run. Receiver is by value; never mutate m through a
// pointer (plan §4.2).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		return m, waitForEvent(m.player)

	case PlaybackPausedMsg:
		m.playing = false
		return m, waitForEvent(m.player)

	case PlaybackErrorMsg:
		m.lastError = msg.Err.Error()
		m.playing = false
		m.playingIdx = -1
		m.currentTrack = Track{}
		return m, tea.Batch(clearErrorAfter(), waitForEvent(m.player))

	case EOFMsg:
		m.playing = false
		m.playingIdx = -1
		m.currentTrack = Track{}
		return m, waitForEvent(m.player)

	case clearErrorMsg:
		m.lastError = ""
		return m, nil
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

	case key.Matches(msg, m.keys.Help):
		m.showFullHelp = !m.showFullHelp
		return m, nil
	}
	return m, nil
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
	// Switching to a different station — replace playback.
	m.playingIdx = m.cursor
	m.playing = true
	m.currentTrack = Track{}
	return m, playCmd(m.player, m.cfg.Stations[m.cursor].URL)
}
