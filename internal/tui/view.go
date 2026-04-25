package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/iRootPro/lofi-player/internal/config"
)

const (
	leftPad     = "    "
	volumeWidth = 14
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
	iconVolume   = "󰕿" //  volume-high (Material Design)
	iconStations = "" //  list
)

const (
	statusGlyphLive   = "●"
	statusGlyphPaused = "◯"
)

// appFrameWidth is the fixed width of the outer rounded border on
// terminals wide enough for it. Below this threshold the frame
// shrinks to fit; above it, the frame is centered so the app reads
// as a contained panel rather than a full-screen TUI.
const appFrameWidth = 100

// View renders the model. Returns an empty string until the first
// WindowSizeMsg arrives so the user never sees a stretched flash on
// startup (plan §6 pitfall).
//
// The whole app is wrapped in a rounded frame whose top border
// embeds the brand and the clock (title-on-the-left, label-on-the-
// right), so the header isn't a separate row inside the frame. On
// wide terminals the frame is centered horizontally at
// appFrameWidth; on narrower ones it shrinks to fit.
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	frameWidth := appFrameWidth
	if frameWidth > m.width-2 {
		frameWidth = m.width - 2
	}
	if frameWidth < 40 {
		frameWidth = 40
	}

	// Clone the model with an adjusted width so children sized off
	// m.width (now-playing track truncation, station list) fit
	// inside the frame's borders.
	inner := m
	inner.width = frameWidth - 2

	var content string
	switch m.mode {
	case modeMini:
		content = inner.viewMini()
	case modeAddStation:
		content = inner.viewAddStation()
	case modeMixer:
		content = inner.viewMixer()
	default:
		content = inner.viewFull()
	}

	title := iconLogo + " lofi.player"
	rightLabel := inner.renderVolume()
	// "?" picks up the brand Primary so it reads as part of the same
	// interactive-element family as the logo and the volume icon;
	// "help" stays muted so the hint sits quietly in the border.
	bottomLabel := m.styles.AppTitle.Render("?") + " " + m.styles.HelpDesc.Render("help")

	framed := renderFrame(
		content,
		title,
		rightLabel,
		bottomLabel,
		frameWidth,
		lipgloss.NewStyle().Foreground(m.theme.Muted),
		m.styles.AppTitle,
	)

	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, framed)
}

// viewMixer overlays the ambient-mixer modal on top of whichever layout
// was active when `x` was pressed, so the user keeps visual context.
func (m Model) viewMixer() string {
	var backdrop string
	if m.modePrev == modeMini {
		backdrop = m.viewMini()
	} else {
		backdrop = m.viewFull()
	}
	card := m.mixerUI.view(m.width, m.styles, m.theme)
	return backdrop + "\n\n" + card
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
	b.WriteString(m.renderNowPlaying())
	b.WriteString("\n\n")
	b.WriteString(m.renderStations())
	if extra := m.renderTransientFooter(); extra != "" {
		b.WriteString("\n\n")
		b.WriteString(extra)
	}
	return b.String()
}

// viewMini renders the compact layout suitable for living in a tmux
// split corner. Stations list and full help are dropped — the brand
// already lives in the frame's top border so no header is needed
// here either.
func (m Model) viewMini() string {
	var b strings.Builder
	b.WriteString(m.renderNowPlaying())
	if m.toast != nil {
		b.WriteString("\n")
		b.WriteString(m.renderToast())
	}
	return b.String()
}

// renderNowPlaying renders the station + track block as plain text.
// The outer app border (in View()) is the only rounded container on
// screen — making the now-playing block its own card too created a
// noisy double-border. Visual hierarchy now comes from typography
// (bold station name) and the status glyph (●/◯/spinner).
func (m Model) renderNowPlaying() string {
	if m.playingIdx < 0 || m.playingIdx >= len(m.cfg.Stations) {
		return leftPad + m.styles.Hint.Render("— no station selected —")
	}

	innerWidth := m.width - len(leftPad)*2
	if innerWidth > nowPlayingMaxWidth {
		innerWidth = nowPlayingMaxWidth
	}
	if innerWidth < 24 {
		innerWidth = 24
	}

	station := m.cfg.Stations[m.playingIdx]
	stationLine := leftPad + m.statusBlock() + "  " + m.styles.StationName.Render(station.Name)
	if icon := m.stationKindIcon(station); icon != "" {
		stationLine += "  " + icon
	}
	trackLine := leftPad + m.formatTrack(innerWidth)
	return stationLine + "\n" + trackLine
}

// stationKindIcon returns a small muted "· kind" tag (e.g. "· youtube"
// or "· stream") next to the station name. Symmetric labeling lets
// the user always see at a glance whether a station resolves through
// the direct stream path or through mpv's ytdl_hook.
//
// Text rather than a Nerd Font glyph: the FA youtube codepoint
// (U+F167) doesn't render reliably across Nerd Font variants, and a
// plain word reads unambiguously on any terminal.
func (m Model) stationKindIcon(s config.Station) string {
	kind := s.EffectiveKind()
	if kind == "" {
		return ""
	}
	return m.styles.SectionHeader.Render("· " + kind)
}

// statusBlock returns the leading status indicator for the now-playing
// card and the playing-station row in the list. While the player is
// waiting for the first PlaybackStarted event after a Play call, it
// renders the spinner instead of the ●/◯ glyph. While playing, the
// live ● gets a soft heartbeat via the SGR Faint attribute toggled
// by the pulse tick — calm signal that audio is alive.
func (m Model) statusBlock() string {
	switch {
	case m.loading:
		return m.spinner.View()
	case m.playing:
		style := m.styles.StatusLive
		if m.pulseDim {
			style = style.Faint(true)
		}
		return style.Render(statusGlyphLive)
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

// renderVolume composes the volume widget — speaker icon followed by
// the fill bar. Lives as the right-side label in the frame's top
// border. The bar is enough on its own; the digit/percent text was
// just visual repetition.
func (m Model) renderVolume() string {
	v := clampVolume(m.volume)
	fill := v * volumeWidth / 100
	bar := m.styles.VolFill.Render(strings.Repeat("▰", fill)) +
		m.styles.VolEmpty.Render(strings.Repeat("▱", volumeWidth-fill))
	return m.styles.VolLabel.Render(iconVolume) + " " + bar
}

func (m Model) renderStations() string {
	var b strings.Builder
	header := iconStations + " stations"
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
		if icon := m.stationKindIcon(s); icon != "" {
			line += "  " + icon
		}
		if i < len(m.cfg.Stations)-1 {
			line += "\n"
		}
		b.WriteString(line)
	}
	return b.String()
}

// renderTransientFooter returns the optional content that appears
// between the stations list and the bottom border:
//   - active toast (auto-dismissed after a few seconds), OR
//   - the full help card while the user is holding `?` open.
//
// When neither is active the function returns "" and viewFull skips
// the slot entirely so the bottom border sits flush with the
// stations list.
func (m Model) renderTransientFooter() string {
	if m.toast != nil {
		return m.renderToast()
	}
	if m.showFullHelp {
		return m.renderFullHelp()
	}
	return ""
}

func (m Model) renderToast() string {
	t := m.toast
	out := leftPad
	if label := t.label(); label != "" {
		out += t.labelStyle(m.styles).Render(label)
	}
	return out + m.styles.HelpDesc.Render(t.Message)
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

