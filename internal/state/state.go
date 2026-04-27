// Package state persists the user's last session — last station, volume,
// theme, plus a forward-compatible Pomodoro blob filled in Phase 3.
//
// State files live at $XDG_STATE_HOME/lofi-player/state.json — and when
// XDG_STATE_HOME isn't set, at $HOME/.local/state/lofi-player/state.json
// on both Linux and macOS. Plan §9 prefers the XDG-style path over the
// macOS-native ~/Library/Application Support for consistency with
// other terminal tools.
//
// Persistence is best-effort (plan §6 Phase 2 pitfall): both Load and
// Save swallow filesystem errors and return safe defaults rather than
// surfacing them to callers. A missing or malformed state file is
// indistinguishable from a fresh install on purpose.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
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
	// Ambient holds per-channel volumes for the ambient mixer keyed
	// by channel id (e.g. {"rain": 40}). Unknown keys round-trip
	// untouched so older builds don't drop channels added by newer ones.
	Ambient map[string]int `json:"ambient,omitempty"`
	// ShowStreamInfo persists the visibility toggle of the
	// stream-info row under the now-playing card. Pointer so
	// omitempty works for an explicit `false` (zero value of bool
	// would otherwise be indistinguishable from "not set").
	ShowStreamInfo *bool `json:"show_stream_info,omitempty"`
}

// Path returns the canonical path to the state file. It honors
// $XDG_STATE_HOME when set; otherwise falls back to $HOME/.local/state/
// on both Linux and macOS (plan §9). The parent directory is created
// on demand by Save.
func Path() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "lofi-player", "state.json"), nil
}

// Load reads the persisted state from the canonical XDG path. A missing
// or unreadable file yields an empty State without an error — see the
// package documentation for the rationale. On macOS, a one-time
// migration moves any pre-existing state from the legacy
// ~/Library/Application Support/lofi-player/ location to the new
// XDG-style path.
func Load() *State {
	p, err := Path()
	if err != nil {
		return &State{}
	}
	migrateLegacyMacOS(p)
	return loadFromFile(p)
}

// migrateLegacyMacOS moves state.json from the macOS-native
// ~/Library/Application Support/lofi-player/ location to the new
// XDG-style path when the new path is empty. Idempotent, no-op on
// Linux, errors swallowed.
func migrateLegacyMacOS(newPath string) {
	if runtime.GOOS != "darwin" {
		return
	}
	if _, err := os.Stat(newPath); err == nil {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	legacy := filepath.Join(home, "Library", "Application Support", "lofi-player", "state.json")
	if _, err := os.Stat(legacy); err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		return
	}
	_ = os.Rename(legacy, newPath)
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
