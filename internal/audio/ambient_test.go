package audio

import (
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestAmbientMixerChannelOrder(t *testing.T) {
	m := NewAmbientMixer()
	got := m.ChannelIDs()
	want := []string{"rain", "fire", "white_noise", "cafe", "thunder"}
	if len(got) != len(want) {
		t.Fatalf("ChannelIDs: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ChannelIDs[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAmbientChannelMetadata(t *testing.T) {
	m := NewAmbientMixer()
	cases := []struct {
		id    string
		label string
	}{
		{"rain", "rain"},
		{"fire", "fire"},
		{"white_noise", "white noise"},
		{"cafe", "cafe"},
		{"thunder", "thunder"},
	}
	for _, c := range cases {
		ch, ok := m.Channel(c.id)
		if !ok {
			t.Fatalf("Channel(%q): not found", c.id)
		}
		if ch.Label != c.label {
			t.Errorf("Channel(%q).Label: got %q, want %q", c.id, ch.Label, c.label)
		}
		if ch.Icon == "" {
			t.Errorf("Channel(%q).Icon: empty", c.id)
		}
	}
}

func withCacheDir(t *testing.T, dir string) {
	t.Helper()
	prev := cacheDirFn
	cacheDirFn = func() (string, error) {
		return filepath.Join(dir, "lofi-player", "ambient"), nil
	}
	t.Cleanup(func() { cacheDirFn = prev })
}

func TestMixerExtractsEmbedToCache(t *testing.T) {
	withCacheDir(t, t.TempDir())

	m := NewAmbientMixer()
	if err := m.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer m.Close()

	cacheDir, _ := m.cacheDir()
	for _, id := range []string{"rain", "fire", "white_noise"} {
		path := filepath.Join(cacheDir, id+".opus")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s on disk: %v", path, err)
		}
	}
}

func TestMixerSkipsExtractIfHashMatches(t *testing.T) {
	cache := t.TempDir()
	withCacheDir(t, cache)

	m1 := NewAmbientMixer()
	if err := m1.Init(); err != nil {
		t.Fatalf("Init #1: %v", err)
	}
	m1.Close()

	target := filepath.Join(cache, "lofi-player", "ambient", "rain.opus")
	info1, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	mtime1 := info1.ModTime()

	backdated := mtime1.Add(-time.Hour)
	if err := os.Chtimes(target, backdated, backdated); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	m2 := NewAmbientMixer()
	if err := m2.Init(); err != nil {
		t.Fatalf("Init #2: %v", err)
	}
	m2.Close()

	info2, _ := os.Stat(target)
	if !info2.ModTime().Equal(backdated) {
		t.Errorf("file was rewritten despite matching hash (mtime changed: %v → %v)", backdated, info2.ModTime())
	}
}

func TestMixerOverwritesIfHashMismatch(t *testing.T) {
	cache := t.TempDir()
	withCacheDir(t, cache)

	dir := filepath.Join(cache, "lofi-player", "ambient")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	target := filepath.Join(dir, "rain.opus")
	bogus := []byte("not the real bytes")
	if err := os.WriteFile(target, bogus, 0o644); err != nil {
		t.Fatalf("write bogus: %v", err)
	}

	m := NewAmbientMixer()
	if err := m.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer m.Close()

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read after init: %v", err)
	}
	if sha256.Sum256(got) == sha256.Sum256(bogus) {
		t.Errorf("file still has bogus content: %q", got)
	}
}

func TestMixerSpawnsMpvSubprocesses(t *testing.T) {
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("mpv not installed")
	}
	withCacheDir(t, t.TempDir())

	m := NewAmbientMixer()
	if err := m.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer m.Close()

	for _, id := range m.ChannelIDs() {
		if m.Disabled(id) {
			t.Errorf("channel %s unexpectedly disabled", id)
		}
	}
}

