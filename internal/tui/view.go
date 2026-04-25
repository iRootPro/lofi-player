package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

const (
	leftPad       = "  "
	progressWidth = 16
	volumeWidth   = 10
)

// View renders the model. Returns an empty string until the first
// WindowSizeMsg arrives so the user never sees a stretched flash on
// startup (plan §6 pitfall).
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteString("\n\n")
	b.WriteString(m.renderNowPlaying())
	b.WriteString("\n\n")
	b.WriteString(m.renderProgress())
	b.WriteString("\n")
	b.WriteString(m.renderVolume())
	b.WriteString("\n\n")
	b.WriteString(m.renderStations())
	b.WriteString("\n\n")
	b.WriteString(m.renderSeparator())
	b.WriteString("\n")
	b.WriteString(m.renderHelpOrError())
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
	fill := m.volume * volumeWidth / 100
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

func (m Model) renderHelpOrError() string {
	if m.lastError != "" {
		return leftPad + m.styles.StationCursor.Render("error: ") + m.styles.HelpDesc.Render(m.lastError)
	}
	if m.showFullHelp {
		return m.renderFullHelp()
	}
	return m.renderShortHelp()
}

func (m Model) renderShortHelp() string {
	parts := renderBindings(m.styles, m.keys.ShortHelp())
	sep := m.styles.HelpSep.Render("  ·  ")
	return leftPad + strings.Join(parts, sep)
}

func (m Model) renderFullHelp() string {
	groups := m.keys.FullHelp()
	labels := []string{"navigation", "playback", "app"}
	var b strings.Builder
	for i, g := range groups {
		if i > 0 {
			b.WriteString("\n")
		}
		if i < len(labels) {
			b.WriteString(leftPad + m.styles.HelpGroup.Render(labels[i]) + "\n")
		}
		for _, binding := range g {
			h := binding.Help()
			if h.Key == "" || h.Desc == "" {
				continue
			}
			b.WriteString(leftPad + "  " +
				m.styles.HelpKey.Render(fmt.Sprintf("%-7s", h.Key)) +
				m.styles.HelpDesc.Render(h.Desc) + "\n")
		}
	}
	return b.String()
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
