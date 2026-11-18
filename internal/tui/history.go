package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	listenhistory "github.com/iRootPro/lofi-player/internal/history"
)

const historyCheckpointInterval = time.Minute

type historyPeriod int

const (
	historyToday historyPeriod = iota
	historyWeek
	historyAllTime
)

type historyModel struct {
	period          historyPeriod
	confirmingClear bool
}

type historyStationTotal struct {
	name     string
	duration time.Duration
}

func (m Model) openHistory() Model {
	m.modePrev = m.mode
	m.mode = modeHistory
	m.historyUI.confirmingClear = false
	return m
}

func (m Model) historyNow() time.Time {
	if !m.nowTime.IsZero() {
		return m.nowTime
	}
	return time.Now()
}

func (m Model) beginListening(at time.Time) Model {
	station, ok := m.activeStation()
	if !ok {
		return m
	}
	source := listenhistory.SourceLibrary
	if m.previewStation != nil {
		source = listenhistory.SourceDiscovery
	}
	m.listeningHistory = m.listeningHistory.Begin(station.Name, source, at)
	m.historySavedAt = at
	return m
}

func (m Model) pauseListening(at time.Time) Model {
	m.listeningHistory = m.listeningHistory.Pause(at)
	m.historySavedAt = at
	return m
}

func (m Model) endListening(at time.Time, reason string) Model {
	m.listeningHistory = m.listeningHistory.End(at, reason)
	m.historySavedAt = at
	return m
}

func (m Model) persistListeningHistory() (Model, tea.Cmd) {
	if m.saveHistory == nil {
		return m, nil
	}
	if err := m.saveHistory(m.listeningHistory); err != nil {
		// A history write must never interrupt playback. Preserve a more
		// specific playback error if one is already visible.
		if m.toast == nil {
			m.toast = &Toast{Message: "Could not save listening history.", Kind: ToastError}
			return m, clearToastAfter()
		}
	}
	return m, nil
}

func (m Model) updateHistory(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if key.Matches(keyMsg, m.keys.Quit) {
		return m, tea.Quit
	}

	if m.historyUI.confirmingClear {
		switch keyMsg.String() {
		case "y", "Y", "enter":
			now := m.historyNow()
			m.listeningHistory = m.listeningHistory.Clear(now)
			m.historySavedAt = now
			m.historyUI.confirmingClear = false
			m, cmd := m.persistListeningHistory()
			if m.toast == nil {
				m.toast = &Toast{Message: "listening history cleared", Kind: ToastSuccess}
				return m, tea.Batch(cmd, clearToastAfter())
			}
			return m, cmd
		case "esc", "n", "N":
			m.historyUI.confirmingClear = false
		}
		return m, nil
	}

	switch keyMsg.String() {
	case "esc", "H":
		m.mode = m.modePrev
		return m, nil
	case "tab", "right", "l":
		m.historyUI.period = (m.historyUI.period + 1) % 3
		return m, nil
	case "shift+tab", "left", "h":
		m.historyUI.period = (m.historyUI.period + 2) % 3
		return m, nil
	case "C":
		m.historyUI.confirmingClear = true
		return m, nil
	}
	return m, nil
}

