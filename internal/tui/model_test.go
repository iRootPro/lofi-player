package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/iRootPro/lofi-player/internal/audio"
	"github.com/iRootPro/lofi-player/internal/config"
)

// fixture returns a Model wired up with a real config and a sized window,
// ready for keypress-driven assertions. The Player is intentionally nil:
// these tests cover state transitions inside Update and never invoke the
// tea.Cmd values returned alongside (which would otherwise call player
// methods).
func fixture() Model {
	cfg := &config.Config{
		Theme:  "tokyo-night",
		Volume: 60,
		Stations: []config.Station{
			{Name: "A", URL: "http://a"},
			{Name: "B", URL: "http://b"},
			{Name: "C", URL: "http://c"},
		},
	}
	m := NewModel(cfg, nil, audio.NewAmbientMixer(), Options{AutoplayStation: -1})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return updated.(Model)
}

func send(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "space":
			msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	return m
}

func TestView_RendersWhenSized(t *testing.T) {
	m := fixture()
	out := m.View()
	if out == "" {
		t.Fatal("View() returned empty string for sized window")
	}
	if !strings.Contains(out, "lofi.player") {
		t.Errorf("View output missing app title; got:\n%s", out)
	}
	if !strings.Contains(out, "stations") {
		t.Errorf("View output missing stations section header")
	}
}

func TestView_EmptyBeforeFirstWindowSize(t *testing.T) {
	m := NewModel(&config.Config{Volume: 60, Stations: []config.Station{{Name: "X"}}}, nil, audio.NewAmbientMixer(), Options{AutoplayStation: -1})
	if got := m.View(); got != "" {
		t.Errorf("View() should be empty before WindowSizeMsg, got %q", got)
	}
}

func TestUpdate_VolumeClamps(t *testing.T) {
	m := fixture()
	// Volume starts at 60. 9 increments of +5 should cap at 100, not go past.
	m = send(t, m, "+", "+", "+", "+", "+", "+", "+", "+", "+", "+", "+", "+")
	if m.volume != 100 {
		t.Errorf("volume after many +: got %d, want 100", m.volume)
	}
	// 25 decrements of -5 should cap at 0.
	for range 25 {
		m = send(t, m, "-")
	}
	if m.volume != 0 {
		t.Errorf("volume after many -: got %d, want 0", m.volume)
	}
}

func TestUpdate_CursorBounds(t *testing.T) {
	m := fixture()
	if m.cursor != 0 {
		t.Fatalf("initial cursor: got %d, want 0", m.cursor)
	}
	// Up at top is a no-op.
	m = send(t, m, "k")
	if m.cursor != 0 {
		t.Errorf("up at top: cursor moved to %d", m.cursor)
	}
	// Walk to the bottom (3 stations: indices 0..2).
	m = send(t, m, "j", "j", "j", "j")
	if m.cursor != 2 {
		t.Errorf("cursor at bottom: got %d, want 2", m.cursor)
	}
}

func TestUpdate_SpaceTogglesAndSwitches(t *testing.T) {
	m := fixture()
	// Space on cursor=0 starts playing station 0.
	m = send(t, m, "space")
	if !m.playing || m.playingIdx != 0 {
		t.Errorf("after first space: playing=%v idx=%d, want true 0", m.playing, m.playingIdx)
	}
	// Space again on the same station pauses.
	m = send(t, m, "space")
	if m.playing || m.playingIdx != 0 {
		t.Errorf("after second space: playing=%v idx=%d, want false 0", m.playing, m.playingIdx)
	}
	// Move cursor and press space — switches to new station and starts playing.
	m = send(t, m, "j", "space")
	if !m.playing || m.playingIdx != 1 {
		t.Errorf("after switching stations: playing=%v idx=%d, want true 1", m.playing, m.playingIdx)
	}
}

func TestUpdate_HelpToggle(t *testing.T) {
	m := fixture()
	if m.showFullHelp {
		t.Fatal("showFullHelp should default to false")
	}
	m = send(t, m, "?")
	if !m.showFullHelp {
		t.Error("? did not enable full help")
	}
	m = send(t, m, "?")
	if m.showFullHelp {
		t.Error("? did not disable full help")
	}
}

func TestUpdate_QuitReturnsTeaQuit(t *testing.T) {
	m := fixture()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("q produced no command, expected tea.Quit")
	}
	// tea.Quit is a tea.Cmd; calling it returns tea.QuitMsg{}.
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("q produced %T, want tea.QuitMsg", cmd())
	}
}

func TestView_ShowsPlayingMarker(t *testing.T) {
	m := fixture()
	m = send(t, m, "space") // dispatch play; model goes into loading state

	if !m.loading {
		t.Errorf("expected loading=true immediately after space, got false")
	}

	// Simulate mpv's PlaybackStarted event so loading clears and the
	// status indicator transitions from spinner → ● glyph.
	updated, _ := m.Update(PlaybackStartedMsg{})
	m = updated.(Model)

	out := m.View()
	if !strings.Contains(out, "●") {
		t.Errorf("expected ● live indicator in view after PlaybackStarted; got:\n%s", out)
	}
	if strings.Contains(out, "◯") {
		t.Errorf("did not expect ◯ paused indicator while playing; got:\n%s", out)
	}

	m = send(t, m, "space") // toggle pause on the same station
	updated, _ = m.Update(PlaybackPausedMsg{})
	m = updated.(Model)

	out = m.View()
	if !strings.Contains(out, "◯") {
		t.Errorf("expected ◯ paused indicator after pause; got:\n%s", out)
	}
}

