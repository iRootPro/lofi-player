package tui

import (
	"context"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/iRootPro/lofi-player/internal/audio"
	"github.com/iRootPro/lofi-player/internal/doctor"
	"github.com/iRootPro/lofi-player/internal/radiobrowser"
	sharepkg "github.com/iRootPro/lofi-player/internal/share"
)

// toastLifetime is how long a Toast stays visible before a delayed Tick
// dismisses it.
const toastLifetime = 3 * time.Second

const discoverySearchDebounce = 350 * time.Millisecond

// playCmd starts (or replaces) playback of url. A nil result means the
// command succeeded and the actual "started playing" signal will arrive
// later via the audio event subscription.
func playCmd(p *audio.Player, url string, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		if err := p.Play(url, requestID); err != nil {
			return PlaybackErrorMsg{Err: err, RequestID: requestID}
		}
		return nil
	}
}

// pauseCmd asks mpv to pause; the matching PlaybackPausedMsg arrives via
// the event subscription, not as a direct return. A failure here is a
// transient IPC issue, not a stream death — surface it as a toast,
// don't rip down the now-playing card.
func pauseCmd(p *audio.Player) tea.Cmd {
	return func() tea.Msg {
		if err := p.Pause(); err != nil {
			return CommandFailedMsg{Action: "pause", Err: err}
		}
		return nil
	}
}

// resumeCmd is the inverse of pauseCmd.
func resumeCmd(p *audio.Player) tea.Cmd {
	return func() tea.Msg {
		if err := p.Resume(); err != nil {
			return CommandFailedMsg{Action: "resume", Err: err}
		}
		return nil
	}
}

// setVolumeCmd pushes a volume change to mpv.
func setVolumeCmd(p *audio.Player, percent int) tea.Cmd {
	return func() tea.Msg {
		if err := p.SetVolume(percent); err != nil {
			return CommandFailedMsg{Action: "set volume", Err: err}
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
			return MetadataChangedMsg{Title: e.Title, Artist: e.Artist, RequestID: e.RequestID}
		case audio.PlaybackStarted:
			return PlaybackStartedMsg{RequestID: e.RequestID}
		case audio.PlaybackLoadStarted:
			return PlaybackLoadStartedMsg{RequestID: e.RequestID}
		case audio.PlaybackPaused:
			return PlaybackPausedMsg{RequestID: e.RequestID}
		case audio.PlaybackError:
			return PlaybackErrorMsg{Err: e.Err, RequestID: e.RequestID}
		case audio.EOF:
			return EOFMsg{RequestID: e.RequestID}
		case audio.StreamInfoChanged:
			return StreamInfoChangedMsg{
				Bitrate:    e.Bitrate,
				Codec:      e.Codec,
				SampleRate: e.SampleRate,
				Channels:   e.Channels,
				RequestID:  e.RequestID,
			}
		case audio.CacheStateChanged:
			return CacheStateChangedMsg{Seconds: e.Seconds, RequestID: e.RequestID}
		case audio.BufferingChanged:
			return BufferingChangedMsg{Stalled: e.Stalled, RequestID: e.RequestID}
		}
		return nil
	}
}

// clearToastAfter schedules a clearToastMsg after toastLifetime.
func clearToastAfter() tea.Cmd {
	return tea.Tick(toastLifetime, func(time.Time) tea.Msg {
		return clearToastMsg{}
	})
}

func copyShareCmd(text string) tea.Cmd {
	return func() tea.Msg {
		return shareCopiedMsg{Err: clipboard.WriteAll(text)}
	}
}

func doctorCheckCmd(ctx context.Context, seq int, opts doctor.Options) tea.Cmd {
	return func() tea.Msg {
		return doctorCheckResultMsg{seq: seq, report: doctor.Collect(ctx, opts)}
	}
}

func copyDoctorCmd(kind, text string) tea.Cmd {
	return func() tea.Msg {
		return doctorCopiedMsg{kind: kind, err: clipboard.WriteAll(text)}
	}
}

func importClipboardCmd() tea.Cmd {
	return func() tea.Msg {
		text, err := clipboard.ReadAll()
		if err != nil {
			return importClipboardMsg{Err: err}
		}
		stations, err := sharepkg.Parse(text)
		return importClipboardMsg{Stations: stations, Err: err}
	}
}

func discoverySearchTick(session, seq int, query string) tea.Cmd {
	return tea.Tick(discoverySearchDebounce, func(time.Time) tea.Msg {
		return discoverySearchTickMsg{session: session, seq: seq, query: query}
	})
}

func discoverySearchCmd(client radiobrowser.Client, session, seq int, query string) tea.Cmd {
	return func() tea.Msg {
		results, err := client.Search(context.Background(), query)
		return discoverySearchResultMsg{session: session, seq: seq, results: results, err: err}
	}
}

// discoveryClickCmd deliberately returns no message: click reporting is
// best-effort telemetry requested by Radio Browser and must never affect
// playback or the visible TUI state.
func discoveryClickCmd(client radiobrowser.Client, stationUUID string) tea.Cmd {
	return func() tea.Msg {
		if client != nil && stationUUID != "" {
			_ = client.RegisterClick(context.Background(), stationUUID)
		}
		return nil
	}
}

// pulseInterval controls the live-indicator pulse cadence. A long
// half-period (700 ms) reads as a calm "alive" signal rather than a
// distracting blink.
const pulseInterval = 700 * time.Millisecond

// pulseTick schedules the next pulseTickMsg.
func pulseTick() tea.Cmd {
	return tea.Tick(pulseInterval, func(time.Time) tea.Msg {
		return pulseTickMsg{}
	})
}

// ambientSaveDebounce is how long volume changes are coalesced before a
// state.Save fires. 500ms is short enough to feel responsive when the
// user lets go and long enough to absorb a held key (which repeats
// every ~50ms).
const ambientSaveDebounce = 500 * time.Millisecond

// ambientSaveTick schedules an ambientSaveTickMsg carrying seq. The
// Update handler compares seq to Model.ambientSaveSeq to drop stale
// ticks scheduled by earlier keypresses.
func ambientSaveTick(seq int) tea.Cmd {
	return tea.Tick(ambientSaveDebounce, func(time.Time) tea.Msg {
		return ambientSaveTickMsg{seq: seq}
	})
}

// logoInterval controls how fast the logo shimmer crest advances.
// 150 ms / cell ≈ 6.7 cells/sec — slow enough to read as "calm
// glow" rather than the strobing of a faster animation.
const logoInterval = 150 * time.Millisecond

// logoTick schedules the next logoTickMsg.
func logoTick() tea.Cmd {
	return tea.Tick(logoInterval, func(time.Time) tea.Msg {
		return logoTickMsg{}
	})
}

// clockInterval is the cadence of the once-a-second clock that drives
// the uptime label.
const clockInterval = time.Second

// clockTick schedules the next clockTickMsg.
func clockTick() tea.Cmd {
	return tea.Tick(clockInterval, func(t time.Time) tea.Msg {
		return clockTickMsg{At: t}
	})
}

// streamReconnectDelay is long enough to avoid flapping on brief network
// hiccups, but short enough to recover quickly after laptop sleep.
const streamReconnectDelay = 8 * time.Second

func reconnectStreamAfter(seq int) tea.Cmd {
	return tea.Tick(streamReconnectDelay, func(time.Time) tea.Msg {
		return reconnectStreamMsg{seq: seq}
	})
}
