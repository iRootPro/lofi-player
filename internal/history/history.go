// Package history persists private, local listening sessions.
//
// History deliberately contains station snapshots rather than stream URLs:
// a stream URL may carry credentials or short-lived query tokens and is never
// needed to render listening statistics. Its file lives beside the rest of
// lofi-player's XDG state, but separately from the small UI state snapshot.
package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	// MinListenSeconds filters accidental station previews from history.
	MinListenSeconds int64 = 30
	// RetentionDays bounds how long completed local history is retained.
	RetentionDays = 90
	// MaxSessions is the secondary retention bound for frequent listeners.
	MaxSessions = 5000
)

const (
	SourceLibrary   = "library"
	SourceDiscovery = "discovery"

	EndPaused      = "paused"
	EndSwitched    = "switched"
	EndError       = "error"
	EndEOF         = "eof"
	EndQuit        = "quit"
	EndInterrupted = "interrupted"
	EndDeleted     = "deleted"
)

// Segment is one continuous period of audible playback. Keeping segments
// makes daily and weekly totals exact even if a listening session spans
// midnight or contains pauses.
type Segment struct {
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}

// Session is a completed listening session. It holds no stream URL or raw
// mpv error data.
type Session struct {
	StationName   string    `json:"station_name"`
	Source        string    `json:"source"`
	StartedAt     time.Time `json:"started_at"`
	EndedAt       time.Time `json:"ended_at"`
	PlayedSeconds int64     `json:"played_seconds"`
	EndedReason   string    `json:"ended_reason"`
	Segments      []Segment `json:"segments"`
}

