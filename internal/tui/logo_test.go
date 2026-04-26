package tui

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/iRootPro/lofi-player/internal/config"
)

func TestLogo_AdvanceIncrementsTick(t *testing.T) {
	var l logo
	for range 5 {
		l.advance()
	}
	if l.tick != 5 {
		t.Errorf("tick after 5 advances: got %d, want 5", l.tick)
	}
}

func TestLogo_CrestColumnSweepsAndWraps(t *testing.T) {
	var l logo
	width := lipgloss.Width(logoLines[0])
	period := width + logoShimmerHalo*2
	first := l.crestColumn(width)
	for range period {
		l.advance()
	}
	if got := l.crestColumn(width); got != first {
		t.Errorf("crest after one full period: got %d, want %d", got, first)
	}
}

func TestRenderLogo_EmptyWhenNoStation(t *testing.T) {
	m := fixture()
	if got := m.renderLogo(); got != "" {
		t.Errorf("renderLogo with no station: got %q, want empty", got)
	}
}

func TestRenderLogo_LineCountAndWidth(t *testing.T) {
	m := fixture()
	m.playingIdx = 0
	out := m.renderLogo()
	if out == "" {
		t.Fatal("renderLogo returned empty while playingIdx=0")
	}
	lines := strings.Split(out, "\n")
	if len(lines) != len(logoLines) {
		t.Fatalf("line count: got %d, want %d", len(lines), len(logoLines))
	}
	want := lipgloss.Width(logoLines[0])
	for i, line := range lines {
		if got := lipgloss.Width(line); got != want {
			t.Errorf("line %d width: got %d, want %d", i, got, want)
		}
	}
}

func TestUpdate_LogoTickAdvancesOnlyWhenPlaying(t *testing.T) {
	m := fixture()
	startTick := m.logo.tick

	updated, cmd := m.Update(logoTickMsg{})
	m = updated.(Model)
	if m.logo.tick != startTick {
		t.Errorf("paused: tick advanced from %d to %d", startTick, m.logo.tick)
	}
	if cmd == nil {
		t.Error("logoTickMsg should always re-arm the tick, got nil cmd")
	}

	m.playing = true
	updated, _ = m.Update(logoTickMsg{})
	m = updated.(Model)
	if m.logo.tick != startTick+1 {
		t.Errorf("playing: tick should advance by 1, got %d (start was %d)", m.logo.tick, startTick)
	}
}

func TestView_LogoRendersInFullView(t *testing.T) {
	m := fixture()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 110, Height: 24})
	m = updated.(Model)
	m.playingIdx = 0
	m.playing = true

	out := m.View()
	// Pick a glyph that is unique to the logo (the rounded corner
	// `╰` only appears in the logo's L letter and the logo's O,
	// plus the app frame's bottom-left corner — all paths produce
	// at least one).
	if !strings.Contains(out, "╰────") {
		t.Errorf("view does not contain the logo's `╰────` segment; got:\n%s", out)
	}
}

// TestLogoVisual prints rendered frames to stdout when EQ_VISUAL is
// set — purely a development aid for verifying the shimmer looks
// right against the now-playing block. Skipped by default.
func TestLogoVisual(t *testing.T) {
	if os.Getenv("EQ_VISUAL") == "" {
		t.Skip("set EQ_VISUAL=1 to dump rendered frames")
	}
	cfg := &config.Config{
		Theme:  "tokyo-night",
		Volume: 60,
		Stations: []config.Station{
			{Name: "Radio Paradise Mellow", URL: "http://x", Kind: config.KindStream},
		},
	}
	m := NewModel(cfg, nil, Options{AutoplayStation: -1})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 110, Height: 24})
	m = updated.(Model)
	m.playingIdx = 0
	m.playing = true
	m.currentTrack = Track{Title: "Wicked Game", Artist: "Chris Isaak"}

	for frame := range 8 {
		fmt.Fprintf(os.Stdout, "\n=== frame %d (tick=%d) ===\n%s\n", frame, m.logo.tick, m.View())
		for range 3 {
			m.logo.advance()
		}
	}
}
