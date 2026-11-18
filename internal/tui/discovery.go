package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

	"github.com/iRootPro/lofi-player/internal/i18n"
	"github.com/iRootPro/lofi-player/internal/radiobrowser"
)

const discoveryMinQueryRunes = 2

type discoveryFocus int

const (
	discoveryInputFocus discoveryFocus = iota
	discoveryResultsFocus
)

// discoveryModel holds one transient public-catalog search session.
type discoveryModel struct {
	input        textinput.Model
	focus        discoveryFocus
	results      []radiobrowser.Station
	cursor       int
	details      bool
	detailScroll int
	loading      bool
	err          string
	cached       bool
	notice       string
	noticeKind   ToastKind
	searchSeq    int
	session      int
}

// discoveryPlayback is the transient playback state presented inside the
// search modal. Keeping it close to the catalog result makes Enter's effect
// visible even though the full now-playing card is behind the overlay.
type discoveryPlayback struct {
	stationName string
	streamURL   string
	loading     bool
	playing     bool
}

func newDiscoveryModel(session int) discoveryModel {
	input := textinput.New()
	input.Placeholder = "ambient, tag:ambient, country:JP…"
	input.CharLimit = 120
	input.Width = 52
	input.Prompt = ""
	input.Focus()
	return discoveryModel{input: input, session: session}
}

func (d discoveryModel) selected() (radiobrowser.Station, bool) {
	if d.cursor < 0 || d.cursor >= len(d.results) {
		return radiobrowser.Station{}, false
	}
	return d.results[d.cursor], true
}

func (d discoveryModel) setFocus(next discoveryFocus) discoveryModel {
	if next == discoveryResultsFocus && len(d.results) == 0 {
		return d
	}
	d.focus = next
	if next == discoveryInputFocus {
		d.input.Focus()
	} else {
		d.input.Blur()
	}
	return d
}

