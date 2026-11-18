package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/iRootPro/lofi-player/internal/updatecheck"
)

type updateNoticeModel struct {
	latestVersion string
	releaseURL    string
}

func (u updateNoticeModel) available() bool {
	return u.latestVersion != ""
}

type updateCheckResultMsg struct {
	result updatecheck.Result
	err    error
}

func checkForUpdateCmd(check func() (updatecheck.Result, error)) tea.Cmd {
	return func() tea.Msg {
		result, err := check()
		return updateCheckResultMsg{result: result, err: err}
	}
}

func (m Model) openUpdateNotice() Model {
	m.modePrev = m.mode
	m.mode = modeUpdate
	return m
}

func (m Model) updateUpdateNotice(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if key.Matches(keyMsg, m.keys.Quit) {
		return m, tea.Quit
	}
	if keyMsg.String() == "esc" || key.Matches(keyMsg, m.keys.Update) {
		m.mode = m.modePrev
	}
	return m, nil
}

func (m Model) viewUpdateNotice() string {
	backdrop := m.modalBackdrop()
	contentWidth := discoveryContentWidth(m.width)
	var body strings.Builder
	body.WriteString(m.styles.SectionHeader.Render(m.t("update_title")))
	body.WriteString("\n\n")
	body.WriteString(m.styles.HelpDesc.Render(m.t("update_body", m.updateNotice.latestVersion)))
	body.WriteString("\n\n")
	body.WriteString(m.styles.HelpKey.Render("Homebrew"))
	body.WriteString("  ")
	body.WriteString(m.styles.HelpDesc.Render("brew upgrade iRootPro/tap/lofi-player"))
	body.WriteString("\n")
	body.WriteString(m.styles.HelpKey.Render(m.t("other")))
	body.WriteString("     ")
	body.WriteString(m.styles.HelpDesc.Render(m.t("release_notes")))
	for _, line := range wrapRunes(m.updateNotice.releaseURL, max(12, contentWidth-10)) {
		body.WriteString("\n          ")
		body.WriteString(m.styles.Hint.Render(line))
	}
	body.WriteString("\n\n")
	body.WriteString(m.styles.HelpKey.Render("u / esc") + " " + m.styles.HelpDesc.Render(m.t("close")))

	card := lipgloss.NewStyle().
		Width(contentWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Muted).
		Padding(1, 3).
		Render(body.String())
	return overlayModal(backdrop, lipgloss.PlaceHorizontal(m.width, lipgloss.Center, card))
}
