package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/iRootPro/lofi-player/internal/audio"
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
	// logoSidePadding is the gap between the logo's right edge and
	// the inner border of the frame. Two cells let the art breathe
	// rather than crowding the rounded border.
	logoSidePadding = 2
	// logoMinGutter is the minimum spacer between the now-playing
	// block and the logo. Below this the logo is dropped to avoid
	// visual collision with the track text.
	logoMinGutter = 2
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
	case modeConfirmDelete:
		content = inner.viewConfirmDelete()
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

// viewConfirmDelete renders a small "delete <name>?" card centered in
// the frame, on top of the prior layout so the user keeps visual
// context for which row is being removed.
func (m Model) viewConfirmDelete() string {
	var backdrop string
	if m.modePrev == modeMini {
		backdrop = m.viewMini()
	} else {
		backdrop = m.viewFull()
	}

	name := "?"
	if m.pendingDeleteIdx >= 0 && m.pendingDeleteIdx < len(m.cfg.Stations) {
		name = m.cfg.Stations[m.pendingDeleteIdx].Name
	}

	prompt := m.styles.HelpDesc.Render("delete ") +
		m.styles.StationName.Render(name) +
		m.styles.HelpDesc.Render("?")
	hint := m.styles.HelpKey.Render("y") + " " +
		m.styles.HelpDesc.Render("confirm") + "  " +
		m.styles.HelpSep.Render("·") + "  " +
		m.styles.HelpKey.Render("n / esc") + " " +
		m.styles.HelpDesc.Render("cancel")

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Muted).
		Padding(0, 2).
		Render(prompt + "\n\n" + hint)

	centered := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, card)
	return backdrop + "\n\n" + centered
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
	body := m.renderBody()
	if extra := m.renderTransientFooter(); extra != "" {
		return body + "\n\n" + extra
	}
	return body
}

// renderBody composes the main left content (now-playing card +
// stations list) with the shimmering logo on the right, centering
// the logo vertically against the full height of the left column
// rather than pinning it to the top. On terminals too narrow to
// fit the logo next to the body without overlap, it is dropped.
func (m Model) renderBody() string {
	var b strings.Builder
	b.WriteString(m.renderNowPlaying())
	b.WriteString("\n\n")
	b.WriteString(m.renderStations())
	left := b.String()

	logo := m.renderLogo()
	if logo == "" {
		return left
	}

	leftWidth := lipgloss.Width(left)
	logoWidth := lipgloss.Width(logo)
	gutter := m.width - leftWidth - logoWidth - logoSidePadding
	if gutter < logoMinGutter {
		return left
	}
	spacer := strings.Repeat(" ", gutter)
	return lipgloss.JoinHorizontal(lipgloss.Center, left, spacer, logo)
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
	if indicator := m.renderAmbientIndicator(); indicator != "" {
		stationLine += "  " + indicator
	}
	trackLine := leftPad + m.formatTrack(innerWidth)
	out := stationLine + "\n" + trackLine
	if info := m.renderStreamInfoLine(); info != "" {
		out += "\n" + info
	}
	return out
}

// bufferBarCells / bufferFullSeconds / bufferLowSeconds /
// bufferShowSeconds shape the network-buffer indicator. 30 s of
// buffered audio reads as full; the bar drains linearly below that
// and crosses into the warning tone when fewer than ~5 s remain so a
// flaky network shows up loudly. Below ~1 s the bar disappears
// entirely — direct Icecast/HTTP streams play near-synchronously and
// hold almost no read-ahead, so an always-visible empty bar there
// reads as broken when it isn't.
const (
	bufferBarCells    = 5
	bufferFullSeconds = 30.0
	bufferLowSeconds  = 5.0
	bufferShowSeconds = 1.0
)

// renderStreamInfoLine composes the technical info row under the
// now-playing card. Audio-technical bits (bitrate / codec / sample
// rate / channels) cluster together separated by a single space; the
// uptime and buffer indicator stand as their own groups separated by
// `·`. Each segment is dropped when its source value is unknown, so
// the line degrades gracefully and collapses to nothing when nothing
// is known yet.
func (m Model) renderStreamInfoLine() string {
	if !m.showStreamInfo {
		return ""
	}
	if m.playingIdx < 0 || m.playingIdx >= len(m.cfg.Stations) {
		return ""
	}

	audioParts := []string{}
	if s := formatBitrateCodec(m.streamInfo, m.styles); s != "" {
		audioParts = append(audioParts, s)
	}
	if s := formatSampleRate(m.streamInfo, m.styles); s != "" {
		audioParts = append(audioParts, s)
	}

	groups := []string{}
	if len(audioParts) > 0 {
		groups = append(groups, strings.Join(audioParts, "  "))
	}
	if s := formatUptime(m.playStartedAt, m.nowTime, m.styles); s != "" {
		groups = append(groups, s)
	}
	if s := renderBufferBar(m.cacheSeconds, m.styles); s != "" {
		groups = append(groups, s)
	}
	if len(groups) == 0 {
		return ""
	}
	sep := "  " + m.styles.HelpSep.Render("·") + "  "
	return leftPad + strings.Join(groups, sep)
}