func (m Model) viewHistory() string {
	backdrop := m.modalBackdrop()
	now := m.historyNow()
	contentWidth := discoveryContentWidth(m.width)
	compact := m.height > 0 && m.height < 20

	if m.historyUI.confirmingClear {
		return m.viewHistoryClearConfirm(contentWidth)
	}

	from, to, label := historyPeriodBounds(m.historyUI.period, now)
	total := m.listeningHistory.DurationBetween(from, to, now)
	sessions := m.historySessionCount(from, to, now)
	top := m.historyTopStations(from, to, now)
	recentLimit := 4
	if compact {
		recentLimit = 2
	}

	var body strings.Builder
	body.WriteString(m.styles.SectionHeader.Render("─── listening history ───"))
	body.WriteString("\n\n")
	body.WriteString(renderHistoryTabs(m.historyUI.period, m.styles))
	body.WriteString("\n")
	body.WriteString(m.styles.HelpDesc.Render(fmt.Sprintf("%s  %s", label, formatHistoryDuration(total))))
	body.WriteString(m.styles.Hint.Render(fmt.Sprintf("  ·  %d sessions", sessions)))

	if m.listeningHistory.Active != nil && !compact {
		body.WriteString("\n")
		active := m.listeningHistory.Active
		body.WriteString(m.styles.Hint.Render("now playing  "))
		body.WriteString(m.styles.StationName.Render(truncateRunes(active.StationName, max(4, contentWidth-16))))
	}

	if len(top) > 0 {
		body.WriteString("\n\n")
		body.WriteString(m.styles.HelpKey.Render("top stations"))
		for i, station := range top {
			body.WriteString("\n")
			prefix := fmt.Sprintf("%d. ", i+1)
			duration := formatHistoryDuration(station.duration)
			nameWidth := max(4, contentWidth-lipgloss.Width(prefix)-lipgloss.Width(duration)-2)
			body.WriteString(m.styles.HelpDesc.Render(prefix))
			body.WriteString(m.styles.StationItem.Render(truncateRunes(station.name, nameWidth)))
			body.WriteString(m.styles.Hint.Render("  " + duration))
		}
	}

	recent := m.historyRecent(recentLimit)
	if len(recent) > 0 {
		body.WriteString("\n\n")
		body.WriteString(m.styles.HelpKey.Render("recent"))
		for _, session := range recent {
			body.WriteString("\n")
			when := formatHistoryWhen(session.EndedAt, now)
			duration := formatHistoryDuration(time.Duration(session.PlayedSeconds) * time.Second)
			prefix := when + "  "
			nameWidth := max(4, contentWidth-lipgloss.Width(prefix)-lipgloss.Width(duration)-2)
			body.WriteString(m.styles.Hint.Render(prefix))
			body.WriteString(m.styles.StationItem.Render(truncateRunes(session.StationName, nameWidth)))
			body.WriteString(m.styles.Hint.Render("  " + duration))
		}
	}

	if len(top) == 0 && len(recent) == 0 && m.listeningHistory.Active == nil {
		body.WriteString("\n\n")
		body.WriteString(m.styles.Hint.Render("History appears after 30 seconds of confirmed playback."))
	}

	body.WriteString("\n\n")
	body.WriteString(m.styles.HelpKey.Render("←/→") + " " + m.styles.HelpDesc.Render("period") + "  " +
		m.styles.HelpSep.Render("·") + "  " +
		m.styles.HelpKey.Render("C") + " " + m.styles.HelpDesc.Render("clear") + "  " +
		m.styles.HelpSep.Render("·") + "  " +
		m.styles.HelpKey.Render("esc") + " " + m.styles.HelpDesc.Render("close"))

	card := lipgloss.NewStyle().
		Width(contentWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Muted).
		Padding(1, 3).
		Render(body.String())
	return overlayModal(backdrop, lipgloss.PlaceHorizontal(m.width, lipgloss.Center, card))
}

func (m Model) viewHistoryClearConfirm(contentWidth int) string {
	// Layer confirmation over the already-visible dashboard so Esc has a
	// predictable visual return target rather than abruptly replacing it.
	dashboard := m
	dashboard.historyUI.confirmingClear = false
	backdrop := dashboard.viewHistory()
	inner := m.styles.SectionHeader.Render("─── clear listening history? ───") + "\n\n" +
		m.styles.HelpDesc.Render("This permanently removes local completed sessions.") + "\n" +
		m.styles.Hint.Render("Current playback continues as a fresh session.") + "\n\n" +
		m.styles.HelpKey.Render("y / enter") + " " + m.styles.HelpDesc.Render("clear") + "  " +
		m.styles.HelpSep.Render("·") + "  " +
		m.styles.HelpKey.Render("esc") + " " + m.styles.HelpDesc.Render("cancel")
	card := lipgloss.NewStyle().
		Width(contentWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Muted).
		Padding(1, 3).
		Render(inner)
	return overlayModal(backdrop, lipgloss.PlaceHorizontal(m.width, lipgloss.Center, card))
}

