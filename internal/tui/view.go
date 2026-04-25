package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

const (
	leftPad     = "  "
	volumeWidth = 10
	// nowPlayingMaxWidth caps the now-playing card so it doesn't
	// stretch across very wide terminals. The card is the focal
	// element of the screen; an 80-cell width keeps it readable
	// without floating in negative space on a 200-cell terminal.
	nowPlayingMaxWidth = 80
)

// Nerd Font icons (FontAwesome subset, PUA range U+F000–U+F8FF).
// Terminals without a Nerd Font will render these as tofu boxes;
// the trade-off is documented in the README.
const (
	iconLogo     = "" //  music
	iconVolume   = "" //  volume-up
	iconStations = "" //  list
)

const (
	statusGlyphLive   = "●"
	statusGlyphPaused = "◯"
)

// View renders the model. Returns an empty string until the first
// WindowSizeMsg arrives so the user never sees a stretched flash on
// startup (plan §6 pitfall).
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	switch m.mode {
	case modeMini:
		return m.viewMini()
	case modeAddStation:
		return m.viewAddStation()
	default:
		return m.viewFull()
	}
}

// viewAddStation overlays the add-station modal on top of whichever
// layout was active when `a` was pressed, so the user keeps visual
// context (now-playing, station list) while typing.
func (m Model) viewAddStation() string {
	// Render the previous layout as the backdrop.
	var backdrop string
	if m.modePrev == modeMini {
		backdrop = m.viewMini()
	} else {
		backdrop = m.viewFull()
	}
	form := m.addForm.view(m.width, m.styles, m.theme.Muted)
	return backdrop + "\n\n" + form
}

func (m Model) viewFull() string {
	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteString("\n\n")
	b.WriteString(m.renderNowPlaying())
	b.WriteString("\n")
	b.WriteString(m.renderVolume())
	b.WriteString("\n")
	b.WriteString(m.renderStations())
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpOrToast())
	return b.String()
}

// viewMini renders the compact layout suitable for living in a tmux
// split corner. Stations list, separator, and full help are dropped;
// the bogus stream "progress bar" is dropped too — Phase 4b will
// re-enable it for local files where duration actually exists.
func (m Model) viewMini() string {
	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteString("\n")
	b.WriteString(m.renderNowPlaying())
	b.WriteString("\n")
	b.WriteString(m.renderVolume())
	b.WriteString("\n")
	if m.toast != nil {
		b.WriteString(m.renderToast())
	} else {
		parts := renderBindings(m.styles, m.keys.MiniShortHelp())
		b.WriteString(leftPad + strings.Join(parts, "   "))
	}
	return b.String()
}

func (m Model) renderHeader() string {
	title := m.styles.AppTitle.Render(iconLogo + "  lofi.player")
	clock := m.styles.Clock.Render(time.Now().Format("15:04"))

	gap := m.width - len(leftPad)*2 - lipgloss.Width(title) - lipgloss.Width(clock)
	if gap < 1 {
		gap = 1
	}
	return leftPad + title + strings.Repeat(" ", gap) + clock
}

// renderNowPlaying wraps the station + track block in a rounded card.
// The card is the screen's primary focus element — everything else
// (volume, stations list) sits visually below it without competing
// borders.
func (m Model) renderNowPlaying() string {
	if m.playingIdx < 0 || m.playingIdx >= len(m.cfg.Stations) {
		return leftPad + m.styles.Hint.Render("— no station selected —")
	}

	cardWidth := m.width - len(leftPad)*2
	if cardWidth > nowPlayingMaxWidth {
		cardWidth = nowPlayingMaxWidth
	}
	if cardWidth < 24 {
		cardWidth = 24
	}
	// Inside the card: 1 char of horizontal padding on each side leaves
	// (cardWidth - 2 borders - 2 padding) for content.
	innerWidth := cardWidth - 4

	name := m.cfg.Stations[m.playingIdx].Name
	stationLine := m.statusBlock() + "  " + m.styles.StationName.Render(name)
	trackLine := m.formatTrack(innerWidth)
	inner := stationLine + "\n" + trackLine

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Muted).
		Padding(0, 1).
		Width(cardWidth).
		MarginLeft(len(leftPad)).
		Render(inner)
	return card
}

// statusBlock returns the leading status indicator for the now-playing
// card and the playing-station row in the list. While the player is
// waiting for the first PlaybackStarted event after a Play call, it
// renders the spinner instead of the ●/◯ glyph.
func (m Model) statusBlock() string {
	switch {
	case m.loading:
		return m.spinner.View()
	case m.playing:
		return m.styles.StatusLive.Render(statusGlyphLive)
	default:
		return m.styles.StatusPaused.Render(statusGlyphPaused)
	}
}