func formatBitrateCodec(si audio.StreamInfoChanged, s Styles) string {
	var parts []string
	if si.Bitrate > 0 {
		parts = append(parts, fmt.Sprintf("%dk", si.Bitrate/1000))
	}
	if si.Codec != "" {
		parts = append(parts, si.Codec)
	}
	if len(parts) == 0 {
		return ""
	}
	return s.Hint.Render(strings.Join(parts, " "))
}

func formatSampleRate(si audio.StreamInfoChanged, s Styles) string {
	var parts []string
	if si.SampleRate > 0 {
		parts = append(parts, fmt.Sprintf("%.1fk", float64(si.SampleRate)/1000))
	}
	switch si.Channels {
	case 0:
	case 1:
		parts = append(parts, "mono")
	case 2:
		parts = append(parts, "stereo")
	default:
		parts = append(parts, fmt.Sprintf("%dch", si.Channels))
	}
	if len(parts) == 0 {
		return ""
	}
	return s.Hint.Render(strings.Join(parts, " "))
}

// formatUptime renders the play-session age as a media-player style
// timestamp: "0:32", "1:05", "12:34", "1:23:45". Unambiguous as time
// (no "s"/"m" suffixes that could collide visually with the buffer's
// seconds value next door).
func formatUptime(started, now time.Time, s Styles) string {
	if started.IsZero() {
		return ""
	}
	if now.IsZero() {
		now = time.Now()
	}
	d := now.Sub(started)
	if d < time.Second {
		return ""
	}
	total := int(d.Seconds())
	hours := total / 3600
	mins := (total / 60) % 60
	secs := total % 60
	var label string
	if hours > 0 {
		label = fmt.Sprintf("%d:%02d:%02d", hours, mins, secs)
	} else {
		label = fmt.Sprintf("%d:%02d", mins, secs)
	}
	return s.Hint.Render(label)
}

