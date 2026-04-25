// Package state persists the user's last session — last station, volume,
// theme, plus a forward-compatible Pomodoro blob filled in Phase 3.
//
// Persistence is best-effort (plan §6 Phase 2 pitfall): both Load and
// Save swallow filesystem errors and return safe defaults rather than
// surfacing them to callers. A missing or malformed state file is
// indistinguishable from a fresh install on purpose.
package state

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
)

// State is the persisted slice of UI session state.
type State struct {
	// Theme is the active palette name on last shutdown.
	Theme string `json:"theme,omitempty"`
	// Volume is the playback volume on last shutdown, 0..100.
	Volume int `json:"volume,omitempty"`
	// LastStationName is the display name of the last actively-played
	// station; the loader matches it back to an index in cfg.Stations
	// so renaming a station doesn't break the autoplay-on-startup path.
	LastStationName string `json:"last_station_name,omitempty"`
	// Pomodoro is opaque storage for Phase 3's pomodoro stats. Phase 2
	// preserves whatever's there on read/write so an upgrade doesn't
	// drop session history.
	Pomodoro json.RawMessage `json:"pomodoro,omitempty"`
}

// Path returns the canonical XDG state path, creating any missing parent
// directories along the way. Errors here are not fatal — callers should
// fall back to in-memory operation if Path returns one.
func Path() (string, error) {
	p, err := xdg.StateFile("lofi-player/state.json")
	if err != nil {
		return "", err
	}
	return p, nil
}

// Load reads the persisted state from the canonical XDG path. A missing
// or unreadable file yields an empty State without an error — see the
// package documentation for the rationale.
func Load() *State {
	p, err := Path()
	if err != nil {
		return &State{}
	}
	return loadFromFile(p)
}

// Save writes s to the canonical XDG path. Errors are returned for the
// caller to log but should not interrupt normal application flow.
func Save(s *State) error {
	p, err := Path()
	if err != nil {
		return err
	}
	return saveToFile(p, s)
}

func loadFromFile(path string) *State {
	data, err := os.ReadFile(path)
	if err != nil {
		// Missing file or any other read error → fresh state.
		return &State{}
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		// Malformed file → fresh state. Don't crash, don't lose data
		// either: the broken file stays on disk for the user to inspect.
		return &State{}
	}
	return &s
}

// saveToFile writes the JSON-encoded state via the standard
// tempfile-then-rename atomic pattern, so a crash mid-write never
// produces a half-written state file.
func saveToFile(path string, s *State) error {
	if s == nil {
		return errors.New("nil state")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".state-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

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

// FileExists is a tiny helper for tests and main wiring; returns true
// only if a regular file is present at path.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return !errors.Is(err, fs.ErrNotExist) && false
	}
	return !info.IsDir()
}
