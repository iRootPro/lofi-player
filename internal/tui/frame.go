package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderFrame draws content inside a rounded border whose top edge
// embeds an optional title on the left and an optional label on the
// right. The result reads like a desktop window's title bar:
//
//	╭─ ♪ lofi.player ──────────────────  16:57 ─╮
//	│                                            │
//	│  ● Lofi Girl 24/7                          │
//	│  ...                                       │
//	╰────────────────────────────────────────────╯
//
// width is the outer width of the frame. content is split on '\n'
// and each line is padded right to inner width = width - 2 so the
// right border sits at a consistent column. One blank line is added
// above and below the content for vertical breathing.
//
// borderStyle paints the corners and "─"/"│" segments. titleStyle
// and rightStyle paint the title and right-label text respectively.
// lipgloss has no native API for this — corners and side bars are
// composed manually so the title can interrupt the top border.
func renderFrame(content, title, rightLabel string, width int, borderStyle, titleStyle, rightStyle lipgloss.Style) string {
	if width < 8 {
		width = 8
	}
	inner := width - 2 // cells between the corner glyphs

	titleSeg := ""
	if title != "" {
		titleSeg = " " + titleStyle.Render(title) + " "
	}
	rightSeg := ""
	if rightLabel != "" {
		rightSeg = " " + rightStyle.Render(rightLabel) + " "
	}

	// Layout: corner ─ titleSeg filler rightSeg ─ corner.
	// Inner = 2 fixed "─" + titleW + fillerW + rightW.
	fillerW := inner - 2 - lipgloss.Width(titleSeg) - lipgloss.Width(rightSeg)
	if fillerW < 0 {
		fillerW = 0
	}

	top := borderStyle.Render("╭") +
		borderStyle.Render("─") + titleSeg +
		borderStyle.Render(strings.Repeat("─", fillerW)) +
		rightSeg + borderStyle.Render("─") +
		borderStyle.Render("╮")

	bottom := borderStyle.Render("╰" + strings.Repeat("─", inner) + "╯")

	leftBar := borderStyle.Render("│")
	rightBar := borderStyle.Render("│")

	pad := func(line string) string {
		w := lipgloss.Width(line)
		gap := inner - w
		if gap < 0 {
			gap = 0
		}
		return leftBar + line + strings.Repeat(" ", gap) + rightBar
	}

	var b strings.Builder
	b.WriteString(top)
	b.WriteString("\n")
	b.WriteString(pad(""))
	b.WriteString("\n")
	for _, line := range strings.Split(content, "\n") {
		b.WriteString(pad(line))
		b.WriteString("\n")
	}
	b.WriteString(pad(""))
	b.WriteString("\n")
	b.WriteString(bottom)
	return b.String()
}