// ActiveSession is checkpointed while playback continues. On the next launch
// Recover closes it at its most recent checkpoint, avoiding an over-count of
// time while the application was not running.
type ActiveSession struct {
	StationName   string    `json:"station_name"`
	Source        string    `json:"source"`
	StartedAt     time.Time `json:"started_at"`
	ActiveSince   time.Time `json:"active_since,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
	PlayedSeconds int64     `json:"played_seconds"`
	// PlayedNanoseconds preserves exact accumulated duration across pauses;
	// PlayedSeconds is retained as a convenient, stable display field.
	PlayedNanoseconds int64     `json:"played_nanoseconds"`
	Segments          []Segment `json:"segments"`
}

// History is the complete on-disk document. Version makes future migrations
// explicit while older files without it remain readable.
type History struct {
	Version  int            `json:"version"`
	Sessions []Session      `json:"sessions"`
	Active   *ActiveSession `json:"active,omitempty"`
}

// Path returns the canonical local history path.
func Path() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "lofi-player", "history.json"), nil
}

// Load returns an empty history for a missing or malformed file. The broken
// file is left untouched so it can be inspected or recovered by the user.
func Load() History {
	p, err := Path()
	if err != nil {
		return History{Version: 1}
	}
	return loadFromFile(p)
}

// Save atomically replaces the history file. The file is private to the user.
func Save(h History) error {
	p, err := Path()
	if err != nil {
		return err
	}
	return saveToFile(p, h)
}

func loadFromFile(path string) History {
	data, err := os.ReadFile(path)
	if err != nil {
		return History{Version: 1}
	}
	var h History
	if err := json.Unmarshal(data, &h); err != nil {
		return History{Version: 1}
	}
	if h.Version == 0 {
		h.Version = 1
	}
	return h.prune(time.Now())
}

func saveToFile(path string, h History) error {
	h = h.prune(time.Now())
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".history-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

// Begin starts a new confirmed playback interval or resumes a paused interval
// for the same station. Switching stations closes the old session first.
func (h History) Begin(stationName, source string, at time.Time) History {
	at = at.UTC()
	if h.Version == 0 {
		h.Version = 1
	}
	if h.Active != nil {
		if h.Active.StationName == stationName && h.Active.Source == source {
			if h.Active.ActiveSince.IsZero() {
				h.Active.ActiveSince = at
			}
			h.Active.UpdatedAt = at
			return h
		}
		h = h.End(at, EndSwitched)
	}
	h.Active = &ActiveSession{
		StationName: stationName,
		Source:      source,
		StartedAt:   at,
		ActiveSince: at,
		UpdatedAt:   at,
	}
	return h
}

// Pause settles the currently audible segment but keeps the session open so a
// later resume of the same station remains one logical listen.
func (h History) Pause(at time.Time) History {
	if h.Active == nil {
		return h
	}
	h.settle(at.UTC())
	return h
}

// Checkpoint commits elapsed listening time to the active document. It is
// intentionally cheap and is called at a low cadence by the TUI.
func (h History) Checkpoint(at time.Time) History {
	if h.Active == nil {
		return h
	}
	at = at.UTC()
	wasPlaying := !h.Active.ActiveSince.IsZero()
	h.settle(at)
	if wasPlaying {
		// Keep the session marked as audible after persisting its completed
		// segment. If the process exits before the next checkpoint, Recover
		// will close at this precise boundary rather than count offline time.
		h.Active.ActiveSince = at
		h.Active.UpdatedAt = at
	}
	return h
}

// End closes a session. Listens shorter than MinListenSeconds are discarded.
func (h History) End(at time.Time, reason string) History {
	if h.Active == nil {
		return h
	}
	at = at.UTC()
	h.settle(at)
	a := *h.Active
	h.Active = nil
	// Segments are the source of truth: they preserve sub-second duration and
	// also let a checkpoint from an older build recover accurately even though
	// its aggregate played_seconds was truncated at every pause.
	played := segmentsDuration(a.Segments)
	if played == 0 && a.PlayedSeconds > 0 {
		// Forward-compatible fallback for a checkpoint written by an earlier
		// build that only had whole seconds.
		played = time.Duration(a.PlayedSeconds) * time.Second
	}
	if played < time.Duration(MinListenSeconds)*time.Second {
		return h
	}
	playedSeconds := int64((played + time.Second/2) / time.Second)
	h.Sessions = append(h.Sessions, Session{
		StationName:   a.StationName,
		Source:        a.Source,
		StartedAt:     a.StartedAt,
		EndedAt:       at,
		PlayedSeconds: playedSeconds,
		EndedReason:   reason,
		Segments:      a.Segments,
	})
	return h.prune(at)
}

// Recover closes a checkpointed active session without charging the time the
// app was not running. It reports whether the caller should persist the result.
func (h History) Recover() (History, bool) {
	if h.Active == nil {
		return h, false
	}
	checkpoint := h.Active.UpdatedAt
	if checkpoint.IsZero() {
		checkpoint = h.Active.StartedAt
	}
	return h.End(checkpoint, EndInterrupted), true
}

// Clear removes completed local history. A currently-playing session starts
// fresh at clear time, so the cleared interval cannot reappear later.
func (h History) Clear(at time.Time) History {
	h.Sessions = nil
	if h.Active != nil {
		station, source := h.Active.StationName, h.Active.Source
		wasPlaying := !h.Active.ActiveSince.IsZero()
		h.Active = nil
		if wasPlaying {
			h = h.Begin(station, source, at)
		} else {
			// Clearing while paused must not turn paused wall-clock time into
			// audible playback. Keep an empty paused session ready for resume.
			h.Active = &ActiveSession{
				StationName: station,
				Source:      source,
				StartedAt:   at.UTC(),
				UpdatedAt:   at.UTC(),
			}
		}
	}
	return h
}

// SegmentsWithActive returns completed session segments plus a synthetic live
// segment ending at now. It never mutates h.
func (h History) SegmentsWithActive(now time.Time) []Segment {
	segments := make([]Segment, 0, len(h.Sessions)+1)
	for _, s := range h.Sessions {
		segments = append(segments, s.Segments...)
	}
	if h.Active != nil {
		segments = append(segments, h.Active.Segments...)
		if !h.Active.ActiveSince.IsZero() && now.After(h.Active.ActiveSince) {
			segments = append(segments, Segment{StartedAt: h.Active.ActiveSince, EndedAt: now})
		}
	}
	return segments
}

// DurationBetween returns exact audible time within [from, to), including an
// active interval when requested. It is suitable for calendar summaries.
func (h History) DurationBetween(from, to, now time.Time) time.Duration {
	if !to.After(from) {
		return 0
	}
	var total time.Duration
	for _, segment := range h.SegmentsWithActive(now) {
		start := maxTime(segment.StartedAt, from)
		end := minTime(segment.EndedAt, to)
		if end.After(start) {
			total += end.Sub(start)
		}
	}
	return total
}

func (h History) settle(at time.Time) {
	if h.Active == nil {
		return
	}
	a := h.Active
	if !a.ActiveSince.IsZero() && at.After(a.ActiveSince) {
		duration := at.Sub(a.ActiveSince)
		a.Segments = append(a.Segments, Segment{StartedAt: a.ActiveSince, EndedAt: at})
		if a.PlayedNanoseconds == 0 && a.PlayedSeconds > 0 {
			a.PlayedNanoseconds = int64(time.Duration(a.PlayedSeconds) * time.Second)
		}
		a.PlayedNanoseconds += int64(duration)
		a.PlayedSeconds = int64(time.Duration(a.PlayedNanoseconds) / time.Second)
	}
	a.ActiveSince = time.Time{}
	a.UpdatedAt = at
}

func (h History) prune(now time.Time) History {
	h.Version = 1
	cutoff := now.AddDate(0, 0, -RetentionDays)
	kept := make([]Session, 0, len(h.Sessions))
	for _, s := range h.Sessions {
		if !s.EndedAt.Before(cutoff) {
			kept = append(kept, s)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].EndedAt.Before(kept[j].EndedAt) })
	if len(kept) > MaxSessions {
		kept = kept[len(kept)-MaxSessions:]
	}
	h.Sessions = kept
	return h
}

func segmentsDuration(segments []Segment) time.Duration {
	var total time.Duration
	for _, segment := range segments {
		if segment.EndedAt.After(segment.StartedAt) {
			total += segment.EndedAt.Sub(segment.StartedAt)
		}
	}
	return total
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