// renderBufferBar shows demuxer cache health. The fill stays in the
// muted tone of the rest of the info row in healthy conditions; once
// the cache drops under bufferLowSeconds the fill switches to the
// warning hue, which is what makes the bar grab the user's attention
// only when a hiccup is impending. The "↯" prefix gives the bar
// context — without it the bar reads as a generic gauge.
func renderBufferBar(sec float64, s Styles) string {
	if sec < bufferShowSeconds {
		return ""
	}
	fill := int(sec * float64(bufferBarCells) / bufferFullSeconds)
	if fill > bufferBarCells {
		fill = bufferBarCells
	}
	if fill < 0 {
		fill = 0
	}
	fillStyle := s.Hint
	if sec < bufferLowSeconds {
		fillStyle = s.BufferLow
	}
	return s.Hint.Render("↯ ") +
		fillStyle.Render(strings.Repeat("▰", fill)) +
		s.VolEmpty.Render(strings.Repeat("▱", bufferBarCells-fill))
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

// renderLogo renders the static "lofi" ASCII art with a soft
// shimmer wave sweeping across it. Returns "" when no station is
// selected so the right-side area collapses cleanly back to just
// the now-playing block in that state.
func (m Model) renderLogo() string {
	if m.playingIdx < 0 || m.playingIdx >= len(m.cfg.Stations) {
		return ""
	}

	width := lipgloss.Width(logoLines[0])
	crest := m.logo.crestColumn(width)

	var out strings.Builder
	for li, line := range logoLines {
		if li > 0 {
			out.WriteByte('\n')
		}
		col := 0
		for _, r := range line {
			if r == ' ' {
				out.WriteRune(' ')
				col++
				continue
			}
			out.WriteString(logoCellStyle(m.styles, col-crest).Render(string(r)))
			col++
		}
	}
	return out.String()
}

// logoCellStyle picks a colour band by signed distance from the
// shimmer crest: 0 is the bright peak, ±1..±halo is the soft halo,
// the rest stays on the muted base — three soft bands with no hard
// edge to the lit zone.
func logoCellStyle(s Styles, dist int) lipgloss.Style {
	if dist < 0 {
		dist = -dist
	}
	switch {
	case dist == 0:
		return s.LogoCrest
	case dist <= logoShimmerHalo:
		return s.LogoMid
	default:
		return s.LogoBase
	}
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

// stationsViewportHeight caps how many station rows the list shows at
// once. Above this the list scrolls with the cursor, with `↑/↓ N more`
// hints so the user can tell something is hidden.
const stationsViewportHeight = 8

func (m Model) renderStations() string {
	if len(m.cfg.Stations) == 0 {
		return leftPad + "    " +
			m.styles.StationCursor.Render("press a") + " " +
			m.styles.Hint.Render("to add one")
	}

	n := len(m.cfg.Stations)
	start, end := 0, n
	if n > stationsViewportHeight {
		start = m.cursor - stationsViewportHeight/2
		if start < 0 {
			start = 0
		}
		if start+stationsViewportHeight > n {
			start = n - stationsViewportHeight
		}
		end = start + stationsViewportHeight
	}

	var b strings.Builder
	if start > 0 {
		b.WriteString(leftPad + "    " +
			m.styles.Hint.Render(fmt.Sprintf("↑ %d more", start)))
		b.WriteString("\n")
	}

	for i := start; i < end; i++ {
		s := m.cfg.Stations[i]
		// Three-cell prefix: cursor chevron + space + playing-status dot.
		// The dot column is consistent so names line up regardless of
		// which row is selected or playing.
		cursor := "  "
		if i == m.cursor {
			cursor = m.styles.Cursor.Render("›") + " "
		}

		marker := " "
		if i == m.playingIdx {
			marker = m.statusBlock()
		}

		unavailable := s.IsYouTube() && !m.youtubeReady

		var name string
		switch {
		case unavailable:
			name = m.styles.Hint.Render(s.Name)
		case i == m.cursor:
			name = m.styles.Cursor.Render(s.Name)
		case i == m.playingIdx:
			name = m.styles.StationPlaying.Render(s.Name)
		default:
			name = m.styles.StationItem.Render(s.Name)
		}

		line := leftPad + cursor + marker + " " + name
		if badge := m.stationKindBadge(s); badge != "" {
			line += "  " + badge
		}
		if unavailable {
			line += "  " + m.styles.Hint.Render("unavailable")
		}
		if i < end-1 || end < n {
			line += "\n"
		}
		b.WriteString(line)
	}

	if end < n {
		b.WriteString(leftPad + "    " +
			m.styles.Hint.Render(fmt.Sprintf("↓ %d more", n-end)))
	}

	return b.String()
}

// stationKindBadge returns a small marker for non-default station
// kinds. Direct streams (the common case) get nothing — the absence
// reads as default. YouTube gets a play triangle; future kinds fall
// back to the kind name.
func (m Model) stationKindBadge(s config.Station) string {
	kind := s.EffectiveKind()
	if kind == "" || kind == config.KindStream {
		return ""
	}
	if kind == config.KindYouTube {
		return m.styles.SectionHeader.Render("▶")
	}
	return m.styles.SectionHeader.Render(kind)
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

	// Three group columns sit side-by-side instead of stacking
	// vertically, collapsing the card from ~22 rows to ~6.
	cols := make([]string, 0, len(groups))
	for i, g := range groups {
		var col strings.Builder
		if i < len(labels) {
			col.WriteString(m.styles.HelpGroup.Render(labels[i]))
			col.WriteString("\n")
		}
		first := true
		for _, binding := range g {
			h := binding.Help()
			if h.Key == "" || h.Desc == "" {
				continue
			}
			if !first {
				col.WriteString("\n")
			}
			first = false
			col.WriteString(
				m.styles.HelpKey.Render(fmt.Sprintf("%-6s", h.Key)) + " " +
					m.styles.HelpDesc.Render(h.Desc))
		}
		cols = append(cols, col.String())
	}

	gap := "   "
	row := lipgloss.JoinHorizontal(lipgloss.Top, interleave(cols, gap)...)

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Muted).
		Padding(0, 2).
		Render(row)

	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, card)
}

// interleave returns ss with sep inserted between every pair of
// elements, so JoinHorizontal can render gap columns without us
// hand-writing each n-1 spacing.
func interleave(ss []string, sep string) []string {
	if len(ss) == 0 {
		return ss
	}
	out := make([]string, 0, len(ss)*2-1)
	for i, s := range ss {
		if i > 0 {
			out = append(out, sep)
		}
		out = append(out, s)
	}
	return out
}

// renderAmbientIndicator returns a compact "· 🌧️🔥" tag composed of
// active-channel icons in canonical order, separated from the station
// kind label by a divider in muted tone. Returns empty when no
// ambient channel is active so the station line stays uncluttered.
func (m Model) renderAmbientIndicator() string {
	if m.mixer == nil {
		return ""
	}
	ids := m.mixer.ActiveIDs()
	if len(ids) == 0 {
		return ""
	}
	var icons []string
	for _, id := range ids {
		ch, ok := m.mixer.Channel(id)
		if !ok {
			continue
		}
		icons = append(icons, ch.Icon)
	}
	if len(icons) == 0 {
		return ""
	}
	return m.styles.SectionHeader.Render("· ") + m.styles.AppTitle.Render(strings.Join(icons, " "))
}
