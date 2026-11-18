package history

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func at(hour, minute int) time.Time {
	return time.Date(2026, time.November, 17, hour, minute, 0, 0, time.UTC)
}

func TestHistoryPausesDoNotCountAsListening(t *testing.T) {
	h := History{}.Begin("Station", SourceLibrary, at(10, 0))
	h = h.Pause(at(10, 20))
	h = h.Begin("Station", SourceLibrary, at(10, 50))
	h = h.End(at(11, 5), EndPaused)

	if len(h.Sessions) != 1 {
		t.Fatalf("sessions = %d, want 1", len(h.Sessions))
	}
	if got, want := h.Sessions[0].PlayedSeconds, int64(35*60); got != want {
		t.Errorf("played seconds = %d, want %d", got, want)
	}
	if got, want := len(h.Sessions[0].Segments), 2; got != want {
		t.Errorf("segments = %d, want %d", got, want)
	}
}

func TestHistorySwitchFinalizesPreviousStation(t *testing.T) {
	h := History{}.Begin("A", SourceLibrary, at(10, 0))
	h = h.Begin("B", SourceDiscovery, at(10, 5))
	h = h.End(at(10, 8), EndPaused)

	if got, want := len(h.Sessions), 2; got != want {
		t.Fatalf("sessions = %d, want %d", got, want)
	}
	if h.Sessions[0].StationName != "A" || h.Sessions[0].EndedReason != EndSwitched {
		t.Errorf("first session = %+v", h.Sessions[0])
	}
	if h.Sessions[1].StationName != "B" || h.Sessions[1].Source != SourceDiscovery {
		t.Errorf("second session = %+v", h.Sessions[1])
	}
}

func TestHistoryDropsShortListen(t *testing.T) {
	h := History{}.Begin("Station", SourceLibrary, at(10, 0)).End(at(10, 0).Add(29*time.Second), EndPaused)
	if len(h.Sessions) != 0 {
		t.Errorf("short listen was retained: %+v", h.Sessions)
	}
}

func TestHistoryKeepsThirtySecondsAcrossFractionalSegments(t *testing.T) {
	start := at(10, 0)
	h := History{}.Begin("Station", SourceLibrary, start)
	h = h.Pause(start.Add(29*time.Second + 900*time.Millisecond))
	h = h.Begin("Station", SourceLibrary, start.Add(time.Minute))
	h = h.End(start.Add(time.Minute).Add(200*time.Millisecond), EndPaused)
	if got, want := len(h.Sessions), 1; got != want {
		t.Fatalf("sessions = %d, want %d; fractional time was lost", got, want)
	}
	if got, want := h.Sessions[0].PlayedSeconds, int64(30); got != want {
		t.Errorf("display seconds = %d, want %d", got, want)
	}
}

func TestHistoryDurationBetweenSplitsAtMidnight(t *testing.T) {
	start := time.Date(2026, time.November, 17, 23, 50, 0, 0, time.UTC)
	end := start.Add(20 * time.Minute)
	h := History{}.Begin("Station", SourceLibrary, start).End(end, EndPaused)
	from := time.Date(2026, time.November, 18, 0, 0, 0, 0, time.UTC)
	if got, want := h.DurationBetween(from, from.Add(24*time.Hour), end), 10*time.Minute; got != want {
		t.Errorf("duration after midnight = %s, want %s", got, want)
	}
}

func TestHistoryRecoverUsesCheckpointNotRestartTime(t *testing.T) {
	h := History{}.Begin("Station", SourceLibrary, at(10, 0))
	h = h.Checkpoint(at(10, 12))
	recovered, changed := h.Recover()
	if !changed || recovered.Active != nil || len(recovered.Sessions) != 1 {
		t.Fatalf("recovery = %+v, changed=%v", recovered, changed)
	}
	if got, want := recovered.Sessions[0].EndedAt, at(10, 12); !got.Equal(want) {
		t.Errorf("recovered end = %s, want %s", got, want)
	}
}

func TestHistoryRecoverUsesExactLegacySegments(t *testing.T) {
	start := at(10, 0)
	h := History{Version: 1, Active: &ActiveSession{
		StationName:   "Station",
		Source:        SourceLibrary,
		StartedAt:     start,
		UpdatedAt:     start.Add(time.Minute),
		PlayedSeconds: 29, // truncated aggregate from the old format
		Segments: []Segment{
			{StartedAt: start, EndedAt: start.Add(29*time.Second + 900*time.Millisecond)},
			{StartedAt: start.Add(50 * time.Second), EndedAt: start.Add(50*time.Second + 200*time.Millisecond)},
		},
	}}
	recovered, changed := h.Recover()
	if !changed || len(recovered.Sessions) != 1 {
		t.Fatalf("legacy fractional session was dropped: %+v", recovered)
	}
}

func TestHistoryClearRestartsActiveListen(t *testing.T) {
	h := History{}.Begin("Station", SourceLibrary, at(10, 0)).End(at(10, 5), EndPaused)
	h = h.Begin("Live", SourceLibrary, at(11, 0))
	h = h.Clear(at(11, 2))
	if len(h.Sessions) != 0 || h.Active == nil || !h.Active.StartedAt.Equal(at(11, 2)) {
		t.Errorf("clear = %+v", h)
	}
}

func TestHistoryClearWhilePausedDoesNotStartListening(t *testing.T) {
	h := History{}.Begin("Station", SourceLibrary, at(10, 0)).Pause(at(10, 5))
	h = h.Clear(at(10, 10))
	if h.Active == nil || !h.Active.ActiveSince.IsZero() {
		t.Fatalf("clear while paused resumed listening: %+v", h.Active)
	}
	h = h.Begin("Station", SourceLibrary, at(10, 30)).End(at(10, 31), EndPaused)
	if got, want := len(h.Sessions), 1; got != want {
		t.Fatalf("sessions = %d, want %d", got, want)
	}
	if got, want := h.Sessions[0].PlayedSeconds, int64(60); got != want {
		t.Errorf("played seconds = %d, want %d", got, want)
	}
}

func TestHistoryRoundTripAndPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deep", "history.json")
	want := History{}.Begin("Station", SourceLibrary, at(10, 0)).End(at(10, 2), EndPaused)
	if err := saveToFile(path, want); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := loadFromFile(path); !reflect.DeepEqual(got, want) {
		t.Errorf("round trip\n got:  %+v\n want: %+v", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("history permissions = %o, want 600", got)
	}
}

func TestPathRespectsXDGStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/state-override")
	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if want := "/tmp/state-override/lofi-player/history.json"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}
