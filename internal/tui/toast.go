package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/iRootPro/lofi-player/internal/i18n"
)

// ToastKind classifies a Toast for visual styling.
type ToastKind int

const (
	// ToastError is for recoverable failures (playback drop, bad URL).
	ToastError ToastKind = iota
	// ToastInfo is for neutral notifications. Reserved for Phase 3+.
	ToastInfo
	// ToastSuccess is for positive confirmations. Reserved for Phase 3+.
	ToastSuccess
)

// Toast is a transient message shown in the help-bar slot. The Update
// loop sets one and schedules a clearToastMsg via tea.Tick to wipe it
// after toastLifetime.
type Toast struct {
	Message string
	Kind    ToastKind
}

// labelStyle picks the prefix (e.g. "error: ") style for the toast kind.
func (t Toast) labelStyle(s Styles) lipgloss.Style {
	switch t.Kind {
	case ToastError:
		return s.StationCursor // accent + bold
	case ToastSuccess:
		return s.StatusLive // success
	default:
		return s.HelpKey // warning
	}
}

// label is the human-readable prefix that precedes the message.
func (t Toast) label(locale i18n.Locale) string {
	switch t.Kind {
	case ToastError:
		return i18n.T(locale, "error_label")
	case ToastSuccess:
		return i18n.T(locale, "success_label")
	default:
		return ""
	}
}