func (d discoveryModel) view(locale i18n.Locale, width, height int, styles Styles, muted lipgloss.Color, playback discoveryPlayback, loadingIndicator string) string {
	if d.details {
		return d.detailsViewFor(locale, width, height, styles, muted)
	}

	contentWidth := discoveryContentWidth(width)
	input := d.input
	input.Width = max(12, contentWidth-lipgloss.Width("search  "))
	inputStyle := styles.HelpDesc
	if d.focus == discoveryInputFocus {
		inputStyle = styles.Cursor
	}

	var body strings.Builder
	body.WriteString(styles.SectionHeader.Render(i18n.T(locale, "discover_radio")))
	body.WriteString("\n\n")
	body.WriteString(inputStyle.Render(i18n.T(locale, "search")))
	body.WriteString(input.View())
	if playback.streamURL != "" {
		body.WriteString("\n\n")
		body.WriteString(renderDiscoveryPlayback(locale, playback, loadingIndicator, styles))
	}
	if d.notice != "" {
		body.WriteString("\n\n")
		body.WriteString(renderDiscoveryNotice(locale, d.notice, d.noticeKind, styles))
	}

	if d.loading {
		body.WriteString("\n\n")
		body.WriteString(styles.Hint.Render(i18n.T(locale, "searching")))
	} else if d.err != "" {
		body.WriteString("\n\n")
		body.WriteString(styles.StationCursor.Render(i18n.T(locale, "error")))
		body.WriteString(styles.HelpDesc.Render(d.err))
	} else if d.cached {
		body.WriteString("\n\n")
		body.WriteString(styles.Hint.Render(i18n.T(locale, "cached_results")))
	} else if utf8.RuneCountInString(strings.TrimSpace(d.input.Value())) < discoveryMinQueryRunes {
		body.WriteString("\n\n")
		body.WriteString(styles.Hint.Render(i18n.T(locale, "type_two")))
	} else if len(d.results) == 0 {
		body.WriteString("\n\n")
		body.WriteString(styles.Hint.Render(i18n.T(locale, "no_stations")))
	}

	if len(d.results) > 0 {
		body.WriteString("\n\n")
		limit := len(d.results)
		if limit > 7 {
			limit = 7
		}
		start := 0
		if d.cursor >= limit {
			start = d.cursor - limit + 1
		}
		if start+limit > len(d.results) {
			start = len(d.results) - limit
		}
		for i := start; i < start+limit; i++ {
			station := d.results[i]
			selected := i == d.cursor && d.focus == discoveryResultsFocus
			active := playback.streamURL != "" && station.StreamURL == playback.streamURL
			prefix := "  "
			nameStyle := styles.StationItem
			if selected {
				prefix = styles.Cursor.Render("›") + " "
				nameStyle = styles.Cursor
			} else if active {
				prefix = styles.StatusLive.Render("▶") + " "
				nameStyle = styles.StationPlaying
			}
			const minNameWidth = 4
			metaBudget := max(0, contentWidth-lipgloss.Width(prefix)-minNameWidth-lipgloss.Width("  "))
			meta := discoveryRowMeta(station, metaBudget)
			if active {
				playbackMeta := discoveryRowPlaybackLabel(locale, playback)
				metaBudget -= lipgloss.Width(playbackMeta)
				if meta != "" {
					metaBudget -= lipgloss.Width(" · ")
				}
				meta = discoveryRowMeta(station, max(0, metaBudget))
				if meta != "" {
					meta += " · "
				}
				meta += playbackMeta
			}
			nameBudget := contentWidth - lipgloss.Width(prefix)
			if meta != "" {
				nameBudget -= lipgloss.Width("  " + meta)
			}
			if nameBudget < 4 {
				nameBudget = 4
			}
			body.WriteString(prefix)
			body.WriteString(nameStyle.Render(truncateRunes(station.Name, nameBudget)))
			if meta != "" {
				body.WriteString(styles.Hint.Render("  " + meta))
			}
			if i < start+limit-1 {
				body.WriteByte('\n')
			}
		}
		if len(d.results) > limit {
			body.WriteString("\n")
			body.WriteString(styles.Hint.Render(i18n.T(locale, "more_results", len(d.results)-limit)))
		}
		if station, ok := d.selected(); ok {
			body.WriteString("\n\n")
			body.WriteString(renderDiscoverySummary(locale, station, contentWidth, styles))
		}
	}

	body.WriteString("\n\n")
	body.WriteString(styles.HelpKey.Render("↓") + " " + styles.HelpDesc.Render(i18n.T(locale, "results")) + "  " +
		styles.HelpSep.Render("·") + "  " +
		styles.HelpKey.Render("↑") + " " + styles.HelpDesc.Render(i18n.T(locale, "input")) + "  " +
		styles.HelpSep.Render("·") + "  " +
		styles.HelpKey.Render("jk/↑↓") + " " + styles.HelpDesc.Render(i18n.T(locale, "select")) + "  " +
		styles.HelpSep.Render("·") + "  " +
		styles.HelpKey.Render("enter") + " " + styles.HelpDesc.Render(i18n.T(locale, "play")) + "  " +
		styles.HelpSep.Render("·") + "  " +
		styles.HelpKey.Render("s") + " " + styles.HelpDesc.Render(i18n.T(locale, "save")) + "  " +
		styles.HelpSep.Render("·") + "  " +
		styles.HelpKey.Render("v") + " " + styles.HelpDesc.Render(i18n.T(locale, "details")) + "  " +
		styles.HelpSep.Render("·") + "  " +
		styles.HelpKey.Render("esc") + " " + styles.HelpDesc.Render(i18n.T(locale, "close")))

	card := lipgloss.NewStyle().
		Width(contentWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(muted).
		Padding(1, 3).
		Render(body.String())
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, card)
}

func renderDiscoveryPlayback(locale i18n.Locale, playback discoveryPlayback, loadingIndicator string, styles Styles) string {
	if playback.loading {
		return styles.Cursor.Render(loadingIndicator) + "  " +
			styles.StationName.Render(i18n.T(locale, "loading_station", truncateRunes(playback.stationName, 42))) + "  " +
			styles.Hint.Render(i18n.T(locale, "not_saved"))
	}
	if playback.playing {
		return styles.StatusLive.Render(statusGlyphLive) + "  " +
			styles.StationPlaying.Render(i18n.T(locale, "playing_station", truncateRunes(playback.stationName, 42))) + "  " +
			styles.Hint.Render(i18n.T(locale, "not_saved"))
	}
	return styles.HelpDesc.Render(truncateRunes(playback.stationName, 42)) + "  " + styles.Hint.Render(i18n.T(locale, "not_saved"))
}

