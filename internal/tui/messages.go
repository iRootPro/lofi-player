package tui

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

// pulseTickMsg toggles the live indicator's brightness for a soft
// "alive" pulse while a station is actively playing.
type pulseTickMsg struct{}

// ambientSaveTickMsg is the deferred fire from a 500ms debounce window.
// seq is the snapshot of Model.ambientSaveSeq at schedule time; the
// handler ignores stale ticks (msg.seq < current seq) so a held key
// collapses to a single save once the user lets go.
type ambientSaveTickMsg struct{ seq int }

// logoTickMsg drives the shimmer that sweeps across the ASCII logo
// next to the now-playing card. The tick runs globally; Update
// advances the logo's internal counter only while playing so the
// shimmer freezes on pause without breaking the tick chain.
type logoTickMsg struct{}
