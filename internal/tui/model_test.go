package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/iRootPro/lofi-player/internal/audio"
	"github.com/iRootPro/lofi-player/internal/config"
	"github.com/iRootPro/lofi-player/internal/theme"
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
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		next, cmd := m.Update(msg)
		m = next.(Model)
		// Drain immediate cmd output so post-conditions (e.g. ambient
		// SetVolume side effects, which now run via tea.Cmd) actually
		// land before the next assertion. Skips ticks/timers — only
		// dispatches commands whose synchronous result is a non-tick
		// non-key value.
		if cmd != nil {
			drainCmd(t, cmd)
		}
	}
	return m
}

// drainCmd runs cmd synchronously. tea.Batch packs multiple cmds; we
// can't introspect them from outside bubbletea, so we just call the
// outer function — for batch it returns a BatchMsg containing the
// inner cmds, and we execute each.
//
// fixture() builds models with a nil *audio.Player; cmds that touch
// it (setVolumeCmd, pause/resume) panic when forced to run here. In
// production tea.Program owns goroutine lifecycle and never
// dispatches into these on the nil-player path. recover() keeps the
// tests honest about post-conditions without forcing us to fake mpv.
func drainCmd(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	defer func() { _ = recover() }()
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			drainCmd(t, c)
		}
	}
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
	for _, name := range []string{"A", "B", "C"} {
		if !strings.Contains(out, name) {
			t.Errorf("View output missing station %q in list", name)
		}
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

func TestThemePickerPreviewCancelAndConfirm(t *testing.T) {
	m := fixture()
	m = send(t, m, "t")
	if m.mode != modeThemePicker {
		t.Fatalf("mode after t: got %v, want modeThemePicker", m.mode)
	}
	if m.themeBeforePicker != "tokyo-night" {
		t.Fatalf("themeBeforePicker = %q, want tokyo-night", m.themeBeforePicker)
	}

	m = send(t, m, "j")
	if m.theme.Name != "catppuccin-mocha" {
		t.Fatalf("theme after preview down = %q, want catppuccin-mocha", m.theme.Name)
	}
	m = send(t, m, "esc")
	if m.mode != modeFull {
		t.Fatalf("mode after esc: got %v, want modeFull", m.mode)
	}
	if m.theme.Name != "tokyo-night" {
		t.Fatalf("theme after cancel = %q, want tokyo-night", m.theme.Name)
	}

	m = send(t, m, "t", "j", "enter")
	if m.mode != modeFull {
		t.Fatalf("mode after enter: got %v, want modeFull", m.mode)
	}
	if m.theme.Name != "catppuccin-mocha" {
		t.Fatalf("theme after confirm = %q, want catppuccin-mocha", m.theme.Name)
	}
}

func TestThemePickerSavesOnlyOnConfirm(t *testing.T) {
	cfg := &config.Config{Theme: "tokyo-night", Volume: 60, Stations: []config.Station{{Name: "A", URL: "http://a"}}}
	var saved []string
	m := NewModel(cfg, nil, audio.NewAmbientMixer(), Options{
		AutoplayStation: -1,
		SaveTheme: func(name string) error {
			saved = append(saved, name)
			return nil
		},
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)

	m = send(t, m, "t", "j")
	if len(saved) != 0 {
		t.Fatalf("preview saved theme unexpectedly: %v", saved)
	}
	m = send(t, m, "esc")
	if len(saved) != 0 {
		t.Fatalf("cancel saved theme unexpectedly: %v", saved)
	}

	m = send(t, m, "t", "j")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil {
		t.Fatalf("successful theme save returned unexpected command")
	}
	if len(saved) != 1 || saved[0] != "catppuccin-mocha" {
		t.Fatalf("saved themes = %v, want [catppuccin-mocha]", saved)
	}
	if m.mode != modeFull {
		t.Fatalf("mode after confirm: got %v, want modeFull", m.mode)
	}
}

func TestThemePickerViewAndTopChip(t *testing.T) {
	m := fixture()
	out := m.View()
	if !strings.Contains(out, "Tokyo Night") {
		t.Fatalf("View missing active theme chip; got:\n%s", out)
	}

	m = send(t, m, "t")
	out = m.View()
	for _, want := range []string{"themes", "enter", "esc"} {
		if !strings.Contains(out, want) {
			t.Fatalf("theme picker missing %q; got:\n%s", want, out)
		}
	}
	for _, info := range theme.Infos() {
		for _, want := range []string{info.DisplayName, info.Name} {
			if !strings.Contains(out, want) {
				t.Fatalf("theme picker missing %q; got:\n%s", want, out)
			}
		}
	}
}

func TestSettingsModalAdjustSaveAndCancel(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := config.Defaults()
	cfg.Stations = []config.Station{{Name: "A", URL: "http://a"}}
	m := NewModel(&cfg, nil, audio.NewAmbientMixer(), Options{AutoplayStation: -1})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)

	m = send(t, m, "o")
	if m.mode != modeSettings {
		t.Fatalf("mode after o: got %v, want modeSettings", m.mode)
	}
	out := m.View()
	for _, want := range []string{"settings", "network buffer", "initial buffer"} {
		if !strings.Contains(out, want) {
			t.Fatalf("settings view missing %q; got:\n%s", want, out)
		}
	}

	m = send(t, m, "l", "j", "l", "l") // buffer 30→35, initial 0→10
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("saving settings produced no toast command")
	}
	if m.mode != modeFull {
		t.Fatalf("mode after saving settings: got %v, want modeFull", m.mode)
	}
	if cfg.BufferSeconds != 35 || cfg.InitialBufferSeconds != 10 {
		t.Fatalf("saved buffer settings = %d/%d, want 35/10", cfg.BufferSeconds, cfg.InitialBufferSeconds)
	}

	m = send(t, m, "o", "j", "l")
	if m.settingsInitialBufferSeconds != 10 {
		t.Fatalf("initial buffer exceeded cap: got %d, want 10", m.settingsInitialBufferSeconds)
	}

	m = send(t, m, "0", "esc")
	if cfg.BufferSeconds != 35 || cfg.InitialBufferSeconds != 10 {
		t.Fatalf("cancel changed config to %d/%d", cfg.BufferSeconds, cfg.InitialBufferSeconds)
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

func TestPlaybackStartedWhileIdleDoesNotMarkPlaying(t *testing.T) {
	m := fixture()
	if m.playingIdx != -1 {
		t.Fatalf("fixture playingIdx = %d, want -1", m.playingIdx)
	}
	updated, _ := m.Update(PlaybackStartedMsg{})
	m = updated.(Model)
	if m.playing || m.loading || !m.playStartedAt.IsZero() {
		t.Fatalf("idle PlaybackStarted mutated playback state: playing=%v loading=%v started=%v", m.playing, m.loading, m.playStartedAt)
	}
}

func TestBufferingStallReconnectsActiveStream(t *testing.T) {
	m := fixture()
	m.playingIdx = 0
	m.playing = true
	m.loading = false

	updated, cmd := m.Update(BufferingChangedMsg{Stalled: true})
	m = updated.(Model)
	if !m.bufferingStalled || m.reconnectSeq == 0 || cmd == nil {
		t.Fatalf("stall did not arm reconnect: stalled=%v seq=%d cmdNil=%v", m.bufferingStalled, m.reconnectSeq, cmd == nil)
	}

	m.currentTrack = Track{Title: "old"}
	m.streamInfo = audio.StreamInfoChanged{Bitrate: 128000}
	m.cacheSeconds = 3
	m.playStartedAt = time.Now()
	updated, cmd = m.Update(reconnectStreamMsg{seq: m.reconnectSeq})
	m = updated.(Model)
	if !m.loading || m.currentTrack != (Track{}) || m.streamInfo != (audio.StreamInfoChanged{}) || m.cacheSeconds != 0 || !m.playStartedAt.IsZero() || cmd == nil {
		t.Fatalf("reconnect did not reset playback state: loading=%v track=%+v info=%+v cache=%v started=%v cmdNil=%v", m.loading, m.currentTrack, m.streamInfo, m.cacheSeconds, m.playStartedAt, cmd == nil)
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

func TestPressSOpensShareModal(t *testing.T) {
	m := fixture()
	m = send(t, m, "s")
	if m.mode != modeShareStation {
		t.Fatalf("mode after s: got %v, want modeShareStation", m.mode)
	}
	if !strings.Contains(m.shareSnippet, "stations:") || !strings.Contains(m.shareSnippet, "name: A") {
		t.Fatalf("share snippet missing station data:\n%s", m.shareSnippet)
	}
}

func TestShareEscClosesModal(t *testing.T) {
	m := fixture()
	m = send(t, m, "s", "esc")
	if m.mode != modeFull {
		t.Fatalf("mode after s+esc: got %v, want modeFull", m.mode)
	}
	if m.shareSnippet != "" {
		t.Fatalf("shareSnippet not cleared: %q", m.shareSnippet)
	}
}

func TestImportClipboardMsgOpensPreviewAndSkipsDuplicates(t *testing.T) {
	m := fixture()
	updated, _ := m.Update(importClipboardMsg{Stations: []config.Station{
		{Name: "B duplicate", URL: "http://b"},
		{Name: "D", URL: "http://d"},
	}})
	m = updated.(Model)
	if m.mode != modeImportStations {
		t.Fatalf("mode after importClipboardMsg: got %v, want modeImportStations", m.mode)
	}
	if len(m.importStations) != 1 || m.importStations[0].Name != "D" || m.importSkipped != 1 {
		t.Fatalf("import preview = %+v skipped=%d, want D and 1 skipped", m.importStations, m.importSkipped)
	}
}

func TestCommitImportAppendsStations(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := fixture()
	m.mode = modeImportStations
	m.modePrev = modeFull
	m.importStations = []config.Station{{Name: "D", URL: "http://d"}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.mode != modeFull {
		t.Fatalf("mode after commit import: got %v, want modeFull", m.mode)
	}
	if len(m.cfg.Stations) != 4 || m.cfg.Stations[3].Name != "D" {
		t.Fatalf("stations after import = %+v", m.cfg.Stations)
	}
	if m.toast == nil || m.toast.Kind != ToastSuccess {
		t.Fatalf("expected success toast, got %+v", m.toast)
	}
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
	rainCh, _ := am.Channel("rain")
	if !strings.Contains(out, rainCh.Icon) {
		t.Errorf("expected rain icon %q in station line:\n%s", rainCh.Icon, out)
	}
}

func TestStationLineHidesIndicatorWhenSilent(t *testing.T) {
	m := fixture()
	m.playingIdx = 0
	out := m.renderNowPlaying()
	for _, id := range m.mixer.ChannelIDs() {
		ch, _ := m.mixer.Channel(id)
		if strings.Contains(out, ch.Icon) {
			t.Errorf("ambient icon %q (%s) leaked to silent station line:\n%s", ch.Icon, id, out)
		}
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
		SaveAmbient: func(snap map[string]int) error {
			saveCalls++
			lastSnapshot = snap
			return nil
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
	// A tick with no callback should be a quiet no-op — toast remains
	// nil and the seq counter is untouched.
	beforeSeq := m.ambientSaveSeq
	updated, _ := m.Update(ambientSaveTickMsg{seq: 0})
	out := updated.(Model)
	if out.toast != nil {
		t.Errorf("nil callback produced toast: %+v", out.toast)
	}
	if out.ambientSaveSeq != beforeSeq {
		t.Errorf("ambientSaveSeq mutated: got %d, want %d", out.ambientSaveSeq, beforeSeq)
	}
}

func TestAmbientSaveErrorSurfacesAsToast(t *testing.T) {
	cfg := &config.Config{Theme: "tokyo-night", Volume: 60, Stations: []config.Station{{Name: "A"}}}
	restore := audio.SetCacheDirForTest(t.TempDir())
	t.Cleanup(restore)
	am := audio.NewAmbientMixer()
	if err := am.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = am.Close() })

	m := NewModel(cfg, nil, am, Options{
		AutoplayStation: -1,
		SaveAmbient: func(snap map[string]int) error {
			return errors.New("disk full")
		},
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)
	m = send(t, m, "x", "l") // open mixer + one volume bump

	updated, _ = m.Update(ambientSaveTickMsg{seq: m.ambientSaveSeq})
	m = updated.(Model)
	if m.toast == nil || m.toast.Kind != ToastError {
		t.Fatalf("expected error toast, got %+v", m.toast)
	}
	if !strings.Contains(m.toast.Message, "disk full") {
		t.Errorf("toast message missing wrapped err: %q", m.toast.Message)
	}
}

func TestMixerKeyReturnsScheduledTick(t *testing.T) {
	// Sanity check that pressing a mixer key actually returns a non-nil
	// tea.Cmd so the runtime schedules the debounce tick. The handler
	// path is covered separately; this guards the wiring.
	m := fixture()
	m = send(t, m, "x")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if cmd == nil {
		t.Error("mixer key produced nil cmd; expected scheduled debounce tick")
	}
	_ = updated
}
