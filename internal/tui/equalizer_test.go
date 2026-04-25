package tui

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/iRootPro/lofi-player/internal/config"
)

func TestEqualizer_NewHasExpectedShape(t *testing.T) {
	e := newEqualizer()
	if got := len(e.bars); got != equalizerBarCount {
		t.Fatalf("bar count: got %d, want %d", got, equalizerBarCount)
	}
	for i, b := range e.bars {
		if b.speed < 0.20 || b.speed > 0.45 {
			t.Errorf("bar %d speed %g out of [0.20, 0.45]", i, b.speed)
		}
	}
}

func TestEqualizer_AdvanceAddsSpeedToPhase(t *testing.T) {
	e := equalizer{bars: []equalizerBar{
		{phase: 0, speed: 0.3},
		{phase: 1, speed: 0.1},
	}}
	e.advance()
	e.advance()
	e.advance()
	if got := e.bars[0].phase; math.Abs(got-0.9) > 1e-9 {
		t.Errorf("bar 0 phase after 3 advances: got %g, want ~0.9", got)
	}
	if got := e.bars[1].phase; math.Abs(got-1.3) > 1e-9 {
		t.Errorf("bar 1 phase after 3 advances: got %g, want ~1.3", got)
	}
}

func TestEqualizer_HeightsBounded(t *testing.T) {
	e := newEqualizer()
	for tick := range 200 {
		for i, h := range e.heights(false) {
			if h < 0 || h > equalizerMaxHeight {
				t.Fatalf("tick %d bar %d height %d out of [0, %d]", tick, i, h, equalizerMaxHeight)
			}
		}
		e.advance()
	}
}

func TestEqualizer_LoadingSquashesAmplitude(t *testing.T) {
	e := newEqualizer()
	for tick := range 200 {
		for i, h := range e.heights(true) {
			if h < 0 || h > 2 {
				t.Fatalf("loading tick %d bar %d height %d out of [0, 2]", tick, i, h)
			}
		}
		e.advance()
	}
}

func TestEqualizer_GlyphsMapping(t *testing.T) {
	cases := []struct {
		h          int
		wantTop    rune
		wantBottom rune
	}{
		{0, ' ', ' '},
		{1, ' ', '▁'},
		{4, ' ', '▄'},
		{5, '▁', '█'},
		{8, '▄', '█'},
	}
	for _, c := range cases {
		gotTop, gotBot := equalizerGlyphs(c.h)
		if gotTop != c.wantTop || gotBot != c.wantBottom {
			t.Errorf("equalizerGlyphs(%d) = (%q,%q), want (%q,%q)",
				c.h, gotTop, gotBot, c.wantTop, c.wantBottom)
		}
	}
}

func TestRenderEqualizer_EmptyWhenNoStation(t *testing.T) {
	m := fixture()
	if m.playingIdx != -1 {
		t.Fatalf("fixture should start with playingIdx=-1, got %d", m.playingIdx)
	}
	if got := m.renderEqualizer(); got != "" {
		t.Errorf("renderEqualizer with no station: got %q, want empty", got)
	}
}

func TestRenderEqualizer_TwoLinesWithExpectedWidth(t *testing.T) {
	m := fixture()
	m.playingIdx = 0
	out := m.renderEqualizer()
	if out == "" {
		t.Fatal("renderEqualizer returned empty while playingIdx=0")
	}
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("renderEqualizer line count: got %d, want 2 — output:\n%s", len(lines), out)
	}
	wantWidth := 2*equalizerBarCount - 1
	for i, line := range lines {
		if got := lipgloss.Width(line); got != wantWidth {
			t.Errorf("line %d width: got %d, want %d (line=%q)", i, got, wantWidth, line)
		}
	}
}

func TestUpdate_EqualizerTickAdvancesOnlyWhenPlaying(t *testing.T) {
	m := fixture()
	// While not playing, advance() must not run — phases stay frozen.
	frozen := append([]equalizerBar(nil), m.eq.bars...)
	updated, cmd := m.Update(equalizerTickMsg{})
	m = updated.(Model)
	for i, b := range m.eq.bars {
		if b.phase != frozen[i].phase {
			t.Errorf("paused: bar %d phase changed from %g to %g", i, frozen[i].phase, b.phase)
		}
	}
	if cmd == nil {
		t.Error("equalizerTickMsg should always re-arm the tick, got nil cmd")
	}

	// Flip to playing and tick again — phases must advance.
	m.playing = true
	before := append([]equalizerBar(nil), m.eq.bars...)
	updated, _ = m.Update(equalizerTickMsg{})
	m = updated.(Model)
	for i, b := range m.eq.bars {
		want := before[i].phase + before[i].speed
		if math.Abs(b.phase-want) > 1e-9 {
			t.Errorf("playing: bar %d phase: got %g, want %g", i, b.phase, want)
		}
	}
}

func TestView_EqualizerJoinsNowPlaying(t *testing.T) {
	m := fixture()
	// Force a wide enough window so renderTopBlock keeps the equalizer.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 110, Height: 24})
	m = updated.(Model)
	m.playingIdx = 0
	m.playing = true

	out := m.View()
	if !strings.Contains(out, "█") && !strings.Contains(out, "▁") &&
		!strings.Contains(out, "▂") && !strings.Contains(out, "▃") &&
		!strings.Contains(out, "▄") {
		t.Errorf("view does not contain any equalizer glyph; got:\n%s", out)
	}
}

// TestEqualizerVisual prints a few rendered frames of the full View
// to stdout when the EQ_VISUAL env var is set — purely a development
// aid for verifying the bar layout looks right next to now-playing.
// Skipped by default so CI stays quiet.
func TestEqualizerVisual(t *testing.T) {
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
	m.currentTrack = Track{Title: "Don't Get Your Back Up", Artist: "Sarah Harmer"}

	for frame := range 5 {
		fmt.Fprintf(os.Stdout, "\n=== frame %d ===\n%s\n", frame, m.View())
		for range 3 {
			m.eq.advance()
		}
	}
}