func TestPressXOpensMixer(t *testing.T) {
	m := fixture()
	m = send(t, m, "x")
	if m.mode != modeMixer {
		t.Errorf("mode after x: got %v, want modeMixer", m.mode)
	}
}

func TestPressEscClosesMixer(t *testing.T) {
	m := fixture()
	m = send(t, m, "x", "esc")
	if m.mode != modeFull {
		t.Errorf("mode after x+esc: got %v, want modeFull", m.mode)
	}
}

func TestPressXAgainClosesMixer(t *testing.T) {
	m := fixture()
	m = send(t, m, "x", "x")
	if m.mode != modeFull {
		t.Errorf("mode after x+x: got %v, want modeFull", m.mode)
	}
}

func TestKeyMapHasMixerOpenX(t *testing.T) {
	km := DefaultKeyMap()
	for _, k := range km.MixerOpen.Keys() {
		if k == "x" {
			return
		}
	}
	t.Error("MixerOpen does not include 'x'")
}

func TestStationLineShowsActiveAmbient(t *testing.T) {
	restore := audio.SetCacheDirForTest(t.TempDir())
	t.Cleanup(restore)
	am := audio.NewAmbientMixer()
	if err := am.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = am.Close() })

	cfg := &config.Config{
		Theme:    "tokyo-night",
		Volume:   60,
		Stations: []config.Station{{Name: "A", URL: "http://a"}},
	}
	m := NewModel(cfg, nil, am, Options{AutoplayStation: -1})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)
	m.playingIdx = 0
	_ = am.SetVolume("rain", 40)

	out := m.renderNowPlaying()
	if !strings.Contains(out, "🌧️") {
		t.Errorf("expected rain icon in station line:\n%s", out)
	}
}

func TestStationLineHidesIndicatorWhenSilent(t *testing.T) {
	m := fixture()
	m.playingIdx = 0
	out := m.renderNowPlaying()
	if strings.Contains(out, "🌧️") || strings.Contains(out, "🔥") || strings.Contains(out, "⚪") {
		t.Errorf("ambient icons leaked to silent station line:\n%s", out)
	}
}

func TestGlobalKeysDisabledInMixerModal(t *testing.T) {
	m := fixture()
	m = send(t, m, "x") // open mixer
	if m.mode != modeMixer {
		t.Fatalf("setup: mode = %v, want modeMixer", m.mode)
	}

	// Volume key — must not change m.volume while modal is open.
	originalVolume := m.volume
	m = send(t, m, "+")
	if m.volume != originalVolume {
		t.Errorf("global '+' fired in mixer modal: %d -> %d", originalVolume, m.volume)
	}

	// Theme cycle — must not swap theme.
	originalTheme := m.theme.Name
	m = send(t, m, "t")
	if m.theme.Name != originalTheme {
		t.Errorf("global 't' fired in mixer modal: %s -> %s", originalTheme, m.theme.Name)
	}

	// Help — must not toggle.
	originalHelp := m.showFullHelp
	m = send(t, m, "?")
	if m.showFullHelp != originalHelp {
		t.Errorf("global '?' fired in mixer modal: %v -> %v", originalHelp, m.showFullHelp)
	}

	// AddStation — must not switch mode away from modeMixer.
	m = send(t, m, "a")
	if m.mode != modeMixer {
		t.Errorf("global 'a' fired in mixer modal: mode = %v", m.mode)
	}
}

func TestAmbientSaveDebounceTickCoalesces(t *testing.T) {
	saveCalls := 0
	var lastSnapshot map[string]int
	cfg := &config.Config{
		Theme:    "tokyo-night",
		Volume:   60,
		Stations: []config.Station{{Name: "A", URL: "http://a"}},
	}

	restore := audio.SetCacheDirForTest(t.TempDir())
	t.Cleanup(restore)
	am := audio.NewAmbientMixer()
	if err := am.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = am.Close() })

	m := NewModel(cfg, nil, am, Options{
		AutoplayStation: -1,
		SaveAmbient: func(snap map[string]int) {
			saveCalls++
			lastSnapshot = snap
		},
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)

	// Two volume changes inside the modal.
	m = send(t, m, "x") // open mixer
	m = send(t, m, "l") // bump rain to 5
	m = send(t, m, "l") // bump rain to 10

	// Stale tick (older seq) — must not save.
	m1, _ := m.Update(ambientSaveTickMsg{seq: 1})
	m = m1.(Model)
	if saveCalls != 0 {
		t.Errorf("stale tick fired save: %d calls, want 0", saveCalls)
	}

	// Latest tick — should fire one save with current snapshot.
	m1, _ = m.Update(ambientSaveTickMsg{seq: m.ambientSaveSeq})
	m = m1.(Model)
	if saveCalls != 1 {
		t.Fatalf("save count after fresh tick: got %d, want 1", saveCalls)
	}
	if lastSnapshot["rain"] != 10 {
		t.Errorf("snapshot: got %+v, want rain=10", lastSnapshot)
	}
}

func TestAmbientSaveSkippedWhenCallbackNil(t *testing.T) {
	m := fixture()
	// fixture() builds NewModel with Options that don't set SaveAmbient.
	// A tick with no callback should be a quiet no-op.
	updated, _ := m.Update(ambientSaveTickMsg{seq: 0})
	if updated == nil {
		t.Error("Update returned nil model")
	}
}
