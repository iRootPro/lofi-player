package tui

import "time"

// Bridge messages between internal/audio events and the Bubble Tea
// Update loop. The translator in commands.go maps each audio.Event into
// the matching XxxMsg here.

// MetadataChangedMsg carries a fresh ICY (or media-title) update.
type MetadataChangedMsg struct {
	Title  string
	Artist string
}

// PlaybackStartedMsg fires when mpv unpauses.
type PlaybackStartedMsg struct{}

// PlaybackPausedMsg fires when mpv enters paused state.
type PlaybackPausedMsg struct{}

// PlaybackErrorMsg surfaces a recoverable playback failure (DNS, 404,
// network drop). The TUI shows it as a transient message in the
// help-bar slot and auto-clears after a few seconds.
type PlaybackErrorMsg struct {
	Err error
}

// EOFMsg fires when a stream ends. For live streams this normally only
// happens when the server shuts down.
type EOFMsg struct{}

// clearToastMsg is delivered by a delayed tea.Tick to wipe an active
// Toast after its lifetime expires.
type clearToastMsg struct{}

// pomodoroTickMsg is the 1 Hz tick that drives the pomodoro state
// machine, listened-time accumulation, and countdown display refresh
// while a session is active.
type pomodoroTickMsg struct{ at time.Time }
