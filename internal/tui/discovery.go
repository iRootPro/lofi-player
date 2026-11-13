package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

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
	input      textinput.Model
	focus      discoveryFocus
	results    []radiobrowser.Station
	cursor     int
	loading    bool
	err        string
	cached     bool
	notice     string
	noticeKind ToastKind
	searchSeq  int
	session    int
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

func (d discoveryModel) view(width int, styles Styles, muted lipgloss.Color, playback discoveryPlayback, loadingIndicator string) string {
	inputStyle := styles.HelpDesc
	if d.focus == discoveryInputFocus {
		inputStyle = styles.Cursor
	}

	var body strings.Builder
	body.WriteString(styles.SectionHeader.Render("─── discover radio ───"))
	body.WriteString("\n\n")
	body.WriteString(inputStyle.Render("search  "))
	body.WriteString(d.input.View())
	if playback.streamURL != "" {
		body.WriteString("\n\n")
		body.WriteString(renderDiscoveryPlayback(playback, loadingIndicator, styles))
	}
	if d.notice != "" {
		body.WriteString("\n\n")
		body.WriteString(renderDiscoveryNotice(d.notice, d.noticeKind, styles))
	}

	if d.loading {
		body.WriteString("\n\n")
		body.WriteString(styles.Hint.Render("searching Radio Browser…"))
	} else if d.err != "" {
		body.WriteString("\n\n")
		body.WriteString(styles.StationCursor.Render("error: "))
		body.WriteString(styles.HelpDesc.Render(d.err))
	} else if d.cached {
		body.WriteString("\n\n")
		body.WriteString(styles.Hint.Render("cached results — Radio Browser is unavailable"))
	} else if utf8.RuneCountInString(strings.TrimSpace(d.input.Value())) < discoveryMinQueryRunes {
		body.WriteString("\n\n")
		body.WriteString(styles.Hint.Render("type at least 2 characters; results appear automatically"))
	} else if len(d.results) == 0 {
		body.WriteString("\n\n")
		body.WriteString(styles.Hint.Render("no stations found"))
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
			meta := discoveryMeta(station)
			if active {
				if meta != "" {
					meta += " · "
				}
				meta += discoveryRowPlaybackLabel(playback)
			}
			nameBudget := 54 - lipgloss.Width(prefix) - lipgloss.Width(meta)
			if nameBudget < 12 {
				nameBudget = 12
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
			body.WriteString(styles.Hint.Render(fmt.Sprintf("%d more results", len(d.results)-limit)))
		}
	}

	body.WriteString("\n\n")
	body.WriteString(styles.HelpKey.Render("↓") + " " + styles.HelpDesc.Render("results") + "  " +
		styles.HelpSep.Render("·") + "  " +
		styles.HelpKey.Render("↑") + " " + styles.HelpDesc.Render("input") + "  " +
		styles.HelpSep.Render("·") + "  " +
		styles.HelpKey.Render("jk/↑↓") + " " + styles.HelpDesc.Render("select") + "  " +
		styles.HelpSep.Render("·") + "  " +
		styles.HelpKey.Render("enter") + " " + styles.HelpDesc.Render("play") + "  " +
		styles.HelpSep.Render("·") + "  " +
		styles.HelpKey.Render("s") + " " + styles.HelpDesc.Render("save") + "  " +
		styles.HelpSep.Render("·") + "  " +
		styles.HelpKey.Render("esc") + " " + styles.HelpDesc.Render("close"))

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(muted).
		Padding(1, 3).
		Render(body.String())
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, card)
}

func renderDiscoveryPlayback(playback discoveryPlayback, loadingIndicator string, styles Styles) string {
	if playback.loading {
		return styles.Cursor.Render(loadingIndicator) + "  " +
			styles.StationName.Render("Loading "+truncateRunes(playback.stationName, 42)+"…") + "  " +
			styles.Hint.Render("not saved")
	}
	if playback.playing {
		return styles.StatusLive.Render(statusGlyphLive) + "  " +
			styles.StationPlaying.Render("Playing "+truncateRunes(playback.stationName, 42)) + "  " +
			styles.Hint.Render("not saved")
	}
	return styles.HelpDesc.Render("Paused "+truncateRunes(playback.stationName, 42)) + "  " +
		styles.Hint.Render("not saved")
}

func discoveryRowPlaybackLabel(playback discoveryPlayback) string {
	if playback.loading {
		return "loading…"
	}
	if playback.playing {
		return "playing"
	}
	return "paused"
}

func renderDiscoveryNotice(message string, kind ToastKind, styles Styles) string {
	switch kind {
	case ToastSuccess:
		return styles.StatusLive.Render("ok: ") + styles.StationPlaying.Render(message)
	case ToastError:
		return styles.StationCursor.Render("error: ") + styles.HelpDesc.Render(message)
	default:
		return styles.HelpKey.Render("note: ") + styles.HelpDesc.Render(message)
	}
}

func discoveryMeta(station radiobrowser.Station) string {
	parts := make([]string, 0, 3)
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
	if station.Tags != "" {
		parts = append(parts, truncateRunes(station.Tags, 20))
	}
	return strings.Join(parts, " · ")
}
