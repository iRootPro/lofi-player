package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"github.com/iRootPro/lofi-player/internal/pomodoro"
)

const (
	leftPad       = "  "
	progressWidth = 16
	volumeWidth   = 10

	// twoColMinWidth is the smallest terminal width at which the
	// stations + pomodoro/today panels appear side by side. Below this
	// threshold the right column collapses underneath instead.
	twoColMinWidth = 70
	// streakBarWidth caps the streak bar at 7 cells so a long streak
	// doesn't widen the right column unbounded.
	streakBarWidth = 7
)

// View renders the model. Returns an empty string until the first
// WindowSizeMsg arrives so the user never sees a stretched flash on
// startup (plan §6 pitfall).
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	if m.mode == modeMini {
		return m.viewMini()
	}
	return m.viewFull()
}

func (m Model) viewFull() string {
	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteString("\n\n")
	b.WriteString(m.renderNowPlaying())
	b.WriteString("\n\n")
	b.WriteString(m.renderProgress())
	b.WriteString("\n")
	b.WriteString(m.renderVolume())
	b.WriteString("\n\n")
	b.WriteString(m.renderMainArea())
	b.WriteString("\n\n")
	b.WriteString(m.renderSeparator())
	b.WriteString("\n")
	b.WriteString(m.renderHelpOrToast())
	return b.String()
}

// renderMainArea returns the stations-list block with pomodoro/today
// side panels attached when either the timer is active or persisted
// stats are non-empty. Narrow terminals fall back to vertically
// stacking the panels under the stations list.
func (m Model) renderMainArea() string {
	stations := m.renderStations()
	if !m.hasPomodoroSurface() {
		return stations
	}
	right := m.renderRightColumn()
	if m.width < twoColMinWidth {
		return stations + "\n\n" + right
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, stations, "    ", right)
}

// hasPomodoroSurface reports whether the pomodoro/today panels should
// be rendered at all. When false, stations gets the full width.
func (m Model) hasPomodoroSurface() bool {
	return m.session.Phase != pomodoro.PhaseIdle || m.stats.ListenedToday > 0 || m.stats.Streak > 0
}

func (m Model) renderRightColumn() string {
	var b strings.Builder
	if m.session.Phase != pomodoro.PhaseIdle {
		b.WriteString(m.renderPomodoroBlock())
	}
	if m.stats.ListenedToday > 0 || m.stats.Streak > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(m.renderTodayBlock())
	}
	return b.String()
}

func (m Model) renderPomodoroBlock() string {
	rem := pomodoro.Remaining(m.session, time.Now())
	mm := int(rem / time.Minute)
	ss := int((rem % time.Minute) / time.Second)
	cycle := m.cfg.Pomodoro.RoundsUntilLongBreak
	round := m.session.Round + 1
	if round > cycle && cycle > 0 {
		round = ((round - 1) % cycle) + 1
	}

	var b strings.Builder
	b.WriteString(m.styles.SectionHeader.Render("─── pomodoro ───"))
	b.WriteString("\n")
	phaseLabel := m.session.Phase.String()
	timeLabel := fmt.Sprintf("%02d:%02d", mm, ss)
	b.WriteString(fmt.Sprintf("%-10s%s",
		m.styles.HelpDesc.Render(phaseLabel),
		m.styles.StatusLive.Render(timeLabel)))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%-10s%s",
		m.styles.HelpDesc.Render("round"),
		m.styles.HelpKey.Render(fmt.Sprintf("%d / %d", round, cycle))))
	return b.String()
}

func (m Model) renderTodayBlock() string {
	var b strings.Builder
	b.WriteString(m.styles.SectionHeader.Render("─── today ───"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%-10s%s",
		m.styles.HelpDesc.Render("listened"),
		m.styles.StationItem.Render(formatListened(m.stats.ListenedToday))))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%-10s%s",
		m.styles.HelpDesc.Render("streak"),
		m.renderStreakBar(m.stats.Streak)))
	return b.String()
}

