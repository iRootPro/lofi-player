package tui

import "math"

// equalizerBarCount is the number of vertical bars rendered next to
// the now-playing block. Each bar occupies one cell of width with a
// single space between bars (rendered width = 2*equalizerBarCount-1).
const equalizerBarCount = 16

// equalizerMaxHeight is the upper bound of a bar's animated height.
// A bar spans two terminal rows; heights 0..4 fill only the bottom
// row, 5..8 keep the bottom row full and grow into the top row.
const equalizerMaxHeight = 8

// equalizerLowerGlyphs[h] is the bottom-row glyph for heights 0..4.
// h=0 deliberately renders as a space, not ▁, so quiet bars feel
// truly empty rather than sitting on a permanent baseline.
var equalizerLowerGlyphs = [...]rune{' ', '▁', '▂', '▃', '▄'}

// equalizerBar is the per-column animation state. phase is advanced
// every tick by speed and the visible height is derived from
// sin(phase) — there is no real audio feeding this animation, since
// the project plays audio via mpv's JSON-IPC and never sees PCM.
type equalizerBar struct {
	phase float64
	speed float64
}

// equalizer drives the decorative bar animation. State is seeded
// deterministically (no math/rand) so snapshot tests stay stable.
type equalizer struct {
	bars []equalizerBar
}

// newEqualizer returns an equalizer whose first frame is already
// non-uniform: starting phases are spread over [0, π) so adjacent
// bars don't sit at the same height, and per-bar speeds vary across
// [0.20, 0.45] rad/tick so neighbours don't crawl in lockstep.
func newEqualizer() equalizer {
	bars := make([]equalizerBar, equalizerBarCount)
	for i := range bars {
		t := float64(i) / float64(equalizerBarCount)
		// A non-monotonic mapping from i to speed so adjacent bars
		// don't share visibly similar tempos.
		speedShape := 0.5 + 0.5*math.Sin(2*math.Pi*t*1.7)
		bars[i] = equalizerBar{
			phase: float64(i) * math.Pi / float64(equalizerBarCount),
			speed: 0.20 + 0.25*speedShape,
		}
	}
	return equalizer{bars: bars}
}

// advance ticks every bar's phase forward by its own speed.
func (e *equalizer) advance() {
	for i := range e.bars {
		e.bars[i].phase += e.bars[i].speed
	}
}

// heights returns the current height of each bar in [0, equalizerMaxHeight].
// When loading is true the amplitude is squashed so the bars stay a
// quiet ripple and don't compete with the buffering spinner.
func (e equalizer) heights(loading bool) []int {
	out := make([]int, len(e.bars))
	maxH := float64(equalizerMaxHeight)
	if loading {
		maxH = 2
	}
	for i, b := range e.bars {
		// 0.5 + 0.5*sin keeps v in [0,1] so multiplying by maxH lands
		// inside [0, equalizerMaxHeight]; no clamping is needed.
		v := 0.5 + 0.5*math.Sin(b.phase)
		out[i] = int(math.Round(v * maxH))
	}
	return out
}

// equalizerGlyphs splits a height into the (top, bottom) glyph pair
// for the bar's two rows. 0..4 grow within the bottom row; 5..8 keep
// the bottom row full and grow into the top row using the same
// lower-block ramp shifted up by one cell.
func equalizerGlyphs(h int) (top, bottom rune) {
	switch {
	case h <= 0:
		return ' ', ' '
	case h <= 4:
		return ' ', equalizerLowerGlyphs[h]
	default:
		return equalizerLowerGlyphs[h-4], '█'
	}
}
