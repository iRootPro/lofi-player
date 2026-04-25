// Package theme defines the color palettes used by the renderer.
//
// A Theme is a flat bag of semantic color roles. The TUI's Styles type
// builds lipgloss.Style values from a Theme; swapping themes at runtime
// is therefore a matter of recomputing Styles.
package theme

import "github.com/charmbracelet/lipgloss"

// Theme is a flat palette of semantic color roles.
//
// Roles are intentionally semantic (Primary, Accent, Success) rather than
// chromatic (Purple, Pink, Green) so that styles defined in terms of roles
// continue to make sense across themes.
type Theme struct {
	// Name is the canonical identifier used in config files (e.g. "tokyo-night").
	Name string

	// Background is the terminal background. Renderers should not paint it
	// explicitly so that translucent terminals stay translucent.
	Background lipgloss.Color
	// Foreground is the default text color.
	Foreground lipgloss.Color
	// Muted is for section headers, hints, and separators.
	Muted lipgloss.Color
	// Subtle is for empty progress segments.
	Subtle lipgloss.Color
	// Primary is the brand color, used for the app title.
	Primary lipgloss.Color
	// Secondary is the station name in the now-playing block.
	Secondary lipgloss.Color
	// Accent marks the selected station and the ♪ glyph.
	Accent lipgloss.Color
	// Success is for "live", filled progress segments, and the focus state.
	Success lipgloss.Color
	// Warning is for help-bar keys and artist names.
	Warning lipgloss.Color
	// Info is for the clock, percentages, and other temporal indicators.
	Info lipgloss.Color
}

// TokyoNight returns the default Tokyo Night palette.
func TokyoNight() Theme {
	return Theme{
		Name:       "tokyo-night",
		Background: lipgloss.Color("#1a1b26"),
		Foreground: lipgloss.Color("#c0caf5"),
		Muted:      lipgloss.Color("#565f89"),
		Subtle:     lipgloss.Color("#414868"),
		Primary:    lipgloss.Color("#bb9af7"),
		Secondary:  lipgloss.Color("#7aa2f7"),
		Accent:     lipgloss.Color("#f7768e"),
		Success:    lipgloss.Color("#9ece6a"),
		Warning:    lipgloss.Color("#e0af68"),
		Info:       lipgloss.Color("#7dcfff"),
	}
}

// Lookup returns the theme registered under name. If name is empty or
// unknown, Lookup returns Tokyo Night and false; otherwise it returns the
// matched theme and true. Callers can use the bool to warn the user when
// their config references a theme that doesn't exist.
func Lookup(name string) (Theme, bool) {
	if t, ok := registry[name]; ok {
		return t(), true
	}
	return TokyoNight(), false
}

// registry maps theme names to constructors. Phase 0 only ships Tokyo Night;
// Phase 2 adds catppuccin-mocha, gruvbox-dark, and rose-pine.
var registry = map[string]func() Theme{
	"tokyo-night": TokyoNight,
}