func discoveryRowPlaybackLabel(locale i18n.Locale, playback discoveryPlayback) string {
	if playback.loading {
		return i18n.T(locale, "loading")
	}
	if playback.playing {
		return i18n.T(locale, "playing")
	}
	return i18n.T(locale, "paused")
}

func renderDiscoveryNotice(locale i18n.Locale, message string, kind ToastKind, styles Styles) string {
	switch kind {
	case ToastSuccess:
		return styles.StatusLive.Render(i18n.T(locale, "success_label")) + styles.StationPlaying.Render(message)
	case ToastError:
		return styles.StationCursor.Render(i18n.T(locale, "error_label")) + styles.HelpDesc.Render(message)
	default:
		return styles.HelpKey.Render(i18n.T(locale, "note_label")) + styles.HelpDesc.Render(message)
	}
}

// discoveryContentWidth reserves the discovery card's border and horizontal
// padding. It expands on wide terminals without exceeding the app frame.
func discoveryContentWidth(width int) int {
	if width <= 0 {
		return 72
	}
	available := width - 8 // rounded border plus three cells of padding per side
	if available < 24 {
		return 24
	}
	if available > 96 {
		return 96
	}
	return available
}

func renderDiscoverySummary(locale i18n.Locale, station radiobrowser.Station, width int, styles Styles) string {
	var body strings.Builder
	body.WriteString(styles.Hint.Render(i18n.T(locale, "selected")))
	body.WriteString(styles.StationName.Render(truncateRunes(station.Name, max(4, width-10))))
	if meta := discoveryRowMeta(station, width); meta != "" {
		body.WriteString("\n")
		body.WriteString(styles.Hint.Render(meta))
	}
	if station.Tags != "" {
		body.WriteString("\n")
		body.WriteString(styles.Hint.Render(i18n.T(locale, "tags") + truncateRunes(station.Tags, max(4, width-6))))
	}
	body.WriteString("\n")
	body.WriteString(styles.Hint.Render(i18n.T(locale, "press_v")))
	return body.String()
}

func (d discoveryModel) detailsView(width, height int, styles Styles, muted lipgloss.Color) string {
	return d.detailsViewFor(i18n.EN, width, height, styles, muted)
}

func (d discoveryModel) detailsViewFor(locale i18n.Locale, width, height int, styles Styles, muted lipgloss.Color) string {
	station, ok := d.selected()
	if !ok {
		return ""
	}
	contentWidth := discoveryContentWidth(width)
	lines := discoveryDetailLinesFor(locale, station, contentWidth)
	visible := discoveryDetailViewportRows(height)
	maxScroll := max(0, len(lines)-visible)
	scroll := min(d.detailScroll, maxScroll)
	compact := discoveryDetailCompact(height)

	var body strings.Builder
	body.WriteString(styles.SectionHeader.Render(i18n.T(locale, "station_details")))
	if compact {
		body.WriteByte('\n')
	} else {
		body.WriteString("\n\n")
	}
	for i := scroll; i < min(scroll+visible, len(lines)); i++ {
		if i > scroll {
			body.WriteByte('\n')
		}
		if i == 0 {
			body.WriteString(styles.StationName.Render(lines[i]))
		} else if strings.HasPrefix(lines[i], i18n.T(locale, "tags")) {
			body.WriteString(styles.Hint.Render(lines[i]))
		} else {
			body.WriteString(styles.HelpDesc.Render(lines[i]))
		}
	}
	if maxScroll > 0 && !compact {
		body.WriteString("\n\n")
		body.WriteString(styles.Hint.Render(i18n.T(locale, "lines_of", scroll+1, min(scroll+visible, len(lines)), len(lines))))
	}
	if compact {
		body.WriteString("\n")
		body.WriteString(styles.HelpKey.Render("esc") + " " + styles.HelpDesc.Render(i18n.T(locale, "back")))
	} else {
		body.WriteString("\n\n")
		body.WriteString(styles.HelpKey.Render("jk/↑↓") + " " + styles.HelpDesc.Render(i18n.T(locale, "scroll")) + "  " +
			styles.HelpSep.Render("·") + "  " +
			styles.HelpKey.Render("v / esc") + " " + styles.HelpDesc.Render(i18n.T(locale, "back")))
	}

	cardStyle := lipgloss.NewStyle().
		Width(contentWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(muted)
	if compact {
		cardStyle = cardStyle.Padding(0, 1)
	} else {
		cardStyle = cardStyle.Padding(1, 3)
	}
	card := cardStyle.Render(body.String())
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, card)
}

