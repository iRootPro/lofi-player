package tui

import (
	"fmt"
	"strings"

	"github.com/iRootPro/lofi-player/internal/theme"
)

// StatusLine returns a single colored status string suitable for tmux's
// status-right (or any "run command, show output" integration). It does
// not depend on an active mpv session — it reads from the persisted
// state passed in by the caller. When nothing has ever played the line
// degrades to the brand name plus the current volume.
//
// Width is intentionally kept narrow (small volume bar, no clock) so
// the line fits next to other tmux widgets.
func StatusLine(themeName, station, volPercent string, volume int) string {
	const barWidth = 6

	t, _ := theme.Lookup(themeName)
	s := NewStyles(t)

	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}
	fill := volume * barWidth / 100
	bar := s.VolFill.Render(strings.Repeat("▰", fill)) +
		s.VolEmpty.Render(strings.Repeat("▱", barWidth-fill))

	if station == "" {
		station = "lofi.player"
	}

	return fmt.Sprintf("%s %s  %s  %s",
		s.AppTitle.Render("♪"),
		s.StationName.Render(station),
		bar,
		s.VolPercent.Render(volPercent),
	)
}
