// Package theme defines the color palettes used by the renderer.
//
// A Theme is a flat bag of semantic color roles. The TUI's Styles type
// builds lipgloss.Style values from a Theme; swapping themes at runtime
// is therefore a matter of recomputing Styles.
package theme

import "github.com/charmbracelet/lipgloss"

// Info describes a built-in theme for pickers, help text, and docs.
type Info struct {
	// Name is the canonical identifier used in config/state files.
	Name string
	// DisplayName is the human-friendly label shown in the TUI.
	DisplayName string
	// Description is a short mood note that helps users choose a palette.
	Description string
}

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

// CatppuccinMocha returns the Catppuccin Mocha palette
// (https://github.com/catppuccin/catppuccin).
func CatppuccinMocha() Theme {
	return Theme{
		Name:       "catppuccin-mocha",
		Background: lipgloss.Color("#1e1e2e"), // Base
		Foreground: lipgloss.Color("#cdd6f4"), // Text
		Muted:      lipgloss.Color("#6c7086"), // Overlay0
		Subtle:     lipgloss.Color("#585b70"), // Surface2
		Primary:    lipgloss.Color("#cba6f7"), // Mauve
		Secondary:  lipgloss.Color("#89b4fa"), // Blue
		Accent:     lipgloss.Color("#f5c2e7"), // Pink
		Success:    lipgloss.Color("#a6e3a1"), // Green
		Warning:    lipgloss.Color("#f9e2af"), // Yellow
		Info:       lipgloss.Color("#89dceb"), // Sky
	}
}

// GruvboxDark returns the Gruvbox Dark palette
// (https://github.com/morhetz/gruvbox), bright variants for accent colors.
func GruvboxDark() Theme {
	return Theme{
		Name:       "gruvbox-dark",
		Background: lipgloss.Color("#282828"), // bg0
		Foreground: lipgloss.Color("#ebdbb2"), // fg1
		Muted:      lipgloss.Color("#928374"), // gray
		Subtle:     lipgloss.Color("#504945"), // bg2
		Primary:    lipgloss.Color("#d3869b"), // bright_purple
		Secondary:  lipgloss.Color("#83a598"), // bright_blue
		Accent:     lipgloss.Color("#fb4934"), // bright_red
		Success:    lipgloss.Color("#b8bb26"), // bright_green
		Warning:    lipgloss.Color("#fabd2f"), // bright_yellow
		Info:       lipgloss.Color("#8ec07c"), // bright_aqua
	}
}

// RosePine returns the Rose Pine palette (https://rosepinetheme.com/).
// Rose Pine is a cool palette with no true green; Pine (teal) substitutes
// for the Success role.
func RosePine() Theme {
	return Theme{
		Name:       "rose-pine",
		Background: lipgloss.Color("#191724"), // Base
		Foreground: lipgloss.Color("#e0def4"), // Text
		Muted:      lipgloss.Color("#6e6a86"), // Muted
		Subtle:     lipgloss.Color("#1f1d2e"), // Surface
		Primary:    lipgloss.Color("#c4a7e7"), // Iris
		Secondary:  lipgloss.Color("#9ccfd8"), // Foam
		Accent:     lipgloss.Color("#ebbcba"), // Rose
		Success:    lipgloss.Color("#31748f"), // Pine
		Warning:    lipgloss.Color("#f6c177"), // Gold
		Info:       lipgloss.Color("#eb6f92"), // Love
	}
}

// CatppuccinLatte returns the Catppuccin Latte light palette
// (https://github.com/catppuccin/catppuccin).
func CatppuccinLatte() Theme {
	return Theme{
		Name:       "catppuccin-latte",
		Background: lipgloss.Color("#eff1f5"), // Base
		Foreground: lipgloss.Color("#4c4f69"), // Text
		Muted:      lipgloss.Color("#9ca0b0"), // Overlay0
		Subtle:     lipgloss.Color("#ccd0da"), // Surface0
		Primary:    lipgloss.Color("#8839ef"), // Mauve
		Secondary:  lipgloss.Color("#1e66f5"), // Blue
		Accent:     lipgloss.Color("#ea76cb"), // Pink
		Success:    lipgloss.Color("#40a02b"), // Green
		Warning:    lipgloss.Color("#df8e1d"), // Yellow
		Info:       lipgloss.Color("#04a5e5"), // Sky
	}
}

// RosePineDawn returns the Rose Pine Dawn light palette
// (https://rosepinetheme.com/).
func RosePineDawn() Theme {
	return Theme{
		Name:       "rose-pine-dawn",
		Background: lipgloss.Color("#faf4ed"), // Base
		Foreground: lipgloss.Color("#575279"), // Text
		Muted:      lipgloss.Color("#9893a5"), // Muted
		Subtle:     lipgloss.Color("#fffaf3"), // Surface
		Primary:    lipgloss.Color("#907aa9"), // Iris
		Secondary:  lipgloss.Color("#56949f"), // Foam
		Accent:     lipgloss.Color("#d7827e"), // Rose
		Success:    lipgloss.Color("#286983"), // Pine
		Warning:    lipgloss.Color("#ea9d34"), // Gold
		Info:       lipgloss.Color("#b4637a"), // Love
	}
}

