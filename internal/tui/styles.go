package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/iRootPro/lofi-player/internal/theme"
)

// Styles holds the lipgloss styles derived from a Theme.
//
// Swapping themes at runtime is a matter of recomputing Styles via
// NewStyles — every render path reads from this bag.
type Styles struct {
	AppTitle       lipgloss.Style
	Clock          lipgloss.Style
	StationName    lipgloss.Style
	StatusLive     lipgloss.Style
	StatusPaused   lipgloss.Style
	VolLabel       lipgloss.Style
	VolFill        lipgloss.Style
	VolEmpty       lipgloss.Style
	VolPercent     lipgloss.Style
	SectionHeader  lipgloss.Style
	StationItem    lipgloss.Style
	StationCursor  lipgloss.Style
	StationPlaying lipgloss.Style
	// Cursor styles the navigation cursor glyph and the highlighted
	// station name. Distinct from StationCursor (which is the accent
	// pink used for error indicators and "press a" hints) so the
	// cursor's blue doesn't conflict with the green playing dot when
	// they share a row.
	Cursor    lipgloss.Style
	HelpKey   lipgloss.Style
	HelpDesc  lipgloss.Style
	HelpSep   lipgloss.Style
	HelpGroup lipgloss.Style
	Hint      lipgloss.Style
	// EqHigh / EqMid / EqLow color the decorative equalizer bars by
	// height: peaks pick up the brand Primary, mid-range Secondary,
	// and the quiet base sits in Muted so the composition reads as
	// layered rather than flat.
	EqHigh lipgloss.Style
	EqMid  lipgloss.Style
	EqLow  lipgloss.Style
}

// NewStyles builds Styles from a Theme. Bold is reserved for the app title
// and the highlighted station; everything else relies on color.
func NewStyles(t theme.Theme) Styles {
	muted := lipgloss.NewStyle().Foreground(t.Muted)
	return Styles{
		AppTitle: lipgloss.NewStyle().Foreground(t.Primary),
		// Clock sits as a quiet auxiliary in the top-right; the muted
		// tone keeps it from competing with the brand on the left.
		Clock:        muted,
		StationName:  lipgloss.NewStyle().Foreground(t.Secondary).Bold(true),
		StatusLive:   lipgloss.NewStyle().Foreground(t.Success),
		StatusPaused: muted,
		// VolLabel matches the fill color so the speaker icon and the
		// filled cells read as one unified volume widget.
		VolLabel: lipgloss.NewStyle().Foreground(t.Primary),
		VolFill:  lipgloss.NewStyle().Foreground(t.Primary),
		// VolEmpty is the placeholder track behind the fill cells.
		// Muted (lighter than Subtle) keeps the empty segments visible
		// against the dark background so the bar reads as "fill on a
		// rail" rather than disappearing into negative space.
		VolEmpty:       lipgloss.NewStyle().Foreground(t.Muted),
		VolPercent:     lipgloss.NewStyle().Foreground(t.Info),
		SectionHeader:  muted,
		StationItem:    lipgloss.NewStyle().Foreground(t.Foreground),
		StationCursor:  lipgloss.NewStyle().Foreground(t.Accent).Bold(true),
		StationPlaying: lipgloss.NewStyle().Foreground(t.Accent),
		Cursor:         lipgloss.NewStyle().Foreground(t.Secondary).Bold(true),
		HelpKey:        lipgloss.NewStyle().Foreground(t.Warning),
		HelpDesc:       muted,
		HelpSep:        muted,
		HelpGroup:      muted,
		Hint:           muted,
		EqHigh:         lipgloss.NewStyle().Foreground(t.Primary),
		EqMid:          lipgloss.NewStyle().Foreground(t.Secondary),
		EqLow:          muted,
	}
}