func discoveryDetailLines(station radiobrowser.Station, width int) []string {
	return discoveryDetailLinesFor(i18n.EN, station, width)
}

func discoveryDetailLinesFor(locale i18n.Locale, station radiobrowser.Station, width int) []string {
	lines := wrapRunes(station.Name, width)
	if meta := discoveryFullMeta(station); meta != "" {
		lines = append(lines, wrapRunes(meta, width)...)
	}
	if station.Tags != "" {
		for i, line := range wrapRunes(station.Tags, max(4, width-6)) {
			if i == 0 {
				lines = append(lines, i18n.T(locale, "tags")+line)
			} else {
				lines = append(lines, "      "+line)
			}
		}
	}
	return lines
}

func discoveryDetailViewportRows(height int) int {
	if height <= 0 {
		return 8
	}
	if discoveryDetailCompact(height) {
		return max(1, height-4)
	}
	return min(12, max(1, height-12))
}

func discoveryDetailCompact(height int) bool {
	return height > 0 && height < 14
}

func (d discoveryModel) detailMaxScroll(windowWidth, height int) int {
	station, ok := d.selected()
	if !ok {
		return 0
	}
	frameWidth := appFrameWidth
	if frameWidth > windowWidth-2 {
		frameWidth = windowWidth - 2
	}
	if frameWidth < 40 {
		frameWidth = 40
	}
	contentWidth := discoveryContentWidth(frameWidth - 2)
	return max(0, len(discoveryDetailLines(station, contentWidth))-discoveryDetailViewportRows(height))
}

// wrapRunes splits text into display-width bounded lines without splitting a
// UTF-8 rune. It favours whitespace boundaries but also breaks a single long
// word, so untrusted catalog metadata cannot widen the modal.
func wrapRunes(s string, width int) []string {
	if width < 1 {
		return nil
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	lines := make([]string, 0, len(words))
	line := ""
	flush := func() {
		if line != "" {
			lines = append(lines, line)
			line = ""
		}
	}
	for _, word := range words {
		if line != "" && lipgloss.Width(line+" "+word) <= width {
			line += " " + word
			continue
		}
		flush()
		for lipgloss.Width(word) > width {
			part := truncateRunes(word, width)
			part = strings.TrimSuffix(part, "…")
			if part == "" {
				break
			}
			lines = append(lines, part)
			word = strings.TrimPrefix(word, part)
		}
		line = word
	}
	flush()
	return lines
}

func discoveryRowMeta(station radiobrowser.Station, width int) string {
	if width < 1 {
		return ""
	}
	parts := make([]string, 0, 2)
	if station.CountryCode != "" {
		parts = append(parts, truncateRunes(station.CountryCode, 4))
	}
	if station.Codec != "" {
		codec := station.Codec
		if station.Bitrate > 0 {
			codec = fmt.Sprintf("%s %dk", station.Codec, station.Bitrate)
		}
		parts = append(parts, truncateRunes(codec, 18))
	}
	return truncateRunes(strings.Join(parts, " · "), width)
}

func discoveryFullMeta(station radiobrowser.Station) string {
	parts := make([]string, 0, 2)
	if station.CountryCode != "" {
		parts = append(parts, station.CountryCode)
	}
	if station.Codec != "" {
		if station.Bitrate > 0 {
			parts = append(parts, fmt.Sprintf("%s %dk", station.Codec, station.Bitrate))
		} else {
			parts = append(parts, station.Codec)
		}
	}
	return strings.Join(parts, " · ")
}
