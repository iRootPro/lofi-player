package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/iRootPro/lofi-player/internal/config"
)

// addStationForm holds the modal "add station" form: two text inputs
// (name, url) plus an index of which one is currently focused. Tab
// cycles focus, Enter submits, Esc cancels. The form is purely UI —
// persistence is the caller's responsibility.
type addStationForm struct {
	name    textinput.Model
	url     textinput.Model
	focused int // 0 = name, 1 = url
	err     string
}

// addFormResult communicates the outcome of a form submission back to
// the parent Update. Cancelled is true when the user pressed Esc;
// Saved.* fields are valid when Cancelled is false.
type addFormResult struct {
	Cancelled bool
	Station   config.Station
}

// newAddStationForm constructs a fresh form with the name field focused.
func newAddStationForm() addStationForm {
	name := textinput.New()
	name.Placeholder = "e.g. Lofi Girl 24/7"
	name.CharLimit = 80
	name.Width = 36
	name.Prompt = ""
	name.Focus()

	url := textinput.New()
	url.Placeholder = "https://..."
	url.CharLimit = 256
	url.Width = 36
	url.Prompt = ""

	return addStationForm{name: name, url: url, focused: 0}
}

// update routes the message to whichever input is focused, after first
// handling the form-level keys (Tab, Enter, Esc). The returned ok bool
// is true when the form is still open; result is meaningful only when
// ok is false.
func (f addStationForm) update(msg tea.Msg) (addStationForm, addFormResult, bool, tea.Cmd) {
	if km, isKey := msg.(tea.KeyMsg); isKey {
		switch km.String() {
		case "esc":
			return f, addFormResult{Cancelled: true}, false, nil
		case "tab":
			f = f.advanceFocus(1)
			return f, addFormResult{}, true, nil
		case "shift+tab":
			f = f.advanceFocus(-1)
			return f, addFormResult{}, true, nil
		case "enter":
			name := strings.TrimSpace(f.name.Value())
			url := strings.TrimSpace(f.url.Value())
			if name == "" {
				f.err = "name is required"
				return f, addFormResult{}, true, nil
			}
			if url == "" {
				f.err = "url is required"
				return f, addFormResult{}, true, nil
			}
			station := config.Station{
				Name: name,
				URL:  url,
				Kind: detectKind(url),
			}
			return f, addFormResult{Station: station}, false, nil
		}
	}

	var cmd tea.Cmd
	switch f.focused {
	case 0:
		f.name, cmd = f.name.Update(msg)
	case 1:
		f.url, cmd = f.url.Update(msg)
	}
	// Typing clears any prior validation message.
	if _, isKey := msg.(tea.KeyMsg); isKey {
		f.err = ""
	}
	return f, addFormResult{}, true, cmd
}

func (f addStationForm) advanceFocus(delta int) addStationForm {
	f.focused = (f.focused + delta + 2) % 2
	switch f.focused {
	case 0:
		f.name.Focus()
		f.url.Blur()
	case 1:
		f.name.Blur()
		f.url.Focus()
	}
	return f
}

// view renders the form as a centered, rounded-border card. width is
// the terminal width used to center the card; styles supplies the
// active theme's lipgloss palette so the card matches the rest of the
// UI on theme change.
func (f addStationForm) view(width int, styles Styles, themeMuted lipgloss.Color) string {
	label := func(s string) string {
		return styles.HelpDesc.Render(s)
	}

	var inner strings.Builder
	inner.WriteString(styles.SectionHeader.Render("─── add station ───"))
	inner.WriteString("\n\n")
	inner.WriteString(label("name  ") + f.name.View())
	inner.WriteString("\n")
	inner.WriteString(label("url   ") + f.url.View())
	inner.WriteString("\n\n")

	if f.err != "" {
		inner.WriteString(styles.StationCursor.Render("error: "))
		inner.WriteString(styles.HelpDesc.Render(f.err))
		inner.WriteString("\n\n")
	}

	hint := styles.HelpKey.Render("enter") + " " +
		styles.HelpDesc.Render("save") + "  " +
		styles.HelpSep.Render("·") + "  " +
		styles.HelpKey.Render("tab") + " " +
		styles.HelpDesc.Render("next field") + "  " +
		styles.HelpSep.Render("·") + "  " +
		styles.HelpKey.Render("esc") + " " +
		styles.HelpDesc.Render("cancel")
	inner.WriteString(hint)

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(themeMuted).
		Padding(1, 3).
		Render(inner.String())

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, card)
}

// detectKind classifies a URL for storage. The check is conservative —
// only obvious YouTube hosts trip the youtube branch; everything else
// stays as the default stream kind.
func detectKind(url string) string {
	u := strings.ToLower(url)
	if strings.Contains(u, "youtube.com") || strings.Contains(u, "youtu.be") {
		return config.KindYouTube
	}
	return config.KindStream
}
