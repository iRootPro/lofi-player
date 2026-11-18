package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/iRootPro/lofi-player/internal/i18n"
	"github.com/iRootPro/lofi-player/internal/theme"
)

// InstallCmd is one platform's install instruction for a missing
// runtime dependency. Used by RenderMissingDependency to render a
// platform-by-platform list under the dependency name.
type InstallCmd struct {
	Platform string
	Cmd      string
}

// RenderMissingDependency builds a styled "can't start" card for a
// hard-blocking missing dependency (e.g. mpv). Caller writes the
// result to stderr and exits — there is no event loop. The card
// renders against the Tokyo Night palette since the user's chosen
// theme is loaded later (and may not exist if config is broken).
func RenderMissingDependency(locale i18n.Locale, name, role string, cmds []InstallCmd) string {
	t, _ := theme.Lookup("tokyo-night")
	s := NewStyles(t)

	title := s.AppTitle.Render(iconLogo+" lofi.player ") +
		s.HelpDesc.Render(i18n.T(locale, "cannot_start"))

	missing := s.StationCursor.Render("✗ ") +
		s.StationName.Render(name) +
		s.HelpDesc.Render(" — "+role)

	var lines []string
	lines = append(lines, title, "", missing, "")
	for _, c := range cmds {
		platform := s.HelpKey.Render(c.Platform + ":")
		lines = append(lines, "    "+platform+"  "+s.HelpDesc.Render(c.Cmd))
	}
	lines = append(lines,
		"",
		s.HelpDesc.Render(i18n.T(locale, "install_again")),
	)

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Muted).
		Padding(1, 3).
		Render(strings.Join(lines, "\n"))

	return card + "\n"
}
