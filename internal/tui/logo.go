package tui

// logoLines is the static ASCII art for the "lofi" logo rendered
// next to the now-playing card. The Nerd Font music note from the
// frame title is mirrored as a prefix on the middle row, so the
// logo reads as a callback to the brand element in the top border.
// The "i" uses a middle dot to match the lowercase reading of the
// app name and to give the rightmost column more presence than a
// bare vertical bar.
//
// All rows must be padded to the same display width so the
// shimmer's column math lines up across rows; the leading padding
// on rows 1 and 3 mirrors the iconLogo + space on row 2.
var logoLines = [...]string{
	"  ┃     ┏━━━┓ ┏━━━━ ●",
	iconLogo + " ┃     ┃   ┃ ┣━━   ┃",
	"  ┗━━━━ ┗━━━┛ ┃     ┃",
}

// logoShimmerHalo is the radius (in cells) of the lit zone around
// the shimmer crest. The crest cell uses LogoCrest, cells within
// ±halo use LogoMid, and the rest stay on LogoBase — three soft
// bands rather than a hard spotlight.
const logoShimmerHalo = 2

// logo holds the shimmer-animation state for the ASCII logo. tick
// advances on every logoTickMsg while a station is playing and
// freezes on pause, so the shimmer pauses with the audio.
type logo struct {
	tick int
}

func (l *logo) advance() {
	l.tick++
}

// crestColumn returns the on-screen column of the shimmer crest.
// The wave enters from off-screen left (negative columns), sweeps
// across the logo, exits off-screen right, then wraps — leaving a
// brief dim "rest" each cycle so the shimmer reads as a calm breath
// rather than a continuous spinner.
func (l logo) crestColumn(width int) int {
	period := width + logoShimmerHalo*2
	return (l.tick % period) - logoShimmerHalo
}
