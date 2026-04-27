package tui

import (
	"fmt"
	"strings"

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

// handle applies a single key string to the mixer. Volume changes are
// pushed straight to the audio mixer; the model itself doesn't cache
// them.
func (m mixerModel) handle(key string) mixerModel {
	if m.mixer == nil {
		return m
	}
	ids := m.mixer.ChannelIDs()
	switch key {
	case "j", "down":
		if m.selected < len(ids)-1 {
			m.selected++
		}
	case "k", "up":
		if m.selected > 0 {
			m.selected--
		}
	case "l", "right":
		m.adjust(mixerStepFine)
	case "h", "left":
		m.adjust(-mixerStepFine)
	case "L":
		m.adjust(mixerStepCoarse)
	case "H":
		m.adjust(-mixerStepCoarse)
	case "0":
		m.set(0)
	case "1":
		m.set(100)
	}
	return m
}

func (m mixerModel) adjust(delta int) {
	id := m.Selected()
	if id == "" {
		return
	}
	_ = m.mixer.SetVolume(id, m.mixer.Volume(id)+delta)
}

func (m mixerModel) set(v int) {
	id := m.Selected()
	if id == "" {
		return
	}
	_ = m.mixer.SetVolume(id, v)
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
