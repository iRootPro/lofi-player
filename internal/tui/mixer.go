package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/iRootPro/lofi-player/internal/audio"
	"github.com/iRootPro/lofi-player/internal/theme"
)

// mixerModel is the modal's transient UI state. The single source of
// truth for volumes is audio.AmbientMixer; mixerModel only knows which
// channel the user has selected with j/k.
type mixerModel struct {
	mixer    *audio.AmbientMixer
	selected int
}

func newMixerModel(am *audio.AmbientMixer) mixerModel {
	return mixerModel{mixer: am}
}

func (m mixerModel) Selected() string {
	if m.mixer == nil {
		return ""
	}
	ids := m.mixer.ChannelIDs()
	if m.selected < 0 || m.selected >= len(ids) {
		return ""
	}
	return ids[m.selected]
}

const (
	mixerStepFine   = 5
	mixerStepCoarse = 25
)

// handle applies a single key string to the mixer. Returns the
// updated model plus an optional tea.Cmd for the IPC volume change —
// running it in a goroutine keeps the Update loop responsive when
// mpv stalls (sync calls used to freeze the UI for up to 4 s under a
// busy mpv during auto-repeat).
func (m mixerModel) handle(key string) (mixerModel, tea.Cmd) {
	if m.mixer == nil {
		return m, nil
	}
	ids := m.mixer.ChannelIDs()
	switch key {
	case "j", "down":
		if m.selected < len(ids)-1 {
			m.selected++
		}
		return m, nil
	case "k", "up":
		if m.selected > 0 {
			m.selected--
		}
		return m, nil
	case "l", "right":
		return m, m.adjustCmd(mixerStepFine)
	case "h", "left":
		return m, m.adjustCmd(-mixerStepFine)
	case "L":
		return m, m.adjustCmd(mixerStepCoarse)
	case "H":
		return m, m.adjustCmd(-mixerStepCoarse)
	case "0":
		return m, m.setCmd(0)
	case "1":
		return m, m.setCmd(100)
	}
	return m, nil
}

func (m mixerModel) adjustCmd(delta int) tea.Cmd {
	id := m.Selected()
	if id == "" {
		return nil
	}
	target := m.mixer.Volume(id) + delta
	mixer := m.mixer
	return func() tea.Msg {
		if err := mixer.SetVolume(id, target); err != nil {
			return CommandFailedMsg{Action: "set " + id + " volume", Err: err}
		}
		return nil
	}
}

func (m mixerModel) setCmd(v int) tea.Cmd {
	id := m.Selected()
	if id == "" {
		return nil
	}
	mixer := m.mixer
	return func() tea.Msg {
		if err := mixer.SetVolume(id, v); err != nil {
			return CommandFailedMsg{Action: "set " + id + " volume", Err: err}
		}
		return nil
	}
}

const mixerBarWidth = 14

func (m mixerModel) view(width int, styles Styles, t theme.Theme) string {
	if m.mixer == nil {
		return ""
	}
	var inner strings.Builder
	inner.WriteString(styles.SectionHeader.Render("ambient mixer"))
	inner.WriteString("\n")

	ids := m.mixer.ChannelIDs()
	for _, id := range ids {
		ch, _ := m.mixer.Channel(id)
		v := m.mixer.Volume(id)
		disabled := m.mixer.Disabled(id)
		selected := ids[m.selected] == id
		inner.WriteString("\n")
		inner.WriteString(m.renderRow(ch, v, disabled, selected, styles))
	}
	inner.WriteString("\n\n")
	inner.WriteString(m.renderHint(styles))

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Muted).
		Padding(0, 2).
		Render(inner.String())

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, card)
}

func (m mixerModel) renderRow(ch audio.AmbientChannel, v int, disabled, selected bool, styles Styles) string {
	cursor := "  "
	if selected {
		cursor = styles.Cursor.Render("›") + " "
	}
	label := fmt.Sprintf("%-12s", ch.Label)
	// Icon picks up the brand Primary tone like the logo / volume / stations
	// glyphs so the modal sits in the same visual family as the main view.
	icon := styles.AppTitle.Render(ch.Icon)

	if disabled {
		return cursor + icon + "  " + styles.Hint.Render(label) + styles.Hint.Render("unavailable")
	}

	fill := v * mixerBarWidth / 100
	bar := styles.VolFill.Render(strings.Repeat("▰", fill)) +
		styles.VolEmpty.Render(strings.Repeat("▱", mixerBarWidth-fill))
	value := fmt.Sprintf("%3d", v)

	// Selected row uses Cursor's color but explicitly drops Bold so the
	// label doesn't out-weigh the small Nerd Font icon next to it.
	switch {
	case selected:
		labelStyle := styles.Cursor.Bold(false)
		return cursor + icon + "  " + labelStyle.Render(label) + bar + "  " + labelStyle.Render(value)
	case v == 0:
		return cursor + icon + "  " + styles.Hint.Render(label) + bar + "  " + styles.Hint.Render(value)
	default:
		return cursor + icon + "  " + styles.StationItem.Render(label) + bar + "  " + styles.StationItem.Render(value)
	}
}

func (m mixerModel) renderHint(styles Styles) string {
	pair := func(k, d string) string {
		return styles.HelpKey.Render(k) + " " + styles.HelpDesc.Render(d)
	}
	sep := "  " + styles.HelpSep.Render("·") + "  "
	return pair("j/k", "select") + sep +
		pair("h/l", "±5") + sep +
		pair("0", "mute") + sep +
		pair("x", "close")
}
