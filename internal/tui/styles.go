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
	ProgressFill   lipgloss.Style
	ProgressEmpty  lipgloss.Style
	ProgressLabel  lipgloss.Style
	VolLabel       lipgloss.Style
	VolFill        lipgloss.Style
	VolEmpty       lipgloss.Style
	VolPercent     lipgloss.Style
	SectionHeader  lipgloss.Style
	StationItem    lipgloss.Style
	StationCursor  lipgloss.Style
	StationPlaying lipgloss.Style
	AddStation     lipgloss.Style
	Separator      lipgloss.Style
	HelpKey        lipgloss.Style
	HelpDesc       lipgloss.Style
	HelpSep        lipgloss.Style
	HelpGroup      lipgloss.Style
	Hint           lipgloss.Style
}

// NewStyles builds Styles from a Theme. Bold is reserved for the app title
// and the highlighted station; everything else relies on color.
func NewStyles(t theme.Theme) Styles {
	muted := lipgloss.NewStyle().Foreground(t.Muted)
	return Styles{
		AppTitle:       lipgloss.NewStyle().Foreground(t.Primary).Bold(true),
		Clock:          lipgloss.NewStyle().Foreground(t.Info),
		StationName:    lipgloss.NewStyle().Foreground(t.Secondary).Bold(true),
		StatusLive:     lipgloss.NewStyle().Foreground(t.Success),
		StatusPaused:   muted,
		ProgressFill:   lipgloss.NewStyle().Foreground(t.Success),
		ProgressEmpty:  lipgloss.NewStyle().Foreground(t.Subtle),
		ProgressLabel:  lipgloss.NewStyle().Foreground(t.Info),
		VolLabel:       muted,
		VolFill:        lipgloss.NewStyle().Foreground(t.Primary),
		VolEmpty:       lipgloss.NewStyle().Foreground(t.Subtle),
		VolPercent:     lipgloss.NewStyle().Foreground(t.Info),
		SectionHeader:  muted,
		StationItem:    lipgloss.NewStyle().Foreground(t.Foreground),
		StationCursor:  lipgloss.NewStyle().Foreground(t.Accent).Bold(true),
		StationPlaying: lipgloss.NewStyle().Foreground(t.Accent),
		AddStation:     muted,
		Separator:      muted,
		HelpKey:        lipgloss.NewStyle().Foreground(t.Warning),
		HelpDesc:       muted,
		HelpSep:        muted,
		HelpGroup:      muted,
		Hint:           muted,
	}
}
