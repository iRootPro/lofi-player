package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/iRootPro/lofi-player/internal/config"
)

// addStationForm holds the modal form used both for adding a new
// station and for editing an existing one. The two flows share the
// same UI (two text inputs, focus cycling, validation); they differ
// only in the title shown on top and what the parent does with the
// result. editIdx >= 0 marks the form as editing station[editIdx];
// -1 marks it as a fresh add.
type addStationForm struct {
	name    textinput.Model
	url     textinput.Model
	focused int // 0 = name, 1 = url
	err     string
	editIdx int
}

// addFormResult communicates the outcome of a form submission back to
// the parent Update. Cancelled is true when the user pressed Esc;
// Station fields are valid when Cancelled is false. EditIdx mirrors
// the form's editIdx so the parent can decide between append and
// in-place update without tracking the intent itself.
type addFormResult struct {
	Cancelled bool
	Station   config.Station
	EditIdx   int
}

// newAddStationForm constructs a fresh form for adding a new station,
// with both fields empty and the name field focused.
func newAddStationForm() addStationForm {
	return newStationForm("", "", -1)
}

// newEditStationForm constructs a form pre-filled with an existing
// station's name and url for in-place editing.
func newEditStationForm(idx int, s config.Station) addStationForm {
	return newStationForm(s.Name, s.URL, idx)
}

func newStationForm(initName, initURL string, editIdx int) addStationForm {
	name := textinput.New()
	name.Placeholder = "e.g. Lofi Girl 24/7"
	name.CharLimit = 80
	name.Width = 36
	name.Prompt = ""
	name.SetValue(initName)
	name.CursorEnd()
	name.Focus()

	url := textinput.New()
	url.Placeholder = "https://..."
	url.CharLimit = 256
	url.Width = 36
	url.Prompt = ""
	url.SetValue(initURL)
	url.CursorEnd()

	return addStationForm{name: name, url: url, focused: 0, editIdx: editIdx}
}

// update routes the message to whichever input is focused, after first
// handling the form-level keys (Tab, Enter, Esc). The returned ok bool
// is true when the form is still open; result is meaningful only when
// ok is false.
func (f addStationForm) update(msg tea.Msg) (addStationForm, addFormResult, bool, tea.Cmd) {
	if km, isKey := msg.(tea.KeyMsg); isKey {
		switch km.String() {
		case "esc":
			return f, addFormResult{Cancelled: true, EditIdx: f.editIdx}, false, nil
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
			return f, addFormResult{Station: station, EditIdx: f.editIdx}, false, nil
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

	title := "─── add station ───"
	if f.editIdx >= 0 {
		title = "─── edit station ───"
	}
	var inner strings.Builder
	inner.WriteString(styles.SectionHeader.Render(title))
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
