package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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
	m := NewModel(cfg, nil, Options{AutoplayStation: -1})
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
	m := NewModel(&config.Config{Volume: 60, Stations: []config.Station{{Name: "X"}}}, nil, Options{AutoplayStation: -1})
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
	m = send(t, m, "space") // play station A
	out := m.View()
	// The playing-state indicator is now a ● glyph (Unicode "BLACK
	// CIRCLE") shown both in the now-playing card and beside the
	// currently-playing station in the list.
	if !strings.Contains(out, "●") {
		t.Errorf("expected ● live indicator in view after starting playback; got:\n%s", out)
	}
	if strings.Contains(out, "◯") {
		t.Errorf("did not expect ◯ paused indicator while playing; got:\n%s", out)
	}

	m = send(t, m, "space") // pause
	out = m.View()
	if !strings.Contains(out, "◯") {
		t.Errorf("expected ◯ paused indicator after pause; got:\n%s", out)
	}
}