// SolarizedLight returns the Solarized Light palette
// (https://ethanschoonover.com/solarized/).
func SolarizedLight() Theme {
	return Theme{
		Name:       "solarized-light",
		Background: lipgloss.Color("#fdf6e3"), // base3
		Foreground: lipgloss.Color("#657b83"), // base00
		Muted:      lipgloss.Color("#93a1a1"), // base1
		Subtle:     lipgloss.Color("#eee8d5"), // base2
		Primary:    lipgloss.Color("#6c71c4"), // violet
		Secondary:  lipgloss.Color("#268bd2"), // blue
		Accent:     lipgloss.Color("#d33682"), // magenta
		Success:    lipgloss.Color("#859900"), // green
		Warning:    lipgloss.Color("#b58900"), // yellow
		Info:       lipgloss.Color("#2aa198"), // cyan
	}
}

// Paper returns a warm, low-glare light palette tuned for plain terminal
// backgrounds and long listening sessions.
func Paper() Theme {
	return Theme{
		Name:       "paper",
		Background: lipgloss.Color("#f7f3ea"),
		Foreground: lipgloss.Color("#3f3a32"),
		Muted:      lipgloss.Color("#8b8174"),
		Subtle:     lipgloss.Color("#e3dccf"),
		Primary:    lipgloss.Color("#5c6bc0"),
		Secondary:  lipgloss.Color("#00897b"),
		Accent:     lipgloss.Color("#d81b60"),
		Success:    lipgloss.Color("#558b2f"),
		Warning:    lipgloss.Color("#c77700"),
		Info:       lipgloss.Color("#0277bd"),
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

// Names returns all registered theme names in stable order — Tokyo Night
// first (the default), then the rest alphabetically. Callers use this to
// drive the theme picker in the TUI.
func Names() []string {
	out := make([]string, len(infos))
	for i, info := range infos {
		out[i] = info.Name
	}
	return out
}

// InfoFor returns the metadata for a registered theme. Unknown names fall
// back to Tokyo Night metadata and false, mirroring Lookup.
func InfoFor(name string) (Info, bool) {
	for _, info := range infos {
		if info.Name == name {
			return info, true
		}
	}
	return infos[0], false
}

// Infos returns all registered theme metadata in the same stable order as
// Names. The returned slice is a copy so callers cannot mutate the registry.
func Infos() []Info {
	out := make([]Info, len(infos))
	copy(out, infos)
	return out
}

// Next returns the theme name that follows current in the cycle order
// from Names(). Wraps around at the end. Unknown current returns the
// first name.
func Next(current string) string {
	names := Names()
	for i, n := range names {
		if n == current {
			return names[(i+1)%len(names)]
		}
	}
	return names[0]
}

// infos is the canonical order used by Names and the TUI picker.
var infos = []Info{
	{Name: "tokyo-night", DisplayName: "Tokyo Night", Description: "cool neon on deep blue"},
	{Name: "catppuccin-mocha", DisplayName: "Catppuccin Mocha", Description: "soft pastels on warm charcoal"},
	{Name: "gruvbox-dark", DisplayName: "Gruvbox Dark", Description: "earthy contrast with vintage warmth"},
	{Name: "rose-pine", DisplayName: "Rose Pine", Description: "muted mauve, calm and low-glare"},
	{Name: "catppuccin-latte", DisplayName: "Catppuccin Latte", Description: "airy pastels on a clean light base"},
	{Name: "rose-pine-dawn", DisplayName: "Rose Pine Dawn", Description: "warm sunrise paper with soft mauve"},
	{Name: "solarized-light", DisplayName: "Solarized Light", Description: "classic low-contrast terminal daylight"},
	{Name: "paper", DisplayName: "Paper", Description: "warm ink on cream for bright rooms"},
}

// registry maps theme names to constructors. Phase 0 shipped only Tokyo
// Night; Phase 2 adds catppuccin-mocha, gruvbox-dark, and rose-pine.
var registry = map[string]func() Theme{
	"tokyo-night":      TokyoNight,
	"catppuccin-mocha": CatppuccinMocha,
	"gruvbox-dark":     GruvboxDark,
	"rose-pine":        RosePine,
	"catppuccin-latte": CatppuccinLatte,
	"rose-pine-dawn":   RosePineDawn,
	"solarized-light":  SolarizedLight,
	"paper":            Paper,
}