// formatTrack returns the second line of the now-playing block.
//
//   - Empty metadata → a muted "…" placeholder. Audio may already be
//     playing (the status spinner has cleared), but ICY / media-title
//     hasn't resolved yet; the placeholder keeps the card two lines
//     tall without a second animated element.
//   - Real "Artist — Title" metadata → title in foreground, artist in
//     the warning accent. The normal "real track playing" case.
//   - Title only (no artist split) → muted styling. mpv's ytdl_hook
//     surfaces the YouTube channel description here when no track
//     metadata exists ("lofi hip hop radio  beats to relax/study
//     to ..."); rendering it muted communicates "stream descriptor"
//     rather than "song title".
//
// Long strings are truncated to maxWidth with an ellipsis so the card
// doesn't reflow when a verbose value arrives.
func (m Model) formatTrack(maxWidth int) string {
	if m.currentTrack.Title == "" && m.currentTrack.Artist == "" {
		return m.styles.Hint.Render("…")
	}

	sep := "  —  "
	switch {
	case m.currentTrack.Artist != "" && m.currentTrack.Title != "":
		artist := m.currentTrack.Artist
		artistRendered := m.styles.HelpKey.Render(artist)
		titleBudget := maxWidth - lipgloss.Width(sep) - lipgloss.Width(artist)
		if titleBudget < 4 {
			titleBudget = 4
		}
		title := truncateRunes(m.currentTrack.Title, titleBudget)
		return m.styles.StationItem.Render(title) +
			m.styles.SectionHeader.Render(sep) +
			artistRendered
	case m.currentTrack.Title != "":
		return m.styles.HelpDesc.Render(truncateRunes(m.currentTrack.Title, maxWidth))
	default:
		return m.styles.HelpKey.Render(truncateRunes(m.currentTrack.Artist, maxWidth))
	}
}

// truncateRunes shortens s to at most maxWidth display cells, appending
// "…" when truncation happens. Operates on runes (not bytes) so
// multi-byte characters split cleanly.
func truncateRunes(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	runes := []rune(s)
	for i := len(runes) - 1; i > 0; i-- {
		candidate := string(runes[:i]) + "…"
		if lipgloss.Width(candidate) <= maxWidth {
			return candidate
		}
	}
	return "…"
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
		m.styles.VolLabel.Render(iconVolume+"  ") +
		bar +
		"  " +
		m.styles.VolPercent.Render(fmt.Sprintf("%d%%", m.volume))
}

func (m Model) renderStations() string {
	var b strings.Builder
	header := iconStations + "  stations"
	if n := len(m.cfg.Stations); n > 0 {
		header += fmt.Sprintf("  ·  %d", n)
	}
	b.WriteString(leftPad + m.styles.SectionHeader.Render(header))
	b.WriteString("\n")

	if len(m.cfg.Stations) == 0 {
		// Indent matches the station-name column (leftPad + 4-cell prefix).
		b.WriteString(leftPad + "    " +
			m.styles.StationCursor.Render("press a") + " " +
			m.styles.Hint.Render("to add one"))
		b.WriteString("\n")
		return b.String()
	}

	for i, s := range m.cfg.Stations {
		// Three-cell prefix: cursor bar + space + playing-status dot.
		// The dot lives in the same column for every row so the station
		// names line up regardless of which one is playing or selected.
		cursor := "  "
		if i == m.cursor {
			cursor = m.styles.Cursor.Render("▎") + " "
		}

		marker := " "
		if i == m.playingIdx {
			marker = m.statusBlock()
		}

		var name string
		switch {
		case i == m.cursor:
			name = m.styles.Cursor.Render(s.Name)
		case i == m.playingIdx:
			name = m.styles.StationPlaying.Render(s.Name)
		default:
			name = m.styles.StationItem.Render(s.Name)
		}

		// Drop the trailing newline on the last row — viewFull adds the
		// inter-block gap itself, so trailing \n stacks with the gap
		// and inflates the spacing.
		line := leftPad + cursor + marker + " " + name
		if i < len(m.cfg.Stations)-1 {
			line += "\n"
		}
		b.WriteString(line)
	}
	return b.String()
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
	// Triple-space gives a soft "tab" between bindings without the
	// noise of an explicit separator glyph.
	return leftPad + strings.Join(parts, "   ")
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