func TestMixerCloseTerminatesSubprocesses(t *testing.T) {
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("mpv not installed")
	}
	withCacheDir(t, t.TempDir())

	m := NewAmbientMixer()
	if err := m.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestMixerDisabledForUnknownID(t *testing.T) {
	withCacheDir(t, t.TempDir())
	m := NewAmbientMixer()
	if err := m.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer m.Close()
	if m.Disabled("does-not-exist") {
		t.Error("Disabled(unknown) returned true; expected false (treat unknown as not-disabled)")
	}
}

func TestMixerSetVolumeSkippedWhenDisabled(t *testing.T) {
	withCacheDir(t, t.TempDir())
	m := NewAmbientMixer()
	if err := m.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer m.Close()

	// Tests run on systems with or without mpv. Either way, SetVolume
	// must not error: disabled channels silently no-op, working
	// channels accept the volume.
	for _, id := range m.ChannelIDs() {
		if err := m.SetVolume(id, 50); err != nil {
			t.Errorf("SetVolume(%s): %v", id, err)
		}
		if got := m.Volume(id); got != 50 {
			t.Errorf("Volume(%s) after SetVolume(50): got %d, want 50", id, got)
		}
	}
}

func TestMixerSetVolumeUnknownChannel(t *testing.T) {
	withCacheDir(t, t.TempDir())
	m := NewAmbientMixer()
	if err := m.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer m.Close()

	if err := m.SetVolume("does_not_exist", 50); err == nil {
		t.Error("SetVolume(unknown): nil error, want error")
	}
}

func TestMixerSetVolumeClamps(t *testing.T) {
	withCacheDir(t, t.TempDir())
	m := NewAmbientMixer()
	if err := m.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer m.Close()

	if err := m.SetVolume("rain", 200); err != nil {
		t.Fatalf("SetVolume(200): %v", err)
	}
	if got := m.Volume("rain"); got != 100 {
		t.Errorf("clamp high: got %d, want 100", got)
	}
	if err := m.SetVolume("rain", -10); err != nil {
		t.Fatalf("SetVolume(-10): %v", err)
	}
	if got := m.Volume("rain"); got != 0 {
		t.Errorf("clamp low: got %d, want 0", got)
	}
}

func TestMixerVolumeDefaultZero(t *testing.T) {
	withCacheDir(t, t.TempDir())
	m := NewAmbientMixer()
	if err := m.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer m.Close()

	for _, id := range m.ChannelIDs() {
		if got := m.Volume(id); got != 0 {
			t.Errorf("default Volume(%s): got %d, want 0", id, got)
		}
	}
	if got := m.Volume("does_not_exist"); got != 0 {
		t.Errorf("Volume(unknown): got %d, want 0", got)
	}
}

func TestMixerVolumesSnapshot(t *testing.T) {
	withCacheDir(t, t.TempDir())
	m := NewAmbientMixer()
	if err := m.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer m.Close()

	got := m.Volumes()
	for _, id := range m.ChannelIDs() {
		if v := got[id]; v != 0 {
			t.Errorf("default Volumes[%s]: got %d, want 0", id, v)
		}
	}

	_ = m.SetVolume("rain", 40)
	_ = m.SetVolume("white_noise", 25)

	got = m.Volumes()
	if got["rain"] != 40 || got["white_noise"] != 25 || got["fire"] != 0 {
		t.Errorf("Volumes after sets: %+v", got)
	}
}

func TestMixerActiveIDsOrder(t *testing.T) {
	withCacheDir(t, t.TempDir())
	m := NewAmbientMixer()
	if err := m.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer m.Close()

	if got := m.ActiveIDs(); len(got) != 0 {
		t.Errorf("default ActiveIDs: %v, want empty", got)
	}

	_ = m.SetVolume("white_noise", 10)
	_ = m.SetVolume("rain", 5)

	got := m.ActiveIDs()
	want := []string{"rain", "white_noise"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("ActiveIDs: got %v, want %v", got, want)
	}
}
