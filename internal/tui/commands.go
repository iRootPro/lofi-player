package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/iRootPro/lofi-player/internal/audio"
)

// errorClearAfter is the lifetime of a transient PlaybackError shown in
// the help-bar slot. Phase 2 will replace this with a proper Toast
// component; for Phase 1 it's just a delayed Tick that clears
// Model.lastError.
const errorClearAfter = 3 * time.Second

// playCmd starts (or replaces) playback of url. A nil result means the
// command succeeded and the actual "started playing" signal will arrive
// later via the audio event subscription.
func playCmd(p *audio.Player, url string) tea.Cmd {
	return func() tea.Msg {
		if err := p.Play(url); err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		return nil
	}
}

// pauseCmd asks mpv to pause; the matching PlaybackPausedMsg arrives via
// the event subscription, not as a direct return.
func pauseCmd(p *audio.Player) tea.Cmd {
	return func() tea.Msg {
		if err := p.Pause(); err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		return nil
	}
}

// resumeCmd is the inverse of pauseCmd.
func resumeCmd(p *audio.Player) tea.Cmd {
	return func() tea.Msg {
		if err := p.Resume(); err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		return nil
	}
}

// setVolumeCmd pushes a volume change to mpv.
func setVolumeCmd(p *audio.Player, percent int) tea.Cmd {
	return func() tea.Msg {
		if err := p.SetVolume(percent); err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		return nil
	}
}

// waitForEvent reads exactly one event from the player and maps it to
// the matching XxxMsg. The Update loop must re-arm the subscription by
// returning waitForEvent(p) again after handling each event (plan §4.2).
func waitForEvent(p *audio.Player) tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-p.Events()
		if !ok {
			// Player closed; let the subscription chain end.
			return nil
		}
		switch e := evt.(type) {
		case audio.MetadataChanged:
			return MetadataChangedMsg{Title: e.Title, Artist: e.Artist}
		case audio.PlaybackStarted:
			return PlaybackStartedMsg{}
		case audio.PlaybackPaused:
			return PlaybackPausedMsg{}
		case audio.PlaybackError:
			return PlaybackErrorMsg{Err: e.Err}
		case audio.EOF:
			return EOFMsg{}
		}
		return nil
	}
}

// clearErrorAfter schedules a clearErrorMsg after errorClearAfter.
func clearErrorAfter() tea.Cmd {
	return tea.Tick(errorClearAfter, func(time.Time) tea.Msg {
		return clearErrorMsg{}
	})
}