func (m Model) renderStreakBar(streak int) string {
	if streak < 0 {
		streak = 0
	}
	fill := streak
	if fill > streakBarWidth {
		fill = streakBarWidth
	}
	filled := m.styles.StatusLive.Render(strings.Repeat("▰", fill))
	empty := m.styles.VolEmpty.Render(strings.Repeat("▱", streakBarWidth-fill))
	suffix := ""
	if streak > streakBarWidth {
		suffix = " " + m.styles.HelpDesc.Render(fmt.Sprintf("+%d", streak-streakBarWidth))
	}
	return filled + empty + suffix
}

// formatListened renders a duration as "Xh Ym" / "Ym" / "<1m". Seconds
// are intentionally dropped — the stat is a daily summary, not a
// stopwatch.
func formatListened(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

// viewMini renders the 6-line compact layout from plan §5.1, suitable
// for living in a tmux split corner. Stations list, separator, and full
// help are dropped; everything else stays.
func (m Model) viewMini() string {
	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteString("\n")
	b.WriteString(m.renderNowPlaying())
	b.WriteString("\n")
	b.WriteString(m.renderProgress())
	b.WriteString("\n")
	b.WriteString(m.renderVolume())
	b.WriteString("\n")
	if m.toast != nil {
		b.WriteString(m.renderToast())
	} else {
		parts := renderBindings(m.styles, m.keys.MiniShortHelp())
		sep := m.styles.HelpSep.Render("  ·  ")
		b.WriteString(leftPad + strings.Join(parts, sep))
	}
	return b.String()
}

func (m Model) renderHeader() string {
	title := m.styles.AppTitle.Render("♪ lofi.player")
	clock := m.styles.Clock.Render(time.Now().Format("15:04"))

	gap := m.width - len(leftPad)*2 - lipgloss.Width(title) - lipgloss.Width(clock)
	if gap < 1 {
		gap = 1
	}
	return leftPad + title + strings.Repeat(" ", gap) + clock
}

func (m Model) renderNowPlaying() string {
	if m.playingIdx < 0 || m.playingIdx >= len(m.cfg.Stations) {
		return leftPad + m.styles.Hint.Render("— no station selected —")
	}
	name := m.cfg.Stations[m.playingIdx].Name
	status := "live"
	statusStyle := m.styles.StatusLive
	if !m.playing {
		status = "paused"
		statusStyle = m.styles.StatusPaused
	}
	dot := m.styles.SectionHeader.Render("  ·  ")

	stationLine := leftPad + m.styles.StationName.Render(name) + dot + statusStyle.Render(status)
	if track := m.formatTrack(); track != "" {
		return stationLine + "\n" + leftPad + track
	}
	return stationLine
}

func (m Model) formatTrack() string {
	if m.currentTrack.Title == "" && m.currentTrack.Artist == "" {
		return ""
	}
	mark := m.styles.StationPlaying.Render("♪") + " "
	switch {
	case m.currentTrack.Artist != "" && m.currentTrack.Title != "":
		return mark +
			m.styles.StationItem.Render(m.currentTrack.Title) +
			m.styles.SectionHeader.Render("  —  ") +
			m.styles.HelpKey.Render(m.currentTrack.Artist)
	case m.currentTrack.Title != "":
		return mark + m.styles.StationItem.Render(m.currentTrack.Title)
	default:
		return mark + m.styles.HelpKey.Render(m.currentTrack.Artist)
	}
}

func (m Model) renderProgress() string {
	caption := "—"
	if m.playingIdx >= 0 {
		if m.playing {
			caption = "live stream"
		} else {
			caption = "paused"
		}
	}
	bar := m.styles.ProgressFill.Render(strings.Repeat("▰", progressWidth))
	if m.playingIdx < 0 || !m.playing {
		bar = m.styles.ProgressEmpty.Render(strings.Repeat("▱", progressWidth))
	}
	return leftPad + bar + "  " + m.styles.ProgressLabel.Render(caption)
}

func (m Model) renderVolume() string {
	displayed := m.volumeDisplayed
	if displayed < 0 {
		displayed = 0
	}
	if displayed > 100 {
		displayed = 100
	}
	fill := int(displayed * volumeWidth / 100)
	if fill < 0 {
		fill = 0
	}
	if fill > volumeWidth {
		fill = volumeWidth
	}
	bar := m.styles.VolFill.Render(strings.Repeat("▰", fill)) +
		m.styles.VolEmpty.Render(strings.Repeat("▱", volumeWidth-fill))
	return leftPad +
		m.styles.VolLabel.Render("VOL ") +
		bar +
		"  " +
		m.styles.VolPercent.Render(fmt.Sprintf("%d%%", m.volume))
}

func (m Model) renderStations() string {
	var b strings.Builder
	b.WriteString(leftPad + m.styles.SectionHeader.Render("─── stations ───"))
	b.WriteString("\n")

	if len(m.cfg.Stations) == 0 {
		b.WriteString(leftPad + "  " + m.styles.Hint.Render("(no stations configured)"))
		b.WriteString("\n")
	}

	for i, s := range m.cfg.Stations {
		var prefix, name string
		switch {
		case i == m.cursor:
			prefix = m.styles.StationCursor.Render("›") + " "
			name = m.styles.StationCursor.Render(s.Name)
		case i == m.playingIdx:
			prefix = "  "
			name = m.styles.StationPlaying.Render(s.Name)
		default:
			prefix = "  "
			name = m.styles.StationItem.Render(s.Name)
		}
		line := leftPad + prefix + name
		if i == m.playingIdx {
			line += "  " + m.styles.StationPlaying.Render("♪")
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("\n")
	b.WriteString(leftPad + "  " + m.styles.AddStation.Render("+ add station"))
	return b.String()
}

func (m Model) renderSeparator() string {
	width := m.width - len(leftPad)*2
	if width < 1 {
		width = 1
	}
	return leftPad + m.styles.Separator.Render(strings.Repeat("─", width))
}

func (m Model) renderHelpOrToast() string {
	if m.toast != nil {
		return m.renderToast()
	}
	if m.showFullHelp {
		return m.renderFullHelp()
	}
	return m.renderShortHelp()
}

func (m Model) renderToast() string {
	t := m.toast
	out := leftPad
	if label := t.label(); label != "" {
		out += t.labelStyle(m.styles).Render(label)
	}
	return out + m.styles.HelpDesc.Render(t.Message)
}

func (m Model) renderShortHelp() string {
	parts := renderBindings(m.styles, m.keys.ShortHelp())
	sep := m.styles.HelpSep.Render("  ·  ")
	return leftPad + strings.Join(parts, sep)
}

func (m Model) renderFullHelp() string {
	groups := m.keys.FullHelp()
	labels := []string{"navigation", "playback", "app"}

	var inner strings.Builder
	for i, g := range groups {
		if i > 0 {
			inner.WriteString("\n\n")
		}
		if i < len(labels) {
			inner.WriteString(m.styles.HelpGroup.Render(labels[i]))
			inner.WriteString("\n")
		}
		for j, binding := range g {
			h := binding.Help()
			if h.Key == "" || h.Desc == "" {
				continue
			}
			if j > 0 {
				inner.WriteString("\n")
			}
			inner.WriteString("  " +
				m.styles.HelpKey.Render(fmt.Sprintf("%-7s", h.Key)) +
				m.styles.HelpDesc.Render(h.Desc))
		}
	}

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Muted).
		Padding(1, 3).
		Render(inner.String())

	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, card)
}

func renderBindings(s Styles, bindings []key.Binding) []string {
	out := make([]string, 0, len(bindings))
	for _, b := range bindings {
		h := b.Help()
		if h.Key == "" || h.Desc == "" {
			continue
		}
		out = append(out, s.HelpKey.Render(h.Key)+" "+s.HelpDesc.Render(h.Desc))
	}
	return out
}