func historyPeriodBounds(period historyPeriod, now time.Time) (time.Time, time.Time, string) {
	localNow := now.In(time.Local)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.Local)
	switch period {
	case historyWeek:
		return today.AddDate(0, 0, -6), today.AddDate(0, 0, 1), "last 7 days"
	case historyAllTime:
		return time.Time{}, now.Add(24 * time.Hour), "all time"
	default:
		return today, today.AddDate(0, 0, 1), "today"
	}
}

func renderHistoryTabs(active historyPeriod, styles Styles) string {
	labels := []string{"today", "7 days", "all time"}
	parts := make([]string, 0, len(labels))
	for i, label := range labels {
		if historyPeriod(i) == active {
			parts = append(parts, styles.Cursor.Render("["+label+"]"))
		} else {
			parts = append(parts, styles.Hint.Render(label))
		}
	}
	return strings.Join(parts, "  ")
}

func (m Model) historyTopStations(from, to, now time.Time) []historyStationTotal {
	totals := make(map[string]time.Duration)
	add := func(name string, segments []listenhistory.Segment) {
		for _, segment := range segments {
			start, end := maxHistoryTime(segment.StartedAt, from), minHistoryTime(segment.EndedAt, to)
			if end.After(start) {
				totals[name] += end.Sub(start)
			}
		}
	}
	for _, session := range m.listeningHistory.Sessions {
		add(session.StationName, session.Segments)
	}
	if active := m.listeningHistory.Active; active != nil {
		segments := append([]listenhistory.Segment(nil), active.Segments...)
		if !active.ActiveSince.IsZero() && now.After(active.ActiveSince) {
			segments = append(segments, listenhistory.Segment{StartedAt: active.ActiveSince, EndedAt: now})
		}
		add(active.StationName, segments)
	}
	out := make([]historyStationTotal, 0, len(totals))
	for name, duration := range totals {
		if duration > 0 {
			out = append(out, historyStationTotal{name: name, duration: duration})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].duration == out[j].duration {
			return strings.ToLower(out[i].name) < strings.ToLower(out[j].name)
		}
		return out[i].duration > out[j].duration
	})
	if len(out) > 3 {
		out = out[:3]
	}
	return out
}

func (m Model) historyRecent(limit int) []listenhistory.Session {
	sessions := append([]listenhistory.Session(nil), m.listeningHistory.Sessions...)
	sort.SliceStable(sessions, func(i, j int) bool { return sessions[i].EndedAt.After(sessions[j].EndedAt) })
	if len(sessions) > limit {
		sessions = sessions[:limit]
	}
	return sessions
}

func (m Model) historySessionCount(from, to, now time.Time) int {
	count := 0
	for _, session := range m.listeningHistory.Sessions {
		if sessionDurationBetween(session.Segments, from, to) > 0 {
			count++
		}
	}
	if active := m.listeningHistory.Active; active != nil {
		segments := append([]listenhistory.Segment(nil), active.Segments...)
		if !active.ActiveSince.IsZero() && now.After(active.ActiveSince) {
			segments = append(segments, listenhistory.Segment{StartedAt: active.ActiveSince, EndedAt: now})
		}
		if sessionDurationBetween(segments, from, to) > 0 {
			count++
		}
	}
	return count
}

func sessionDurationBetween(segments []listenhistory.Segment, from, to time.Time) time.Duration {
	var duration time.Duration
	for _, segment := range segments {
		start, end := maxHistoryTime(segment.StartedAt, from), minHistoryTime(segment.EndedAt, to)
		if end.After(start) {
			duration += end.Sub(start)
		}
	}
	return duration
}

func maxHistoryTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minHistoryTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func formatHistoryDuration(duration time.Duration) string {
	if duration < time.Minute {
		return "<1m"
	}
	minutes := int(duration.Round(time.Minute) / time.Minute)
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dh %02dm", minutes/60, minutes%60)
}

func formatHistoryWhen(when, now time.Time) string {
	localWhen := when.In(time.Local)
	localNow := now.In(time.Local)
	if localWhen.Year() == localNow.Year() && localWhen.YearDay() == localNow.YearDay() {
		return localWhen.Format("15:04")
	}
	return localWhen.Format("Jan 02")
}
