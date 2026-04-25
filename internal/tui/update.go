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
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.cfg.Stations)-1 {
				m.cursor++
			}
		case key.Matches(msg, m.keys.PlayPause):
			if len(m.cfg.Stations) == 0 {
				return m, nil
			}
			if m.playingIdx == m.cursor {
				m.playing = !m.playing
			} else {
				m.playingIdx = m.cursor
				m.playing = true
			}
		case key.Matches(msg, m.keys.VolUp):
			m.volume = clampVolume(m.volume + volumeStep)
		case key.Matches(msg, m.keys.VolDown):
			m.volume = clampVolume(m.volume - volumeStep)
		case key.Matches(msg, m.keys.Help):
			m.showFullHelp = !m.showFullHelp
		}
	}
	return m, nil
}
