package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPath_RespectsXDGStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/state-override")
	got, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	want := "/tmp/state-override/lofi-player/state.json"
	if got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestPath_FallsBackToHomeLocalState(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	got, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".local", "state", "lofi-player", "state.json")
	if got != want {
		t.Errorf("Path() = %q, want %q (no Library/Application Support on macOS)", got, want)
	}
}

func TestLoadFromFile_MissingFileYieldsZero(t *testing.T) {
	dir := t.TempDir()
	got := loadFromFile(filepath.Join(dir, "absent.json"))
	if !reflect.DeepEqual(*got, State{}) {
		t.Errorf("missing file: got %+v, want zero State", *got)
	}
}

func TestLoadFromFile_MalformedJSONYieldsZero(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.json")
	if err := os.WriteFile(path, []byte("not json at all"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got := loadFromFile(path)
	if !reflect.DeepEqual(*got, State{}) {
		t.Errorf("malformed file: got %+v, want zero State", *got)
	}
	// Broken file should NOT be deleted — user should be able to inspect it.
	if _, err := os.Stat(path); err != nil {
		t.Errorf("malformed file removed: %v", err)
	}
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	original := &State{
		Theme:           "catppuccin-mocha",
		Volume:          42,
		LastStationName: "SomaFM Drone Zone",
		Pomodoro:        json.RawMessage(`{"streak":3}`),
	}
	if err := saveToFile(path, original); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := loadFromFile(path)

	if got.Theme != original.Theme {
		t.Errorf("Theme: got %q, want %q", got.Theme, original.Theme)
	}
	if got.Volume != original.Volume {
		t.Errorf("Volume: got %d, want %d", got.Volume, original.Volume)
	}
	if got.LastStationName != original.LastStationName {
		t.Errorf("LastStationName: got %q, want %q", got.LastStationName, original.LastStationName)
	}
	if !sameJSON(got.Pomodoro, original.Pomodoro) {
		t.Errorf("Pomodoro blob mismatch\n got:  %s\n want: %s", got.Pomodoro, original.Pomodoro)
	}
}

func TestSaveCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deep", "nested", "state.json")
	if err := saveToFile(path, &State{Volume: 33}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("state file not at %q: %v", path, err)
	}
}

func TestSaveAtomic_NoPartialFileVisible(t *testing.T) {
	// We can't easily simulate a crash mid-write, but we can verify
	// that the rename target is the only .json file with the user's
	// content — no temp file left behind on success.
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := saveToFile(path, &State{Theme: "rose-pine"}); err != nil {
		t.Fatalf("save: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "state.json" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected only state.json in dir, got %v", names)
	}
}

func TestSave_NilStateRejected(t *testing.T) {
	if err := saveToFile("/tmp/whatever", nil); err == nil {
		t.Error("save(nil) returned no error")
	}
}

func TestPomodoroBlobPreserved(t *testing.T) {
	// Even though Phase 2 doesn't interpret Pomodoro, Save+Load must
	// preserve its semantic content (Phase 3 will parse it). The
	// formatting may change because MarshalIndent reformats the bytes,
	// but the JSON value must still parse to the same structure.
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	pomo := json.RawMessage(`{"streak":7,"sessions_today":3}`)
	if err := saveToFile(path, &State{Pomodoro: pomo}); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := loadFromFile(path)
	if !sameJSON(got.Pomodoro, pomo) {
		t.Errorf("Pomodoro blob mismatch\n got:  %s\n want: %s", got.Pomodoro, pomo)
	}
}

// sameJSON returns true when a and b decode to equal Go values, ignoring
// whitespace and key ordering differences.
func sameJSON(a, b json.RawMessage) bool {
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	return reflect.DeepEqual(av, bv)
}
