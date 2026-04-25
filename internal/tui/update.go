package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/iRootPro/lofi-player/internal/config"
	"github.com/iRootPro/lofi-player/internal/pomodoro"
	"github.com/iRootPro/lofi-player/internal/theme"
)

const volumeStep = 5

// Update applies a message to the model and returns the new model plus
// any commands to run. Receiver is by value; never mutate m through a
// pointer (plan §4.2).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Modal states intercept input first so the form can capture text
	// without the global keymap stealing characters like 'q' or 'p'.
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
		return m, waitForEvent(m.player)

	case PlaybackPausedMsg:
		m.playing = false
		return m, waitForEvent(m.player)

	case PlaybackErrorMsg:
		m.toast = &Toast{Message: msg.Err.Error(), Kind: ToastError}
		m.playing = false
		m.playingIdx = -1
		m.currentTrack = Track{}
		return m, tea.Batch(clearToastAfter(), waitForEvent(m.player))

	case EOFMsg:
		m.playing = false
		m.playingIdx = -1
		m.currentTrack = Track{}
		return m, waitForEvent(m.player)

	case clearToastMsg:
		m.toast = nil
		return m, nil

	case volTickMsg:
		next, vel, settled := stepVolume(m.volumeDisplayed, m.volumeVelocity, float64(m.volume))
		m.volumeDisplayed = next
		m.volumeVelocity = vel
		if settled {
			m.volumeAnimating = false
			return m, nil
		}
		return m, tickVolAnim()

	case pomodoroTickMsg:
		return m.handlePomodoroTick(msg.at)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
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
		return m, m.startVolumeAnim(setVolumeCmd(m.player, m.volume))

	case key.Matches(msg, m.keys.VolDown):
		m.volume = clampVolume(m.volume - volumeStep)
		return m, m.startVolumeAnim(setVolumeCmd(m.player, m.volume))

	case key.Matches(msg, m.keys.ThemeCycle):
		next, _ := theme.Lookup(theme.Next(m.theme.Name))
		m.theme = next
		m.styles = NewStyles(next)
		return m, nil

	case key.Matches(msg, m.keys.Mini):
		if m.mode == modeFull {
			m.mode = modeMini
		} else {
			m.mode = modeFull
		}
		return m, nil

	case key.Matches(msg, m.keys.Pomodoro):
		return m.togglePomodoro()

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

// startVolumeAnim returns a Cmd that batches the player update with
// the spring tick (only if not already animating, so spamming the key
// doesn't pile up overlapping tick loops).
func (m *Model) startVolumeAnim(playerCmd tea.Cmd) tea.Cmd {
	if m.volumeAnimating {
		return playerCmd
	}
	m.volumeAnimating = true
	return tea.Batch(playerCmd, tickVolAnim())
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

// togglePomodoro starts a focus session if idle, otherwise stops the
// current session.
func (m Model) togglePomodoro() (tea.Model, tea.Cmd) {
	now := time.Now()
	if m.session.Phase == pomodoro.PhaseIdle {
		next, _ := pomodoro.Start(m.session, now, m.pomoConfig())
		m.session = next
		m.stats = pomodoro.RegisterFocusStart(m.stats, now)
		m.toast = &Toast{Message: "focus session started", Kind: ToastInfo}
		cmds := []tea.Cmd{
			clearToastAfter(),
			notifyCmd("lofi-player", "focus session started"),
		}
		if !m.pomoTickActive {
			m.pomoTickActive = true
			cmds = append(cmds, pomodoroTick())
		}
		return m, tea.Batch(cmds...)
	}

	next, _ := pomodoro.Stop(m.session)
	m.session = next
	m.toast = &Toast{Message: "focus session stopped", Kind: ToastInfo}
	// Tick will see Idle and stop rescheduling itself.
	return m, clearToastAfter()
}

// handlePomodoroTick advances the state machine, fires side effects on
// transitions, accumulates listened-time, and reschedules itself.
func (m Model) handlePomodoroTick(at time.Time) (tea.Model, tea.Cmd) {
	cfg := m.pomoConfig()
	prevPhase := m.session.Phase
	next, transition := pomodoro.Tick(m.session, at, cfg)
	m.session = next

	var cmds []tea.Cmd

	switch transition {
	case pomodoro.StartedFocus:
		m.stats = pomodoro.RegisterFocusStart(m.stats, at)
		if m.cfg.Pomodoro.AutoResumeOnFocus && !m.playing && m.playingIdx >= 0 {
			m.playing = true
			cmds = append(cmds, resumeCmd(m.player))
		}
		m.toast = &Toast{Message: "focus session", Kind: ToastInfo}
		cmds = append(cmds, clearToastAfter(),
			notifyCmd("lofi-player", "back to focus"))

	case pomodoro.StartedShortBreak:
		if m.cfg.Pomodoro.AutoPauseOnBreak && m.playing {
			m.playing = false
			cmds = append(cmds, pauseCmd(m.player))
		}
		m.toast = &Toast{Message: "short break", Kind: ToastInfo}
		cmds = append(cmds, clearToastAfter(),
			notifyCmd("lofi-player", "take a 5 minute break"))

	case pomodoro.StartedLongBreak:
		if m.cfg.Pomodoro.AutoPauseOnBreak && m.playing {
			m.playing = false
			cmds = append(cmds, pauseCmd(m.player))
		}
		m.toast = &Toast{Message: "long break", Kind: ToastInfo}
		cmds = append(cmds, clearToastAfter(),
			notifyCmd("lofi-player", "take a 15 minute break"))
	}

	// Listening time accumulates only during focus + actively playing.
	if m.session.Phase == pomodoro.PhaseFocus && m.playing && prevPhase == pomodoro.PhaseFocus {
		m.stats = pomodoro.TickListening(m.stats, at, time.Second)
	}

	if m.session.Phase != pomodoro.PhaseIdle {
		cmds = append(cmds, pomodoroTick())
	} else {
		m.pomoTickActive = false
	}
	return m, tea.Batch(cmds...)
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
