package audio

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewAmbientMixerHasThreeChannelsInFixedOrder(t *testing.T) {
	m := NewAmbientMixer()
	got := m.ChannelIDs()
	want := []string{"rain", "fire", "white_noise"}
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
