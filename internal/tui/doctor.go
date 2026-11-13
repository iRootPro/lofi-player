package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/iRootPro/lofi-player/internal/doctor"
)

const doctorCheckTimeout = 6 * time.Second

type doctorModel struct {
	report   doctor.Report
	checking bool
	checkSeq int
	cancel   context.CancelFunc
}

func (m Model) openDoctor() Model {
	m.modePrev = m.mode
	m.mode = modeDoctor
	m.doctorCheckSeq++
	m.doctor = doctorModel{report: m.liveDoctorReport(), checkSeq: m.doctorCheckSeq}
	return m
}

func (m Model) updateDoctor(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch {
	case key.Matches(keyMsg, m.keys.Quit) || keyMsg.String() == "esc":
		if m.doctor.cancel != nil {
			m.doctor.cancel()
		}
		m.doctor.cancel = nil
		m.doctor.checking = false
		m.mode = m.modePrev
		return m, nil
	case keyMsg.String() == "r":
		return m.startDoctorChecks()
	case keyMsg.String() == "c":
		m.doctor.report = m.liveDoctorReportWithChecks()
		return m, copyDoctorCmd("report", m.doctor.report.String())
	case keyMsg.String() == "h":
		host := doctor.Hostname(m.doctorSnapshot().StreamURL)
		if host == "" {
			m.toast = &Toast{Message: "No active stream hostname to copy.", Kind: ToastInfo}
			return m, clearToastAfter()
		}
		return m, copyDoctorCmd("host", host)
	}
	return m, nil
}

func (m Model) startDoctorChecks() (Model, tea.Cmd) {
	if m.doctor.cancel != nil {
		m.doctor.cancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), doctorCheckTimeout)
	m.doctor.cancel = cancel
	m.doctor.checking = true
	m.doctorCheckSeq++
	m.doctor.checkSeq = m.doctorCheckSeq
	m.doctor.report = m.liveDoctorReportWithChecks()
	return m, doctorCheckCmd(ctx, m.doctor.checkSeq, m.doctorOptions())
}

func (m Model) doctorOptions() doctor.Options {
	return doctor.Options{
		ApplicationVersion: m.appVersion,
		Snapshot:           m.doctorSnapshot(),
	}
}

func (m Model) doctorSnapshot() doctor.Snapshot {
	state := ""
	switch {
	case m.loading:
		state = "loading"
	case m.playing:
		state = "playing"
	case m.hasActiveStation():
		state = "paused"
	}
	if m.cacheSeconds > 0 && state == "playing" {
		state = fmt.Sprintf("playing · buffer %.0fs", m.cacheSeconds)
	}

	streamURL := ""
	if station, ok := m.activeStation(); ok {
		streamURL = station.URL
	}
	return doctor.Snapshot{
		PlayerConnected: m.player != nil,
		PlaybackState:   state,
		StreamURL:       streamURL,
		CacheSeconds:    m.cacheSeconds,
		Buffering:       m.bufferingStalled,
		LastIssue:       m.lastPlaybackIssue,
	}
}

func (m Model) liveDoctorReport() doctor.Report {
	return doctor.NewReport(m.doctorOptions())
}

func (m Model) liveDoctorReportWithChecks() doctor.Report {
	live := m.liveDoctorReport()
	live.MPV = m.doctor.report.MPV
	live.YTDLP = m.doctor.report.YTDLP
	live.Config = m.doctor.report.Config
	live.DNS = m.doctor.report.DNS
	live.Discovery = m.doctor.report.Discovery
	return live
}

func (m Model) viewDoctor() string {
	backdrop := m.modalBackdrop()
	report := m.doctor.report
	if report.Application == "" {
		report = m.liveDoctorReport()
	}

	rows := []struct {
		label string
		check doctor.Check
	}{
		{"application", doctor.Check{Status: doctor.StatusOK, Detail: report.Application}},
		{"player", report.Player},
		{"playback", report.Playback},
		{"stream host", report.StreamHost},
		{"discovery/cache", report.Discovery},
		{"last issue", report.LastIssue},
	}
	var body strings.Builder
	for _, row := range rows {
		body.WriteString(m.renderDoctorRow(row.label, row.check))
		body.WriteByte('\n')
	}
	if m.doctor.checking {
		body.WriteString(m.styles.HelpDesc.Render("Checking mpv, config, DNS, and Radio Browser…"))
		body.WriteByte('\n')
	}

	hint := m.styles.HelpKey.Render("c") + " " + m.styles.HelpDesc.Render("copy safe report") + "  " +
		m.styles.HelpSep.Render("·") + "  " +
		m.styles.HelpKey.Render("h") + " " + m.styles.HelpDesc.Render("copy stream host") + "  " +
		m.styles.HelpSep.Render("·") + "  " +
		m.styles.HelpKey.Render("r") + " " + m.styles.HelpDesc.Render("run checks") + "  " +
		m.styles.HelpSep.Render("·") + "  " +
		m.styles.HelpKey.Render("esc") + " " + m.styles.HelpDesc.Render("close")

	inner := m.styles.SectionHeader.Render("─── playback doctor ───") + "\n\n" + strings.TrimRight(body.String(), "\n")
	if m.toast != nil {
		inner += "\n\n" + m.renderToast()
	}
	inner += "\n\n" + hint

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Muted).
		Padding(1, 3).
		Render(inner)
	return overlayModal(backdrop, lipgloss.PlaceHorizontal(m.width, lipgloss.Center, card))
}

func (m Model) renderDoctorRow(label string, check doctor.Check) string {
	glyph := "·"
	style := m.styles.Hint
	switch check.Status {
	case doctor.StatusOK:
		glyph = "●"
		style = m.styles.StatusLive
	case doctor.StatusWarning:
		glyph = "!"
		style = m.styles.HelpKey
	case doctor.StatusError:
		glyph = "×"
		style = m.styles.StationCursor
	}
	return m.styles.StationItem.Render(fmt.Sprintf("%-16s", label)) + " " + style.Render(glyph+" "+string(check.Status)) + " " + m.styles.HelpDesc.Render(check.Detail)
}
